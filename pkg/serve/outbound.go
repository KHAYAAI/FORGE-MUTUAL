package serve

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Outbound returns the client a service uses to call other services. Every outbound request carries
// the request ID of the inbound request it serves, so one ID follows a call across services. In
// production the client presents this service's certificate, verifies the peer against the CA, and
// refuses any URL that is not https. In development it is a plain client.
func Outbound(cfg Config) (*http.Client, error) {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.ForceAttemptHTTP2 = true
	var rt http.RoundTripper = base
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
			tc.RootCAs = pool
		}
		base.TLSClientConfig = tc
	}
	if cfg.Production {
		rt = httpsOnly{next: rt}
	}
	traced := otelhttp.NewTransport(requestIDPropagation{next: rt}, otelhttp.WithPropagators(propagator))
	return &http.Client{Transport: traced, Timeout: 30 * time.Second}, nil
}

// OutboundFromEnv is Outbound with the configuration read from the environment.
func OutboundFromEnv() (*http.Client, error) {
	cfg := Config{
		CertFile:   os.Getenv("TLS_CERT_FILE"),
		KeyFile:    os.Getenv("TLS_KEY_FILE"),
		CAFile:     os.Getenv("TLS_CA_FILE"),
		Production: callerauth.IsProduction(),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return Outbound(cfg)
}

var errPlainHTTP = errors.New("production: outbound calls must use https")

type httpsOnly struct{ next http.RoundTripper }

func (h httpsOnly) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" {
		return nil, errPlainHTTP
	}
	return h.next.RoundTrip(r)
}

type requestIDPropagation struct{ next http.RoundTripper }

func (p requestIDPropagation) RoundTrip(r *http.Request) (*http.Response, error) {
	if id := RequestIDFrom(r.Context()); id != "" && r.Header.Get(RequestIDHeader) == "" {
		r = r.Clone(r.Context())
		r.Header.Set(RequestIDHeader, id)
	}
	return p.next.RoundTrip(r)
}
