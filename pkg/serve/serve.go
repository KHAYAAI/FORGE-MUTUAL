// Package serve is the HTTP server every service runs. It owns what must be the same in every
// service: timeouts, request IDs, panic recovery, access logs, security headers, TLS and mutual TLS
// (the caller's identity is the service name in its client certificate), outbound clients that
// propagate request IDs, and graceful shutdown. A service builds its handler and hands it here.
//
// Environment (read by FromEnv):
//
//	FORGE_ENV        production enables every check below; anything else is development.
//	TLS_CERT_FILE    this service's certificate (PEM). Required in production.
//	TLS_KEY_FILE     its private key (PEM). Required in production.
//	TLS_CA_FILE      the CA that signs every service certificate (PEM). Required in production. Clients
//	                 must present a certificate it signed, and outbound calls verify against it.
//
// A service certificate names its service in a URI subject alternative name:
// spiffe://forge/svc/<name>, where <name> is the caller name the callers files allow (for example
// svc-workspace).
package serve

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// RequestIDHeader carries the request ID in and out of every service.
const RequestIDHeader = "X-Request-Id"

// SPIFFEPrefix is the URI prefix a service certificate uses to name its service.
const SPIFFEPrefix = "spiffe://forge/svc/"

var (
	requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)
	serviceNamePat   = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)
)

// Config is the TLS and listening configuration of one service.
type Config struct {
	Addr       string
	CertFile   string
	KeyFile    string
	CAFile     string
	Production bool
}

// FromEnv reads the TLS settings from the environment and validates them. Production requires a
// certificate, a key and a CA; development may run plain HTTP with none of them.
func FromEnv(addr string) (Config, error) {
	c := Config{
		Addr:       addr,
		CertFile:   os.Getenv("TLS_CERT_FILE"),
		KeyFile:    os.Getenv("TLS_KEY_FILE"),
		CAFile:     os.Getenv("TLS_CA_FILE"),
		Production: callerauth.IsProduction(),
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

// Validate refuses configurations that would serve unauthenticated callers in production.
func (c Config) Validate() error {
	if (c.CertFile == "") != (c.KeyFile == "") {
		return errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	if c.Production {
		if c.CertFile == "" || c.CAFile == "" {
			return errors.New("production requires TLS_CERT_FILE, TLS_KEY_FILE and TLS_CA_FILE: services accept only mutually authenticated callers in production")
		}
	}
	if c.CAFile != "" && c.CertFile == "" {
		return errors.New("TLS_CA_FILE needs TLS_CERT_FILE and TLS_KEY_FILE: a CA without a service certificate cannot authenticate the service")
	}
	return nil
}

// New wraps h with the shared middleware and returns a server ready for Run. With a certificate it
// serves TLS; with a CA as well it requires every client to present a certificate that CA signed.
func New(cfg Config, h http.Handler, log *slog.Logger) (*http.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	// Cleartext HTTP/2 (h2c) is for development and for services behind a TLS-terminating mesh. It
	// wraps the whole chain so that h2c requests get the same request IDs, headers and logs.
	inner := h
	if cfg.CertFile == "" {
		inner = h2c.NewHandler(h, &http2.Server{})
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           chain(inner, log, cfg.CAFile != ""),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	if cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("tls certificate: %w", err)
		}
		tc := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		if cfg.CAFile != "" {
			pool, err := loadCA(cfg.CAFile)
			if err != nil {
				return nil, err
			}
			tc.ClientCAs = pool
			tc.ClientAuth = tls.RequireAndVerifyClientCert
		}
		srv.TLSConfig = tc
	}
	return srv, nil
}

// Run serves until ctx is done, then shuts down gracefully within ten seconds.
func Run(ctx context.Context, srv *http.Server, log *slog.Logger) error {
	tlsOn := srv.TLSConfig != nil && len(srv.TLSConfig.Certificates) > 0
	mtls := tlsOn && srv.TLSConfig.ClientAuth == tls.RequireAndVerifyClientCert
	errc := make(chan error, 1)
	go func() {
		if tlsOn {
			errc <- srv.ListenAndServeTLS("", "")
			return
		}
		errc <- srv.ListenAndServe()
	}()
	log.Info("listening", "addr", srv.Addr, "tls", tlsOn, "mutual_tls", mtls)
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// chain applies the shared middleware. The trace span is outermost, so the request ID and the access log both
// run inside it and can record the trace ID; the request ID is next, so every log line carries it.
func chain(h http.Handler, log *slog.Logger, mutual bool) http.Handler {
	h = clientIdentity(h, log, mutual)
	h = securityHeaders(h)
	h = accessLog(h, log)
	h = recoverPanics(h, log)
	return otelhttp.NewHandler(requestID(h), "forge.inbound", otelhttp.WithPropagators(propagator))
}

// RequestIDFrom returns the request ID on ctx, or "" outside a request.
func RequestIDFrom(ctx context.Context) string {
	s, _ := ctx.Value(requestIDKey{}).(string)
	return s
}

type requestIDKey struct{}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !requestIDPattern.MatchString(id) {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// statusWriter records the status and bytes written, and passes flushing through so streaming RPCs
// still stream.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacking not supported")
	}
	return h.Hijack()
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// accessLog writes one line per request: method, path (never the query string, which can carry
// tokens), status, size, duration and request ID. Headers are never logged.
func accessLog(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", sw.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", RequestIDFrom(r.Context()),
			"trace_id", traceID(r),
		)
	})
}

// recoverPanics turns a panic into a 500 with no detail, logs the stack with the request ID, and
// keeps the server running.
func recoverPanics(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			log.Error("panic", "request_id", RequestIDFrom(r.Context()), "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
			http.Error(w, "internal error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders applies to every response. The services are JSON and RPC APIs, so nothing they
// return may be framed, sniffed, cached or sent with a referrer.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

// clientIdentity sets the transport identity for the request from the client certificate. With
// mutual TLS on, a client whose certificate does not name a service is refused here.
func clientIdentity(next http.Handler, log *slog.Logger, mutual bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			name, err := ServiceName(r.TLS.PeerCertificates[0])
			if err != nil {
				log.Warn("refused client certificate", "request_id", RequestIDFrom(r.Context()), "reason", err.Error())
				http.Error(w, "client certificate is not a service identity", http.StatusForbidden)
				return
			}
			r = r.WithContext(callerauth.WithIdentity(r.Context(), name))
		} else if mutual {
			http.Error(w, "a client certificate is required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ServiceName returns the service a certificate names. Exactly one spiffe://forge/svc/<name> URI is
// required, and the name must be a well-formed service name.
func ServiceName(cert *x509.Certificate) (string, error) {
	var names []string
	for _, u := range cert.URIs {
		if s := u.String(); strings.HasPrefix(s, SPIFFEPrefix) {
			names = append(names, strings.TrimPrefix(s, SPIFFEPrefix))
		}
	}
	if len(names) != 1 {
		return "", fmt.Errorf("certificate must name exactly one service, found %d", len(names))
	}
	if !serviceNamePat.MatchString(names[0]) {
		return "", fmt.Errorf("service name %q is malformed", names[0])
	}
	return names[0], nil
}

func loadCA(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tls CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("tls CA: no certificate found in " + path)
	}
	return pool, nil
}

// Switch is a development-only setting. Set reports whether it is turned on.
type Switch struct {
	Name string
	Set  bool
}

// RefuseInProduction returns an error naming every development-only switch that is on when the
// process runs in production. A service calls it at startup, before it listens, so that a stub
// ledger, a simulated rail or an unsigned policy bundle cannot reach production by configuration.
func RefuseInProduction(switches ...Switch) error {
	if !callerauth.IsProduction() {
		return nil
	}
	var on []string
	for _, s := range switches {
		if s.Set {
			on = append(on, s.Name)
		}
	}
	if len(on) > 0 {
		return fmt.Errorf("production refuses development-only settings: %s", strings.Join(on, ", "))
	}
	return nil
}

// traceID returns the trace ID of the request's span, or "" when tracing is not sampling it.
func traceID(r *http.Request) string {
	if sc := trace.SpanContextFromContext(r.Context()); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
