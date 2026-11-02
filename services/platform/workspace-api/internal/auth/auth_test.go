package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/khayaai/forge-mutual/pkg/oidc"
	"github.com/khayaai/forge-mutual/pkg/oidc/oidctest"
)

func oidcConfig(is *oidctest.Issuer) oidc.Config {
	return oidc.Config{Issuer: is.URL(), Audience: oidctest.Audience, AllowHTTP: true}
}

func bearerRequest(token string) *http.Request {
	r := httptest.NewRequest("GET", "/v1/accounts", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

// A valid token gives the party in its forge_party claim, and the name as the actor.
func TestOIDCSessionComesFromTheToken(t *testing.T) {
	is := oidctest.New(t)
	a, err := Build(context.Background(), Config{Mode: "oidc", Env: "development", OIDC: oidcConfig(is)})
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.Session(bearerRequest(is.RS256(is.Good())))
	if err != nil {
		t.Fatal(err)
	}
	if s.Party != "11111111-1111-4111-8111-111111111111" || s.Actor != "Thandi" {
		t.Fatalf("session = %+v", s)
	}
}

// A token with no party, or a party that is not a UUID, is no session.
func TestOIDCRefusesTokensWithoutAValidParty(t *testing.T) {
	is := oidctest.New(t)
	a, err := Build(context.Background(), Config{Mode: "oidc", Env: "development", OIDC: oidcConfig(is)})
	if err != nil {
		t.Fatal(err)
	}
	noParty := is.Good()
	delete(noParty, "forge_party")
	badParty := is.Good()
	badParty["forge_party"] = "not-a-uuid"
	for name, cl := range map[string]jwt.MapClaims{"no party": noParty, "bad party": badParty} {
		if _, err := a.Session(bearerRequest(is.RS256(cl))); err == nil {
			t.Errorf("%s: a session was granted", name)
		}
	}
}

func TestOIDCRefusesMissingTamperedAndWrongSchemeTokens(t *testing.T) {
	is := oidctest.New(t)
	a, err := Build(context.Background(), Config{Mode: "oidc", Env: "development", OIDC: oidcConfig(is)})
	if err != nil {
		t.Fatal(err)
	}
	good := is.RS256(is.Good())
	tampered := good[:len(good)-4] + "AAAA"
	cases := map[string]*http.Request{
		"no header":   bearerRequest(""),
		"tampered":    bearerRequest(tampered),
		"basic auth":  basicRequest(),
		"junk bearer": bearerRequest("not.a.token"),
	}
	for name, r := range cases {
		if _, err := a.Session(r); err == nil {
			t.Errorf("%s: a session was granted", name)
		}
	}
}

func basicRequest() *http.Request {
	r := httptest.NewRequest("GET", "/v1/accounts", nil)
	r.Header.Set("Authorization", "Basic dGhhbmRpOnNlY3JldA==")
	return r
}

// Dev headers and plain-http OIDC are refused in production; a service with no mode does not start.
func TestBuildRefusesWhatProductionMustRefuse(t *testing.T) {
	is := oidctest.New(t)
	ctx := context.Background()
	if _, err := Build(ctx, Config{Mode: "dev", Env: "production"}); err == nil {
		t.Error("dev authentication must be refused in production")
	}
	if _, err := Build(ctx, Config{Mode: "dev", Env: "development"}); err != nil {
		t.Errorf("dev in development: %v", err)
	}
	httpCfg := oidcConfig(is)
	if _, err := Build(ctx, Config{Mode: "oidc", Env: "production", OIDC: httpCfg}); err == nil {
		t.Error("plain-http OIDC must be refused in production")
	}
	if _, err := Build(ctx, Config{Mode: "", Env: "development"}); err == nil {
		t.Error("no mode configured must refuse to start")
	}
	if _, err := Build(ctx, Config{Mode: "oidc", Env: "development", OIDC: oidc.Config{Issuer: "https://id.example.com", Audience: "x"}}); err == nil {
		t.Error("an unreachable https issuer must fail at start, not at the first request")
	}
}
