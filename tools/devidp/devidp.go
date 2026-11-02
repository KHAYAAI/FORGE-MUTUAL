// Package devidp is a development OpenID Connect provider. It lets the platform's login run and be tested end to end
// without a commercial identity provider. It is not for production: it refuses to start when FORGE_ENV is production,
// it signs tokens with a key it creates at start, and its users are configuration.
//
// It implements the authorization code flow with PKCE (S256), single-use codes, a redirect URI allowlist, and discovery
// and JWKS endpoints. It does not implement refresh tokens, consent, or client authentication.
package devidp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// User is someone who can log in. Party and Roles are optional claims: a customer has a party, staff have roles.
type User struct {
	Sub   string   `json:"sub"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Party string   `json:"forge_party,omitempty"`
	Roles []string `json:"forge_roles,omitempty"`
}

// DefaultUsers are the accounts a local stack and the tests use: a customer and two staff members, each with one role.
var DefaultUsers = []User{
	{Sub: "thandi", Name: "Thandi Dube", Email: "thandi@example.test", Party: "11111111-1111-4111-8111-111111111111"},
	{Sub: "naledi", Name: "Naledi Mokoena", Email: "naledi@example.test", Roles: []string{"compliance"}},
	{Sub: "sipho", Name: "Sipho Ndlovu", Email: "sipho@example.test", Roles: []string{"operations"}},
}

// Config says how the provider behaves.
type Config struct {
	Issuer       string   // the URL the apps use for the provider; it is the iss claim
	Audience     string   // the aud claim of every access token
	RedirectURIs []string // the only redirect URIs a client may name
	Users        []User
	AccessTTL    time.Duration // default 15 minutes
	CodeTTL      time.Duration // default 2 minutes
	Now          func() time.Time
	signingKey   *rsa.PrivateKey
	kid          string
}

type code struct {
	user        User
	clientID    string
	redirectURI string
	challenge   string
	expires     time.Time
}

// Provider serves the endpoints.
type Provider struct {
	cfg   Config
	mu    sync.Mutex
	codes map[string]code
}

// New validates the configuration and creates the signing key. It refuses production.
func New(env string, cfg Config) (*Provider, error) {
	if env == "production" {
		return nil, errors.New("devidp refuses to run in production")
	}
	if cfg.Issuer == "" || cfg.Audience == "" {
		return nil, errors.New("devidp: issuer and audience are required")
	}
	if len(cfg.RedirectURIs) == 0 {
		return nil, errors.New("devidp: at least one redirect URI must be allowed")
	}
	if len(cfg.Users) == 0 {
		cfg.Users = DefaultUsers
	}
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.CodeTTL == 0 {
		cfg.CodeTTL = 2 * time.Minute
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	cfg.signingKey = k
	cfg.kid = "devidp-1"
	return &Provider{cfg: cfg, codes: map[string]code{}}, nil
}

// Handler returns the endpoints.
func (p *Provider) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /jwks", p.jwks)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("GET /login", p.login)
	mux.HandleFunc("POST /token", p.token)
	return mux
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                p.cfg.Issuer,
		"authorization_endpoint":                p.cfg.Issuer + "/authorize",
		"token_endpoint":                        p.cfg.Issuer + "/token",
		"jwks_uri":                              p.cfg.Issuer + "/jwks",
		"response_types_supported":              []string{"code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := p.cfg.signingKey.PublicKey
	writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "kid": p.cfg.kid, "use": "sig", "alg": "RS256",
		"n": b64(pub.N.Bytes()), "e": b64(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

// authorize shows the users to choose from. The request must name an allowed redirect URI and use PKCE with S256.
func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := p.checkAuthorizeRequest(q); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>Development sign-in</title></head><body>")
	b.WriteString("<h1>Development sign-in</h1><p>Choose a user. This page exists only for local development.</p><ul>")
	for _, u := range p.cfg.Users {
		login := url.Values{}
		for k, v := range q {
			login[k] = v
		}
		login.Set("user", u.Sub)
		fmt.Fprintf(&b, `<li><a href="/login?%s">%s</a> (%s)</li>`,
			html.EscapeString(login.Encode()), html.EscapeString(u.Name), html.EscapeString(u.Sub))
	}
	b.WriteString("</ul></body></html>")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// login issues a single-use code for the chosen user and redirects back to the client with it.
func (p *Provider) login(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := p.checkAuthorizeRequest(q); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var chosen *User
	for i := range p.cfg.Users {
		if p.cfg.Users[i].Sub == q.Get("user") {
			chosen = &p.cfg.Users[i]
		}
	}
	if chosen == nil {
		http.Error(w, "unknown user", http.StatusBadRequest)
		return
	}
	c, err := randomString(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	p.mu.Lock()
	p.codes[c] = code{
		user: *chosen, clientID: q.Get("client_id"), redirectURI: q.Get("redirect_uri"),
		challenge: q.Get("code_challenge"), expires: p.cfg.Now().Add(p.cfg.CodeTTL),
	}
	p.mu.Unlock()
	back, _ := url.Parse(q.Get("redirect_uri"))
	back.RawQuery = url.Values{"code": {c}, "state": {q.Get("state")}}.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

// token exchanges a code for an access token. A code works once, only for the client and redirect URI it was issued
// to, and only with the PKCE verifier that matches its challenge.
func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, "invalid_request")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		writeOAuthError(w, "unsupported_grant_type")
		return
	}
	p.mu.Lock()
	c, ok := p.codes[r.PostForm.Get("code")]
	delete(p.codes, r.PostForm.Get("code")) // single use, even when the exchange fails
	p.mu.Unlock()
	if !ok || p.cfg.Now().After(c.expires) {
		writeOAuthError(w, "invalid_grant")
		return
	}
	if r.PostForm.Get("client_id") != c.clientID || r.PostForm.Get("redirect_uri") != c.redirectURI {
		writeOAuthError(w, "invalid_grant")
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	want := b64(sum[:])
	if subtle.ConstantTimeCompare([]byte(want), []byte(c.challenge)) != 1 {
		writeOAuthError(w, "invalid_grant")
		return
	}
	tok, err := p.sign(c)
	if err != nil {
		writeOAuthError(w, "server_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": tok, "token_type": "Bearer", "expires_in": int(p.cfg.AccessTTL.Seconds()),
	})
}

func (p *Provider) sign(c code) (string, error) {
	now := p.cfg.Now()
	claims := map[string]any{
		"iss": p.cfg.Issuer, "aud": p.cfg.Audience, "sub": c.user.Sub, "name": c.user.Name, "email": c.user.Email,
		"azp": c.clientID, "iat": now.Unix(), "exp": now.Add(p.cfg.AccessTTL).Unix(),
	}
	if c.user.Party != "" {
		claims["forge_party"] = c.user.Party
	}
	if len(c.user.Roles) > 0 {
		claims["forge_roles"] = c.user.Roles
	}
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": p.cfg.kid}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signing := b64(hb) + "." + b64(pb)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.cfg.signingKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signing + "." + b64(sig), nil
}

func (p *Provider) checkAuthorizeRequest(q url.Values) error {
	if q.Get("response_type") != "code" {
		return errors.New("response_type must be code")
	}
	if q.Get("client_id") == "" || q.Get("state") == "" {
		return errors.New("client_id and state are required")
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		return errors.New("PKCE with S256 is required")
	}
	for _, allowed := range p.cfg.RedirectURIs {
		if q.Get("redirect_uri") == allowed {
			return nil
		}
	}
	return errors.New("redirect_uri is not allowed")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOAuthError(w http.ResponseWriter, code string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": code})
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b64(b), nil
}
