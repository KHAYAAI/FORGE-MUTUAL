package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/khayaai/forge-mutual/pkg/oidc/oidctest"
)

func newVerifier(t *testing.T, is *oidctest.Issuer) *Verifier {
	t.Helper()
	v, err := New(context.Background(), Config{Issuer: is.URL(), Audience: oidctest.Audience, AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestValidRS256AndES256TokensVerify(t *testing.T) {
	is := oidctest.New(t)
	v := newVerifier(t, is)
	ctx := context.Background()

	c, err := v.Verify(ctx, is.RS256(is.Good()))
	if err != nil {
		t.Fatalf("rs256: %v", err)
	}
	if c.Subject != "user-1" || c.Email != "thandi@example.com" || c.Party != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("claims = %+v", c)
	}
	if len(c.Roles) != 2 || c.Roles[0] != "compliance" {
		t.Fatalf("roles = %v", c.Roles)
	}
	if _, err := v.Verify(ctx, is.Token(jwt.SigningMethodES256, is.ECKey(), "ec-1", is.Good())); err != nil {
		t.Fatalf("es256: %v", err)
	}
}

// Roles may be a space-separated string, as some providers send them.
func TestRolesAsAStringAreRead(t *testing.T) {
	is := oidctest.New(t)
	v := newVerifier(t, is)
	cl := is.Good()
	cl["forge_roles"] = "operations  compliance"
	c, err := v.Verify(context.Background(), is.RS256(cl))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Roles) != 2 || c.Roles[0] != "operations" || c.Roles[1] != "compliance" {
		t.Fatalf("roles = %v", c.Roles)
	}
}

func TestTokensThatAreNotOursAreRefused(t *testing.T) {
	is := oidctest.New(t)
	v := newVerifier(t, is)
	ctx := context.Background()
	refused := map[string]string{
		"wrong issuer":                       is.RS256(withClaim(is.Good(), "iss", "https://evil.example")),
		"wrong audience":                     is.RS256(withClaim(is.Good(), "aud", "someone-else")),
		"expired":                            is.RS256(withClaim(is.Good(), "exp", time.Now().Add(-time.Hour).Unix())),
		"no expiry":                          is.RS256(withoutClaim(is.Good(), "exp")),
		"no subject":                         is.RS256(withoutClaim(is.Good(), "sub")),
		"no key id":                          is.Token(jwt.SigningMethodRS256, is.RSAKey(), "", is.Good()),
		"unknown key id":                     is.Token(jwt.SigningMethodRS256, is.RSAKey(), "rsa-9", is.Good()),
		"signed by someone else":             is.Token(jwt.SigningMethodRS256, mustKey(t), "rsa-1", is.Good()),
		"alg none":                           unsigned(is.Good()),
		"hmac with the public key as secret": hmacWithPublicKey(is),
	}
	for name, raw := range refused {
		if _, err := v.Verify(ctx, raw); err == nil {
			t.Errorf("%s: a token that should be refused was accepted", name)
		}
	}
	if _, err := v.Verify(ctx, ""); err == nil {
		t.Error("an empty token was accepted")
	}
}

func withClaim(c jwt.MapClaims, k string, v any) jwt.MapClaims { c[k] = v; return c }
func withoutClaim(c jwt.MapClaims, k string) jwt.MapClaims     { delete(c, k); return c }

func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// unsigned builds a token with alg "none", the classic downgrade.
func unsigned(c jwt.MapClaims) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, c)
	tok.Header["kid"] = "rsa-1"
	s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	return s
}

// hmacWithPublicKey signs with HS256 using the RSA public key's bytes as the secret: the confusion attack.
func hmacWithPublicKey(is *oidctest.Issuer) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, is.Good())
	tok.Header["kid"] = "rsa-1"
	s, _ := tok.SignedString(is.RSAKey().N.Bytes())
	return s
}

// A rotated key is accepted once it is published, and the throttle keeps a flood of unknown ids from refetching.
func TestKeyRotationIsPickedUpWithoutAFlood(t *testing.T) {
	is := oidctest.New(t)
	v := newVerifier(t, is)
	clock := time.Now()
	v.now = func() time.Time { return clock }
	ctx := context.Background()

	if _, err := v.Verify(ctx, is.Token(jwt.SigningMethodRS256, is.RSAKey(), "rsa-2", is.Good())); err == nil {
		t.Fatal("a key not yet published must be refused")
	}
	is.Publish("rsa-2")

	// Still inside the one-minute throttle: the new key is not fetched yet.
	if _, err := v.Verify(ctx, is.Token(jwt.SigningMethodRS256, is.RSAKey(), "rsa-2", is.Good())); err == nil {
		t.Fatal("the throttle must hold the refetch")
	}
	clock = clock.Add(2 * time.Minute)
	if _, err := v.Verify(ctx, is.Token(jwt.SigningMethodRS256, is.RSAKey(), "rsa-2", is.Good())); err != nil {
		t.Fatalf("rotated key after the throttle: %v", err)
	}
}

// An issuer that is down does not turn every request into a fetch: the throttle applies to failures too.
func TestAnOutageDoesNotHammerTheIssuer(t *testing.T) {
	is := oidctest.New(t)
	v := newVerifier(t, is)
	clock := time.Now()
	v.now = func() time.Time { return clock }
	clock = clock.Add(keyCacheTTL + time.Minute) // the cache is stale
	tok := is.RS256(is.Good())
	is.Retire("rsa-1")
	fetches := 0
	v.http = &http.Client{Transport: countingTransport{n: &fetches, base: http.DefaultTransport}, Timeout: time.Second}
	v.cfg.JWKSURL = "http://127.0.0.1:1/jwks" // unreachable
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		_, _ = v.Verify(ctx, tok)
	}
	if fetches > 1 {
		t.Fatalf("%d fetches during an outage; the throttle allows one a minute", fetches)
	}
}

type countingTransport struct {
	n    *int
	base http.RoundTripper
}

func (c countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	*c.n++
	return c.base.RoundTrip(r)
}

// New refuses to start without keys, and refuses plain http unless a test allows it.
func TestNewFailsClosed(t *testing.T) {
	ctx := context.Background()
	if _, err := New(ctx, Config{Issuer: "http://127.0.0.1:1", Audience: oidctest.Audience}); err == nil {
		t.Error("an http issuer without AllowHTTP must be refused")
	}
	if _, err := New(ctx, Config{Issuer: "https://id.example.com", Audience: ""}); err == nil {
		t.Error("an empty audience must be refused")
	}
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer empty.Close()
	if _, err := New(ctx, Config{Issuer: empty.URL, Audience: oidctest.Audience, AllowHTTP: true, JWKSURL: empty.URL + "/jwks"}); err == nil || !strings.Contains(err.Error(), "no usable") {
		t.Errorf("an empty JWKS must fail: %v", err)
	}
	if _, err := New(ctx, Config{Issuer: "http://127.0.0.1:1", Audience: oidctest.Audience, AllowHTTP: true}); err == nil {
		t.Error("an unreachable issuer must fail at start")
	}
}

// Discovery names the issuer; a document that names another issuer is refused.
func TestDiscoveryMustNameTheIssuer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"issuer":"http://other.example","jwks_uri":"http://other.example/jwks"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if _, err := New(context.Background(), Config{Issuer: srv.URL, Audience: oidctest.Audience, AllowHTTP: true}); err == nil {
		t.Fatal("discovery that names another issuer must be refused")
	}
}

var _ = base64.RawURLEncoding
