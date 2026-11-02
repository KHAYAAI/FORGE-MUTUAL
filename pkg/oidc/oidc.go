// Package oidc verifies bearer tokens issued by an OpenID Connect provider.
//
// It is provider-agnostic. The issuer and audience come from configuration. The signing keys come from the issuer's JWKS,
// found through discovery unless a JWKS address is set. Only RS256 and ES256 are accepted, and the key is looked up by the
// token's key id, so the "none" algorithm and the HMAC-with-public-key confusion are both refused.
package oidc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claim names the platform reads from a token, besides the standard ones.
const (
	ClaimParty = "forge_party" // the customer's party id, a canonical UUID
	ClaimRoles = "forge_roles" // back-office roles: a list, or a space-separated string
)

const (
	keyCacheTTL = 10 * time.Minute
	minRefetch  = time.Minute
	maxBody     = 1 << 20
)

// Config says whose tokens are accepted.
type Config struct {
	Issuer   string        // required, for example https://id.example.com
	Audience string        // required; the token's aud must include it
	JWKSURL  string        // optional; discovered from the issuer when empty
	Leeway   time.Duration // clock skew allowed; default 30 seconds
	// AllowHTTP permits http:// addresses. Only tests set it: production addresses must be https.
	AllowHTTP  bool
	HTTPClient *http.Client
}

// Claims is what a verified token says about its holder.
type Claims struct {
	Subject string
	Email   string
	Name    string
	Party   string
	Roles   []string
}

// Verifier checks tokens against one issuer's keys.
type Verifier struct {
	cfg  Config
	http *http.Client
	now  func() time.Time

	mu          sync.Mutex
	keys        map[string]crypto.PublicKey
	lastFetch   time.Time // last successful load
	lastAttempt time.Time // last load, successful or not; throttles refetches
}

// New validates the configuration and loads the signing keys. It fails when the keys cannot be loaded, so a service
// never starts without them.
func New(ctx context.Context, cfg Config) (*Verifier, error) {
	if err := checkURL("issuer", cfg.Issuer, cfg.AllowHTTP); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Audience) == "" {
		return nil, errors.New("oidc: audience is required")
	}
	if cfg.Leeway == 0 {
		cfg.Leeway = 30 * time.Second
	}
	v := &Verifier{cfg: cfg, http: cfg.HTTPClient, now: time.Now, keys: map[string]crypto.PublicKey{}}
	if v.http == nil {
		v.http = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.JWKSURL == "" {
		jwks, err := v.discover(ctx)
		if err != nil {
			return nil, err
		}
		cfg.JWKSURL = jwks
		v.cfg.JWKSURL = jwks
	}
	if err := checkURL("jwks", v.cfg.JWKSURL, cfg.AllowHTTP); err != nil {
		return nil, err
	}
	if err := v.load(ctx); err != nil {
		return nil, err
	}
	return v, nil
}

// Verify returns the claims of a token that is signed by the issuer, is for this audience, is unexpired, and was issued
// by this issuer. Anything else is an error.
func (v *Verifier) Verify(ctx context.Context, raw string) (Claims, error) {
	if raw == "" {
		return Claims{}, errors.New("oidc: no token")
	}
	keyfunc := func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("oidc: token has no key id")
		}
		key, err := v.key(ctx, kid)
		if err != nil {
			return nil, err
		}
		return key, nil
	}
	tok, err := jwt.Parse(raw, keyfunc,
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithAudience(v.cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(v.cfg.Leeway),
		jwt.WithTimeFunc(v.now),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("oidc: token refused: %w", err)
	}
	mc, ok := tok.Claims.(jwt.MapClaims)
	if !ok || !tok.Valid {
		return Claims{}, errors.New("oidc: token refused")
	}
	return claimsOf(mc)
}

func claimsOf(mc jwt.MapClaims) (Claims, error) {
	sub, _ := mc["sub"].(string)
	if sub == "" {
		return Claims{}, errors.New("oidc: token has no subject")
	}
	c := Claims{Subject: sub}
	c.Email, _ = mc["email"].(string)
	c.Name, _ = mc["name"].(string)
	c.Party, _ = mc[ClaimParty].(string)
	switch r := mc[ClaimRoles].(type) {
	case []any:
		for _, x := range r {
			if s, ok := x.(string); ok {
				c.Roles = append(c.Roles, s)
			}
		}
	case string:
		c.Roles = strings.Fields(r)
	}
	return c, nil
}

// key returns the public key for a key id. An unknown id triggers one refetch, at most once a minute, so a rotated key
// works without letting a flood of bad tokens hammer the issuer.
func (v *Verifier) key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.Lock()
	k, ok := v.keys[kid]
	stale := v.now().Sub(v.lastFetch) > keyCacheTTL
	v.mu.Unlock()
	if ok && !stale {
		return k, nil
	}
	if !ok || stale {
		if err := v.refetch(ctx); err != nil && !ok {
			return nil, err
		}
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, errors.New("oidc: unknown signing key")
}

func (v *Verifier) refetch(ctx context.Context) error {
	v.mu.Lock()
	recent := v.now().Sub(v.lastAttempt) < minRefetch
	v.mu.Unlock()
	if recent {
		return nil
	}
	return v.load(ctx)
}

func (v *Verifier) load(ctx context.Context) error {
	v.mu.Lock()
	v.lastAttempt = v.now()
	v.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return err
	}
	res, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("oidc: jwks: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("oidc: jwks: status %d", res.StatusCode)
	}
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&doc); err != nil {
		return fmt.Errorf("oidc: jwks: %w", err)
	}
	keys := map[string]crypto.PublicKey{}
	for _, j := range doc.Keys {
		k, err := j.publicKey()
		if err != nil || j.Kid == "" {
			continue
		}
		keys[j.Kid] = k
	}
	if len(keys) == 0 {
		return errors.New("oidc: jwks holds no usable signing keys")
	}
	v.mu.Lock()
	v.keys = keys
	v.lastFetch = v.now()
	v.mu.Unlock()
	return nil
}

func (v *Verifier) discover(ctx context.Context) (string, error) {
	u := strings.TrimSuffix(v.cfg.Issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	res, err := v.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("oidc: discovery: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oidc: discovery: status %d", res.StatusCode)
	}
	var doc struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&doc); err != nil {
		return "", fmt.Errorf("oidc: discovery: %w", err)
	}
	if doc.Issuer != v.cfg.Issuer {
		return "", fmt.Errorf("oidc: discovery names issuer %q, expected %q", doc.Issuer, v.cfg.Issuer)
	}
	if doc.JWKSURI == "" {
		return "", errors.New("oidc: discovery names no jwks_uri")
	}
	return doc.JWKSURI, nil
}

func checkURL(what, raw string, allowHTTP bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("oidc: %s must be an absolute URL", what)
	}
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
		return fmt.Errorf("oidc: %s must use https", what)
	}
	return nil
}

// jwk is one key of a JWKS document. Only RSA and P-256 keys are read.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (j jwk) publicKey() (crypto.PublicKey, error) {
	if j.Use != "" && j.Use != "sig" {
		return nil, errors.New("not a signing key")
	}
	switch j.Kty {
	case "RSA":
		n, err := b64big(j.N)
		if err != nil {
			return nil, err
		}
		e, err := b64big(j.E)
		if err != nil {
			return nil, err
		}
		if !e.IsInt64() || e.Int64() < 3 {
			return nil, errors.New("bad exponent")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		if j.Crv != "P-256" {
			return nil, errors.New("unsupported curve")
		}
		x, err := b64big(j.X)
		if err != nil {
			return nil, err
		}
		y, err := b64big(j.Y)
		if err != nil {
			return nil, err
		}
		pk := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
		if !pk.Curve.IsOnCurve(x, y) {
			return nil, errors.New("point not on curve")
		}
		return pk, nil
	}
	return nil, errors.New("unsupported key type")
}

func b64big(s string) (*big.Int, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil, errors.New("bad encoding")
	}
	return new(big.Int).SetBytes(b), nil
}
