package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/khayaai/forge-mutual/pkg/oidc"
	"github.com/khayaai/forge-mutual/pkg/oidc/oidctest"
	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/api"
)

func oidcConfig(is *oidctest.Issuer) oidc.Config {
	return oidc.Config{Issuer: is.URL(), Audience: oidctest.Audience, AllowHTTP: true}
}

func bearerRequest(token string) *http.Request {
	r := httptest.NewRequest("GET", "/v1/cases", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func withRoles(is *oidctest.Issuer, roles any) jwt.MapClaims {
	cl := is.Good()
	cl["forge_roles"] = roles
	return cl
}

// Roles come from the token. Roles the back office does not know are ignored; the known ones are kept.
func TestOIDCRolesComeFromTheToken(t *testing.T) {
	is := oidctest.New(t)
	a, err := Build(context.Background(), Config{Mode: "oidc", Env: "development", OIDC: oidcConfig(is)})
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.Session(bearerRequest(is.RS256(withRoles(is, []string{"compliance", "some-provider-role"}))))
	if err != nil {
		t.Fatal(err)
	}
	if s.Actor != "Thandi" || !s.Roles[api.RoleCompliance] || s.Roles[api.RoleOperations] || len(s.Roles) != 1 {
		t.Fatalf("session = %+v", s)
	}
}

// A token with no role the back office knows has no session, and so does one with no roles at all.
func TestOIDCNeedsAKnownRole(t *testing.T) {
	is := oidctest.New(t)
	a, err := Build(context.Background(), Config{Mode: "oidc", Env: "development", OIDC: oidcConfig(is)})
	if err != nil {
		t.Fatal(err)
	}
	for name, roles := range map[string]any{"unknown only": []string{"admin"}, "none": []string{}, "absent": nil} {
		cl := withRoles(is, roles)
		if roles == nil {
			delete(cl, "forge_roles")
		}
		if _, err := a.Session(bearerRequest(is.RS256(cl))); err == nil {
			t.Errorf("%s: a session was granted", name)
		}
	}
}

func TestOIDCRefusesMissingAndTamperedTokens(t *testing.T) {
	is := oidctest.New(t)
	a, err := Build(context.Background(), Config{Mode: "oidc", Env: "development", OIDC: oidcConfig(is)})
	if err != nil {
		t.Fatal(err)
	}
	good := is.RS256(is.Good())
	for name, r := range map[string]*http.Request{
		"no header": bearerRequest(""),
		"tampered":  bearerRequest(good[:len(good)-4] + "AAAA"),
		"junk":      bearerRequest("not.a.token"),
	} {
		if _, err := a.Session(r); err == nil {
			t.Errorf("%s: a session was granted", name)
		}
	}
}

func TestDevStaffRefusesAnUnknownRole(t *testing.T) {
	r := httptest.NewRequest("GET", "/v1/cases", nil)
	r.Header.Set("X-Forge-Staff", "Naledi")
	r.Header.Set("X-Forge-Roles", "admin")
	if _, err := (DevStaff{}).Session(r); err == nil {
		t.Fatal("an unknown role must be refused")
	}
}

func TestBuildRefusesWhatProductionMustRefuse(t *testing.T) {
	is := oidctest.New(t)
	ctx := context.Background()
	if _, err := Build(ctx, Config{Mode: "dev", Env: "production"}); err == nil {
		t.Error("dev authentication must be refused in production")
	}
	if _, err := Build(ctx, Config{Mode: "oidc", Env: "production", OIDC: oidcConfig(is)}); err == nil {
		t.Error("plain-http OIDC must be refused in production")
	}
	if _, err := Build(ctx, Config{Mode: "", Env: "development"}); err == nil {
		t.Error("no mode configured must refuse to start")
	}
}
