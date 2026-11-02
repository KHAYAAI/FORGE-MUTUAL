// Package auth turns a request into a staff session.
//
// In production the only authenticator is OIDC: a bearer token from the identity provider, verified against its keys. The
// roles come from the token's forge_roles claim. A token whose roles include none of the back office's roles is no session,
// and unknown roles from the provider are ignored, since providers add their own. A development authenticator trusts two
// headers and refuses to run in production.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/khayaai/forge-mutual/pkg/oidc"
	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/api"
)

var errNoSession = errors.New("no session")

// Config chooses how requests become staff sessions.
type Config struct {
	Mode string      // "oidc", or "dev" for local use only
	Env  string      // FORGE_ENV; production refuses dev, and refuses plain-http OIDC addresses
	OIDC oidc.Config // used when Mode is "oidc"
}

// Build returns the configured authenticator, or an error when the configuration is not allowed.
func Build(ctx context.Context, c Config) (api.Authenticator, error) {
	switch c.Mode {
	case "oidc":
		if c.Env == "production" && c.OIDC.AllowHTTP {
			return nil, errors.New("OIDC_ALLOW_HTTP is refused in production")
		}
		v, err := oidc.New(ctx, c.OIDC)
		if err != nil {
			return nil, err
		}
		return OIDC{V: v}, nil
	case "dev":
		if c.Env == "production" {
			return nil, errors.New("dev header authentication is refused in production")
		}
		return DevStaff{}, nil
	case "":
		return nil, errors.New("BACKOFFICE_AUTH is required: oidc in every deployed environment, dev for local use")
	}
	return nil, fmt.Errorf("BACKOFFICE_AUTH %q is not a known mode", c.Mode)
}

// OIDC reads the staff session from a verified bearer token.
type OIDC struct {
	V *oidc.Verifier
}

// Session implements api.Authenticator.
func (o OIDC) Session(r *http.Request) (api.Session, error) {
	raw, ok := bearer(r)
	if !ok {
		return api.Session{}, errNoSession
	}
	c, err := o.V.Verify(r.Context(), raw)
	if err != nil {
		return api.Session{}, errNoSession
	}
	roles := map[string]bool{}
	for _, role := range c.Roles {
		switch role {
		case api.RoleCompliance, api.RoleOperations:
			roles[role] = true
		}
	}
	if len(roles) == 0 {
		return api.Session{}, errNoSession
	}
	actor := first(c.Name, c.Email, c.Subject)
	if len(actor) > 64 {
		actor = actor[:64]
	}
	return api.Session{Actor: actor, Roles: roles}, nil
}

func bearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}

func first(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// DevStaff trusts X-Forge-Staff (the person's name, up to 64 characters) and X-Forge-Roles (a comma-separated list of
// known roles). Local use only. An unknown role is refused, not ignored, so a typo cannot quietly withhold access.
type DevStaff struct{}

var errBadRole = errors.New("unknown role")

// Session implements api.Authenticator.
func (DevStaff) Session(r *http.Request) (api.Session, error) {
	actor := strings.TrimSpace(r.Header.Get("X-Forge-Staff"))
	if actor == "" || len(actor) > 64 {
		return api.Session{}, errNoSession
	}
	roles := map[string]bool{}
	for _, role := range strings.Split(r.Header.Get("X-Forge-Roles"), ",") {
		role = strings.TrimSpace(role)
		switch role {
		case "":
		case api.RoleCompliance, api.RoleOperations:
			roles[role] = true
		default:
			return api.Session{}, errBadRole
		}
	}
	if len(roles) == 0 {
		return api.Session{}, errNoSession
	}
	return api.Session{Actor: actor, Roles: roles}, nil
}
