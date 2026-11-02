// Package auth turns a request into a session.
//
// In production the only authenticator is OIDC: a bearer token from the identity provider, verified against the provider's
// keys. The party comes from the token's forge_party claim and nothing else. A development authenticator trusts two headers
// and refuses to run in production.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/khayaai/forge-mutual/pkg/oidc"
	"github.com/khayaai/forge-mutual/services/platform/workspace-api/internal/api"
)

var errNoSession = errors.New("no session")

// Config chooses how requests become sessions.
type Config struct {
	Mode string      // "oidc", or "dev" for local use only
	Env  string      // FORGE_ENV; production refuses dev, and refuses plain-http OIDC addresses
	OIDC oidc.Config // used when Mode is "oidc"
}

// Build returns the configured authenticator, or an error when the configuration is not allowed. The service does not
// start without one.
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
		return DevHeaders{}, nil
	case "":
		return nil, errors.New("WORKSPACE_AUTH is required: oidc in every deployed environment, dev for local use")
	}
	return nil, fmt.Errorf("WORKSPACE_AUTH %q is not a known mode", c.Mode)
}

// OIDC reads the session from a verified bearer token.
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
	if !api.ValidParty(c.Party) {
		return api.Session{}, errNoSession
	}
	actor := first(c.Name, c.Email, c.Subject)
	if len(actor) > 64 {
		actor = actor[:64]
	}
	return api.Session{Party: c.Party, Actor: actor}, nil
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

// DevHeaders trusts X-Forge-Party (a canonical UUID) and X-Forge-Actor (a name, up to 64 characters). Local use only.
type DevHeaders struct{}

// Session implements api.Authenticator.
func (DevHeaders) Session(r *http.Request) (api.Session, error) {
	party := strings.TrimSpace(r.Header.Get("X-Forge-Party"))
	actor := strings.TrimSpace(r.Header.Get("X-Forge-Actor"))
	if !api.ValidParty(party) || actor == "" || len(actor) > 64 {
		return api.Session{}, errNoSession
	}
	return api.Session{Party: party, Actor: actor}, nil
}
