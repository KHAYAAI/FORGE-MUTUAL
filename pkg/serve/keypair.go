package serve

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/khayaai/forge-mutual/pkg/callerauth"
)

// keyPair is a service's certificate, read from disk and read again when it is renewed.
//
// Production certificates are short-lived, and the platform's PKI renews them in place. A process that read its
// certificate once at start would go on presenting it after it had expired, and every call to and from it would fail
// until someone restarted it. keyPair looks at the two files at most once every checkEvery, and loads them again when
// either has changed. If the new pair does not load (the files are caught half-written, or do not belong together), it
// keeps the pair it has, says so in the log, and tries again at the next check.
type keyPair struct {
	certFile, keyFile string
	checkEvery        time.Duration
	now               func() time.Time
	log               *slog.Logger

	mu      sync.Mutex
	cert    *tls.Certificate
	stamp   [2]time.Time // the files' modification times when cert was loaded
	checked time.Time
}

// certCheckEvery is how often a service looks at its certificate files. A renewal is picked up within this long. Tests
// shorten it.
var certCheckEvery = 15 * time.Second

// loadKeyPair reads the pair once, and fails if it cannot: a service must not start without its certificate.
func loadKeyPair(certFile, keyFile string, log *slog.Logger) (*keyPair, error) {
	kp := &keyPair{certFile: certFile, keyFile: keyFile, checkEvery: certCheckEvery, now: time.Now, log: log}
	cert, stamp, err := kp.read()
	if err != nil {
		return nil, fmt.Errorf("tls certificate: %w", err)
	}
	kp.cert, kp.stamp, kp.checked = cert, stamp, kp.now()
	return kp, nil
}

func (kp *keyPair) read() (*tls.Certificate, [2]time.Time, error) {
	var stamp [2]time.Time
	for i, f := range []string{kp.certFile, kp.keyFile} {
		info, err := os.Stat(f)
		if err != nil {
			return nil, stamp, err
		}
		stamp[i] = info.ModTime()
	}
	cert, err := tls.LoadX509KeyPair(kp.certFile, kp.keyFile)
	if err != nil {
		return nil, stamp, err
	}
	return &cert, stamp, nil
}

// current returns the certificate to present now.
func (kp *keyPair) current() *tls.Certificate {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	if kp.now().Sub(kp.checked) < kp.checkEvery {
		return kp.cert
	}
	kp.checked = kp.now()
	changed := false
	for i, f := range []string{kp.certFile, kp.keyFile} {
		if info, err := os.Stat(f); err != nil || !info.ModTime().Equal(kp.stamp[i]) {
			changed = true
		}
	}
	if !changed {
		return kp.cert
	}
	cert, stamp, err := kp.read()
	if err != nil {
		if kp.log != nil {
			kp.log.Warn("renewed certificate could not be loaded; still presenting the previous one", "file", kp.certFile, "err", err.Error())
		}
		return kp.cert
	}
	kp.cert, kp.stamp = cert, stamp
	if kp.log != nil {
		kp.log.Info("certificate renewed", "file", kp.certFile)
	}
	return kp.cert
}

// forServer and forClient are the two hooks crypto/tls calls for each handshake.
func (kp *keyPair) forServer(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return kp.current(), nil
}

func (kp *keyPair) forClient(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	return kp.current(), nil
}

// OnlyClients returns h restricted to the callers named: a request is served only when its client certificate names
// one of them. It is for a service that exactly one kind of client may reach, such as an API that only its own web
// app's server calls. Without it, mutual TLS admits every service the CA has signed, and a service that authenticates
// people by a token would accept that token from anywhere inside.
//
// The list needs mutual TLS to mean anything, so it is refused without it. Production requires the list.
func OnlyClients(cfg Config, setting string, names []string, h http.Handler) (http.Handler, error) {
	var allowed []string
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			if !serviceNamePat.MatchString(n) {
				return nil, fmt.Errorf("%s: %q is not a service name", setting, n)
			}
			allowed = append(allowed, n)
		}
	}
	if len(allowed) == 0 {
		if cfg.Production {
			return nil, fmt.Errorf("production requires %s: the services whose certificates may call this one", setting)
		}
		return h, nil
	}
	if cfg.CAFile == "" {
		return nil, errors.New(setting + " needs mutual TLS (TLS_CERT_FILE, TLS_KEY_FILE and TLS_CA_FILE): without it there is no caller identity to check")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := callerauth.IdentityFrom(r.Context())
		for _, n := range allowed {
			if caller == n {
				h.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, "this caller may not use this service", http.StatusForbidden)
	}), nil
}

// ClientsFromEnv reads a comma-separated list of service names from the environment.
func ClientsFromEnv(setting string) []string {
	v := os.Getenv(setting)
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.Split(v, ",")
}
