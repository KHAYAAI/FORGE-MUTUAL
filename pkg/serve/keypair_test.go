package serve

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/serve/pkitest"
)

// writePair puts a certificate and key on disk, with a modification time later than whatever was there, as a renewal
// has.
func writePair(t *testing.T, certPath, keyPath string, leaf *pkitest.Leaf, at time.Time) {
	t.Helper()
	for path, b := range map[string][]byte{certPath: leaf.CertPEM, keyPath: leaf.KeyPEM} {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
}

func serial(t *testing.T, cert *tls.Certificate) string {
	t.Helper()
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return parsed.SerialNumber.String()
}

// A certificate that is renewed on disk is presented without a restart, by a server and by a client. A renewal that
// cannot be loaded changes nothing: the service keeps the certificate it has.
func TestARenewedCertificateIsPickedUpWithoutARestart(t *testing.T) {
	ca, err := pkitest.NewCA("forge-test")
	if err != nil {
		t.Fatal(err)
	}
	first, _ := ca.Issue("svc-payments")
	second, _ := ca.Issue("svc-payments")
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "svc.crt"), filepath.Join(dir, "svc.key")
	start := time.Now().Add(-time.Hour)
	writePair(t, certPath, keyPath, first, start)

	clock := start
	kp, err := loadKeyPair(certPath, keyPath, quiet)
	if err != nil {
		t.Fatal(err)
	}
	kp.now = func() time.Time { return clock }
	kp.checked = clock // the pair was loaded "now", on the test's clock
	firstSerial := serial(t, kp.current())

	// Renewed on disk, but the files are not looked at again until the check interval has passed.
	writePair(t, certPath, keyPath, second, start.Add(time.Minute))
	clock = clock.Add(kp.checkEvery - time.Second)
	if got := serial(t, kp.current()); got != firstSerial {
		t.Fatal("the files were read again before the check interval had passed")
	}
	clock = clock.Add(2 * time.Second)
	secondSerial := serial(t, kp.current())
	if secondSerial == firstSerial {
		t.Fatal("the renewed certificate was not picked up")
	}
	fromServer, _ := kp.forServer(nil)
	fromClient, _ := kp.forClient(nil)
	if serial(t, fromServer) != secondSerial || serial(t, fromClient) != secondSerial {
		t.Fatal("the server and client hooks must present the renewed certificate")
	}

	// A renewal caught half-written: the key no longer belongs to the certificate. Keep what works.
	if err := os.WriteFile(keyPath, first.KeyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	later := start.Add(2 * time.Minute)
	if err := os.Chtimes(keyPath, later, later); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(kp.checkEvery + time.Second)
	if got := serial(t, kp.current()); got != secondSerial {
		t.Fatal("a renewal that does not load must not replace a certificate that works")
	}
	// And when the rest of the renewal lands, it is picked up.
	writePair(t, certPath, keyPath, first, start.Add(3*time.Minute))
	clock = clock.Add(kp.checkEvery + time.Second)
	if got := serial(t, kp.current()); got != firstSerial {
		t.Fatal("the completed renewal was not picked up")
	}
}

func TestAServiceDoesNotStartWithoutItsCertificate(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadKeyPair(filepath.Join(dir, "missing.crt"), filepath.Join(dir, "missing.key"), quiet); err == nil {
		t.Fatal("a missing certificate must stop the service from starting")
	}
}

// A real server over mutual TLS presents the renewed certificate to the next connection.
func TestAServerPresentsItsRenewedCertificate(t *testing.T) {
	ca, _ := pkitest.NewCA("forge-test")
	first, _ := ca.Issue("party-service")
	second, _ := ca.Issue("party-service")
	client, _ := ca.Issue("svc-workspace")
	dir := t.TempDir()
	certPath, keyPath, caPath := filepath.Join(dir, "srv.crt"), filepath.Join(dir, "srv.key"), filepath.Join(dir, "ca.pem")
	start := time.Now().Add(-time.Hour)
	writePair(t, certPath, keyPath, first, start)
	if err := os.WriteFile(caPath, ca.CertPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	saved := certCheckEvery
	certCheckEvery = 50 * time.Millisecond
	t.Cleanup(func() { certCheckEvery = saved })
	srv, err := New(Config{Addr: "127.0.0.1:0", CertFile: certPath, KeyFile: keyPath, CAFile: caPath}, http.HandlerFunc(ok), quiet)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(tls.NewListener(ln, srv.TLSConfig)) }()
	t.Cleanup(func() { _ = srv.Close() })

	presented := func() string {
		t.Helper()
		c := clientWith(t, ca, client)
		c.Transport.(*http.Transport).DisableKeepAlives = true
		resp, err := c.Get("https://" + ln.Addr().String() + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.TLS.PeerCertificates[0].SerialNumber.String()
	}
	before := presented()
	writePair(t, certPath, keyPath, second, start.Add(time.Minute))
	for deadline := time.Now().Add(10 * time.Second); presented() == before; {
		if time.Now().After(deadline) {
			t.Fatal("the server still presents its old certificate ten seconds after it was renewed")
		}
		time.Sleep(60 * time.Millisecond)
	}
}

func TestOnlyTheNamedClientsAreServed(t *testing.T) {
	ca, _ := pkitest.NewCA("forge-test")
	srvLeaf, _ := ca.Issue("svc-workspace")
	web, _ := ca.Issue("svc-web")
	payments, _ := ca.Issue("svc-payments")
	dir := t.TempDir()
	cfg := Config{Addr: "127.0.0.1:0", CertFile: filepath.Join(dir, "srv.crt"), KeyFile: filepath.Join(dir, "srv.key"), CAFile: filepath.Join(dir, "ca.pem")}
	for path, b := range map[string][]byte{cfg.CertFile: srvLeaf.CertPEM, cfg.KeyFile: srvLeaf.KeyPEM, cfg.CAFile: ca.CertPEM} {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var served string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = callerauth.IdentityFrom(r.Context())
		ok(w, r)
	})
	h, err := OnlyClients(cfg, "WORKSPACE_CLIENTS", []string{" svc-web ", ""}, inner)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(cfg, h, quiet)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(tls.NewListener(ln, srv.TLSConfig)) }()
	t.Cleanup(func() { _ = srv.Close() })
	base := "https://" + ln.Addr().String() + "/v1/accounts"

	resp, err := clientWith(t, ca, web).Get(base)
	if err != nil || resp.StatusCode != http.StatusOK || served != "svc-web" {
		t.Fatalf("the web app's server must be served: %v %v, served %q", resp, err, served)
	}
	_ = resp.Body.Close()

	// A service the CA signed, but not the one this API is for. Its token or headers do not matter: it is not served.
	served = ""
	req, _ := http.NewRequest("GET", base, nil)
	req.Header.Set("Authorization", "Bearer a-valid-looking-token")
	req.Header.Set("X-Forge-Party", "party-1")
	resp, err = clientWith(t, ca, payments).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || served != "" {
		t.Fatalf("another service was served: status %d, handler saw %q", resp.StatusCode, served)
	}
}

func TestTheClientListIsRequiredInProductionAndNeedsMutualTLS(t *testing.T) {
	inner := http.HandlerFunc(ok)
	mutual := Config{CertFile: "c", KeyFile: "k", CAFile: "ca"}

	if _, err := OnlyClients(Config{Production: true, CertFile: "c", KeyFile: "k", CAFile: "ca"}, "WORKSPACE_CLIENTS", nil, inner); err == nil || !strings.Contains(err.Error(), "production requires WORKSPACE_CLIENTS") {
		t.Fatalf("production with no list: %v", err)
	}
	if _, err := OnlyClients(Config{}, "WORKSPACE_CLIENTS", []string{"svc-web"}, inner); err == nil || !strings.Contains(err.Error(), "needs mutual TLS") {
		t.Fatalf("a list without mutual TLS: %v", err)
	}
	if _, err := OnlyClients(mutual, "WORKSPACE_CLIENTS", []string{"Not A Service"}, inner); err == nil {
		t.Fatal("a malformed name must be refused")
	}
	// Development with no list: unchanged.
	if h, err := OnlyClients(Config{}, "WORKSPACE_CLIENTS", nil, inner); err != nil || h == nil {
		t.Fatalf("development with no list: %v", err)
	}
	t.Setenv("WORKSPACE_CLIENTS", "svc-web, svc-stack-journeys")
	if got := ClientsFromEnv("WORKSPACE_CLIENTS"); len(got) != 2 {
		t.Fatalf("ClientsFromEnv = %v", got)
	}
	t.Setenv("WORKSPACE_CLIENTS", "  ")
	if got := ClientsFromEnv("WORKSPACE_CLIENTS"); got != nil {
		t.Fatalf("an empty setting must be no list, got %v", got)
	}
}
