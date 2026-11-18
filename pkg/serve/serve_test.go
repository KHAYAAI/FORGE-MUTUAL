package serve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/serve/pkitest"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func ok(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func TestRequestIDIsGeneratedOrKept(t *testing.T) {
	h, err := New(Config{Addr: ":0"}, http.HandlerFunc(ok), quiet)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, in string
		keep     bool
	}{
		{"absent", "", false},
		{"well formed", "req-20260203-abcdef", true},
		{"too short", "abc", false},
		{"unsafe characters", "a b\r\nSet-Cookie: x", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/v1/x", nil)
			if c.in != "" {
				r.Header.Set(RequestIDHeader, c.in)
			}
			w := httptest.NewRecorder()
			h.Handler.ServeHTTP(w, r)
			got := w.Header().Get(RequestIDHeader)
			if c.keep && got != c.in {
				t.Fatalf("id = %q, want %q kept", got, c.in)
			}
			if !c.keep && (got == "" || got == c.in || !requestIDPattern.MatchString(got)) {
				t.Fatalf("id = %q, want a fresh well-formed one", got)
			}
		})
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	h, err := New(Config{Addr: ":0"}, http.HandlerFunc(ok), quiet)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestPanicIsRecoveredAndTheServerKeepsServing(t *testing.T) {
	boom := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/boom" {
			panic("secret internal detail")
		}
		ok(w, r)
	})
	h, err := New(Config{Addr: ":0"}, boom, quiet)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("the panic value reached the client")
	}
	w = httptest.NewRecorder()
	h.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/fine", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("after a panic, status = %d, want 200", w.Code)
	}
}

func TestProductionRefusesToStartWithoutCertificates(t *testing.T) {
	t.Setenv("FORGE_ENV", "production")
	for _, name := range []string{"TLS_CERT_FILE", "TLS_KEY_FILE", "TLS_CA_FILE"} {
		t.Setenv(name, "")
	}
	if _, err := FromEnv(":8080"); err == nil {
		t.Fatal("production must refuse to start without TLS and a client CA")
	}
	if _, err := New(Config{Addr: ":0", Production: true}, http.HandlerFunc(ok), quiet); err == nil {
		t.Fatal("New must refuse an unvalidated production configuration")
	}
}

func TestDevelopmentNeedsNoCertificates(t *testing.T) {
	t.Setenv("FORGE_ENV", "development")
	for _, name := range []string{"TLS_CERT_FILE", "TLS_KEY_FILE", "TLS_CA_FILE"} {
		t.Setenv(name, "")
	}
	if _, err := FromEnv(":8080"); err != nil {
		t.Fatalf("development without certificates: %v", err)
	}
}

func TestCertificatesMustComeInPairsAndCANeedsACertificate(t *testing.T) {
	cases := map[string]Config{
		"cert without key": {CertFile: "a"},
		"key without cert": {KeyFile: "a"},
		"CA alone":         {CAFile: "a"},
	}
	for name, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestServiceNameNeedsExactlyOneWellFormedIdentity(t *testing.T) {
	u := func(s string) *url.URL { v, _ := url.Parse(s); return v }
	cases := map[string]struct {
		uris []*url.URL
		ok   bool
	}{
		"one identity":       {[]*url.URL{u("spiffe://forge/svc/svc-workspace")}, true},
		"none":               {nil, false},
		"two identities":     {[]*url.URL{u("spiffe://forge/svc/svc-a"), u("spiffe://forge/svc/svc-b")}, false},
		"malformed name":     {[]*url.URL{u("spiffe://forge/svc/Bad_Name")}, false},
		"other trust domain": {[]*url.URL{u("spiffe://elsewhere/svc/svc-a")}, false},
	}
	for name, c := range cases {
		_, err := ServiceName(&x509.Certificate{URIs: c.uris})
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", name, err, c.ok)
		}
	}
}

// startMutual serves handler over mutual TLS and returns its address. The handler sees the caller
// identity in its context.
func startMutual(t *testing.T, ca *pkitest.CA, serverLeaf *pkitest.Leaf, handler http.Handler) string {
	t.Helper()
	dir := t.TempDir()
	certPath, keyPath, caPath := filepath.Join(dir, "srv.crt"), filepath.Join(dir, "srv.key"), filepath.Join(dir, "ca.pem")
	for path, b := range map[string][]byte{certPath: serverLeaf.CertPEM, keyPath: serverLeaf.KeyPEM, caPath: ca.CertPEM} {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	srv, err := New(Config{Addr: "127.0.0.1:0", CertFile: certPath, KeyFile: keyPath, CAFile: caPath, Production: true}, handler, quiet)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(tls.NewListener(ln, srv.TLSConfig)) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "https://" + ln.Addr().String()
}

func clientWith(t *testing.T, ca *pkitest.CA, leaf *pkitest.Leaf) *http.Client {
	t.Helper()
	tc := &tls.Config{RootCAs: ca.Pool(), MinVersion: tls.VersionTLS12}
	if leaf != nil {
		cert, err := tls.X509KeyPair(leaf.CertPEM, leaf.KeyPEM)
		if err != nil {
			t.Fatal(err)
		}
		tc.Certificates = []tls.Certificate{cert}
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: tc}}
}

func TestMutualTLSNamesTheCallerFromItsCertificate(t *testing.T) {
	ca, err := pkitest.NewCA("forge-test")
	if err != nil {
		t.Fatal(err)
	}
	srvLeaf, _ := ca.Issue("party-service")
	callerLeaf, _ := ca.Issue("svc-workspace")

	var seen string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = callerauth.IdentityFrom(r.Context())
		ok(w, r)
	})
	base := startMutual(t, ca, srvLeaf, handler)

	// Even with a spoofed header, the certificate decides who the caller is.
	req, _ := http.NewRequest("GET", base+"/", nil)
	req.Header.Set(callerauth.DefaultHeader, "svc-admin")
	resp, err := clientWith(t, ca, callerLeaf).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if seen != "svc-workspace" {
		t.Fatalf("caller = %q, want the certificate's svc-workspace", seen)
	}
	if resp.Header.Get("Strict-Transport-Security") == "" {
		t.Error("HSTS missing on a TLS response")
	}
}

func TestMutualTLSRefusesClientsWithoutAValidCertificate(t *testing.T) {
	ca, _ := pkitest.NewCA("forge-test")
	other, _ := pkitest.NewCA("someone-else")
	srvLeaf, _ := ca.Issue("party-service")
	forged, _ := other.Issue("svc-workspace")
	base := startMutual(t, ca, srvLeaf, http.HandlerFunc(ok))

	if _, err := clientWith(t, ca, nil).Get(base + "/"); err == nil {
		t.Error("a client with no certificate was served")
	}
	if _, err := clientWith(t, ca, forged).Get(base + "/"); err == nil {
		t.Error("a certificate from another CA was served")
	}
}

func TestOutboundCarriesRequestIDAndRefusesPlainHTTPInProduction(t *testing.T) {
	var gotID string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get(RequestIDHeader)
		ok(w, r)
	}))
	t.Cleanup(backend.Close)

	dev, err := Outbound(Config{})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequestWithContext(context.WithValue(context.Background(), requestIDKey{}, "req-propagated-1"), "GET", backend.URL, nil)
	resp, err := dev.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if gotID != "req-propagated-1" {
		t.Fatalf("outbound request id = %q, want it propagated", gotID)
	}

	prod, err := Outbound(Config{Production: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prod.Get(backend.URL); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("production outbound over plain HTTP: err = %v, want refused", err)
	}
}

func TestProductionRefusesDevelopmentSwitches(t *testing.T) {
	t.Setenv("FORGE_ENV", "production")
	if err := RefuseInProduction(Switch{"LEDGER_STORE=memory", true}, Switch{"POLICY_DEV_UNSIGNED", false}); err == nil || !strings.Contains(err.Error(), "LEDGER_STORE=memory") {
		t.Fatalf("err = %v, want the memory ledger named", err)
	}
	if err := RefuseInProduction(Switch{"POLICY_DEV_UNSIGNED", false}); err != nil {
		t.Fatalf("switches off must start: %v", err)
	}
	t.Setenv("FORGE_ENV", "development")
	if err := RefuseInProduction(Switch{"LEDGER_STORE=memory", true}); err != nil {
		t.Fatalf("development may use development switches: %v", err)
	}
}
