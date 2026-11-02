package devidp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	clientID    = "forge-web"
	redirectURI = "http://127.0.0.1:3100/api/auth/callback"
	verifier    = "test-verifier-with-enough-entropy-0123456789"
)

// noRedirect stops at the redirect, so the test can read the Location the provider sends the browser to.
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	p, err := New("development", Config{
		Issuer: "http://127.0.0.1:9000", Audience: "forge", RedirectURIs: []string{redirectURI},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(p.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func challenge(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// authorizeAndLogin runs the browser steps and returns the code the client receives.
func authorizeAndLogin(t *testing.T, srv *httptest.Server, user, ch string) string {
	t.Helper()
	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirectURI}, "state": {"s1"},
		"code_challenge": {ch}, "code_challenge_method": {"S256"},
	}
	res, err := noRedirect.Get(srv.URL + "/login?" + q.Encode() + "&user=" + user)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil || res.StatusCode != http.StatusFound {
		t.Fatalf("login: status %d location %q", res.StatusCode, res.Header.Get("Location"))
	}
	if loc.Query().Get("state") != "s1" {
		t.Fatal("the state must come back unchanged")
	}
	return loc.Query().Get("code")
}

func exchange(t *testing.T, srv *httptest.Server, code, v string) (int, string) {
	t.Helper()
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
		"client_id": {clientID}, "code_verifier": {v},
	}
	res, err := http.PostForm(srv.URL+"/token", form)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)
	tok, _ := body["access_token"].(string)
	return res.StatusCode, tok
}

// The happy path issues a token for the chosen user, and the code cannot be used twice.
func TestAuthorizationCodeFlowIssuesATokenOnce(t *testing.T) {
	srv := newServer(t)
	code := authorizeAndLogin(t, srv, "thandi", challenge(verifier))
	status, tok := exchange(t, srv, code, verifier)
	if status != http.StatusOK || strings.Count(tok, ".") != 2 {
		t.Fatalf("exchange: status %d token %q", status, tok)
	}
	claims := decodeClaims(t, tok)
	if claims["sub"] != "thandi" || claims["forge_party"] != "11111111-1111-4111-8111-111111111111" || claims["aud"] != "forge" {
		t.Fatalf("claims = %v", claims)
	}
	if status, _ := exchange(t, srv, code, verifier); status == http.StatusOK {
		t.Fatal("a code must work once")
	}
}

// A wrong PKCE verifier is refused, and it uses up the code: the attacker cannot retry with the right one.
func TestWrongVerifierIsRefusedAndUsesUpTheCode(t *testing.T) {
	srv := newServer(t)
	code := authorizeAndLogin(t, srv, "naledi", challenge(verifier))
	if status, _ := exchange(t, srv, code, "not-the-verifier"); status == http.StatusOK {
		t.Fatal("a wrong verifier must be refused")
	}
	if status, _ := exchange(t, srv, code, verifier); status == http.StatusOK {
		t.Fatal("after a refused exchange the code must be gone")
	}
}

// Redirect URIs outside the allowlist, and requests without PKCE, are refused before anyone logs in.
func TestAuthorizeRefusesUnlistedRedirectsAndNoPKCE(t *testing.T) {
	srv := newServer(t)
	base := url.Values{"response_type": {"code"}, "client_id": {clientID}, "state": {"s"}}
	bad := []url.Values{
		{"redirect_uri": {"https://evil.example/cb"}, "code_challenge": {"c"}, "code_challenge_method": {"S256"}},
		{"redirect_uri": {redirectURI}, "code_challenge": {"c"}, "code_challenge_method": {"plain"}},
		{"redirect_uri": {redirectURI}},
	}
	for _, extra := range bad {
		q := url.Values{}
		for k, v := range base {
			q[k] = v
		}
		for k, v := range extra {
			q[k] = v
		}
		res, err := noRedirect.Get(srv.URL + "/authorize?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", extra, res.StatusCode)
		}
	}
}

// Production is refused, and so is a configuration with no redirect allowlist.
func TestNewRefusesProductionAndOpenRedirects(t *testing.T) {
	if _, err := New("production", Config{Issuer: "https://idp.example", Audience: "forge", RedirectURIs: []string{redirectURI}}); err == nil {
		t.Fatal("devidp must refuse production")
	}
	if _, err := New("development", Config{Issuer: "http://127.0.0.1:9000", Audience: "forge"}); err == nil {
		t.Fatal("devidp must refuse to start with no redirect allowlist")
	}
}

func decodeClaims(t *testing.T, tok string) map[string]any {
	t.Helper()
	parts := strings.Split(tok, ".")
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
