// Package oidctest is a fake OpenID Connect issuer for tests. It serves discovery and a JWKS whose keys a test can publish,
// retire or rotate, and it signs tokens with them. Production code never imports it.
package oidctest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Audience is the audience tokens from this issuer are for, unless a test changes it.
const Audience = "forge-test"

// Issuer is a running fake issuer.
type Issuer struct {
	t      testing.TB
	srv    *httptest.Server
	rsaKey *rsa.PrivateKey
	ecKey  *ecdsa.PrivateKey

	mu        sync.Mutex
	published map[string]bool
}

// New starts an issuer with two published keys: "rsa-1" (RS256) and "ec-1" (ES256).
func New(t testing.TB) *Issuer {
	t.Helper()
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ek, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	is := &Issuer{t: t, rsaKey: rk, ecKey: ek, published: map[string]bool{"rsa-1": true, "ec-1": true}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": is.URL(), "jwks_uri": is.URL() + "/jwks"})
	})
	mux.HandleFunc("/jwks", is.serveJWKS)
	is.srv = httptest.NewServer(mux)
	t.Cleanup(is.srv.Close)
	return is
}

// URL is the issuer's address, which is also its iss claim.
func (is *Issuer) URL() string { return is.srv.URL }

// Publish adds a key to the JWKS. Retire removes one, which is how a test models a rotated-out key.
func (is *Issuer) Publish(kid string) { is.setPublished(kid, true) }
func (is *Issuer) Retire(kid string)  { is.setPublished(kid, false) }

func (is *Issuer) setPublished(kid string, on bool) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.published[kid] = on
}

// RSAKey and ECKey are the signing keys, for tests that sign with a published key.
func (is *Issuer) RSAKey() *rsa.PrivateKey  { return is.rsaKey }
func (is *Issuer) ECKey() *ecdsa.PrivateKey { return is.ecKey }

// Good returns claims for a valid token: a subject, a party, two roles, and an expiry ten minutes away.
func (is *Issuer) Good() jwt.MapClaims {
	return jwt.MapClaims{
		"iss": is.URL(), "aud": Audience, "sub": "user-1", "email": "thandi@example.com", "name": "Thandi",
		"forge_party": "11111111-1111-4111-8111-111111111111", "forge_roles": []string{"compliance", "operations"},
		"iat": time.Now().Unix(), "exp": time.Now().Add(10 * time.Minute).Unix(),
	}
}

// Token signs claims with the given key and key id.
func (is *Issuer) Token(method jwt.SigningMethod, key any, kid string, claims jwt.MapClaims) string {
	is.t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		is.t.Fatal(err)
	}
	return s
}

// RS256 signs with the issuer's RSA key under "rsa-1", the usual case.
func (is *Issuer) RS256(claims jwt.MapClaims) string {
	return is.Token(jwt.SigningMethodRS256, is.rsaKey, "rsa-1", claims)
}

func (is *Issuer) serveJWKS(w http.ResponseWriter, _ *http.Request) {
	is.mu.Lock()
	defer is.mu.Unlock()
	var keys []map[string]string
	if is.published["rsa-1"] {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": "rsa-1", "use": "sig", "alg": "RS256",
			"n": b64(is.rsaKey.N.Bytes()), "e": b64(big.NewInt(int64(is.rsaKey.E)).Bytes())})
	}
	if is.published["ec-1"] {
		keys = append(keys, map[string]string{"kty": "EC", "kid": "ec-1", "use": "sig", "crv": "P-256",
			"x": b64(is.ecKey.X.Bytes()), "y": b64(is.ecKey.Y.Bytes())})
	}
	if is.published["rsa-2"] {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": "rsa-2", "use": "sig",
			"n": b64(is.rsaKey.N.Bytes()), "e": b64(big.NewInt(int64(is.rsaKey.E)).Bytes())})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
