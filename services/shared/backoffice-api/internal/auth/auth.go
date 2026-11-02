// Package auth turns a request into a staff session.
//
// Only a development authenticator is built. It trusts two headers, so it is for local use and tests, and it refuses to run
// in production. There is no production staff authenticator yet, so the service does not start without one configured.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/api"
)

// DevStaff trusts X-Forge-Staff (the person's name, up to 64 characters) and X-Forge-Roles (a comma-separated list of
// known roles). A request with no known role has no session.
type DevStaff struct{}

var (
	errNoSession = errors.New("no session")
	errBadRole   = errors.New("unknown role")
)

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

// Select returns the authenticator for a mode. The mode comes from BACKOFFICE_AUTH and the environment from FORGE_ENV.
func Select(mode, env string) (api.Authenticator, error) {
	switch mode {
	case "dev":
		if env == "production" {
			return nil, errors.New("dev header authentication is refused in production")
		}
		return DevStaff{}, nil
	case "":
		return nil, errors.New("BACKOFFICE_AUTH is required; the only mode built is dev, and a production authenticator is not built yet")
	}
	return nil, fmt.Errorf("BACKOFFICE_AUTH %q is not a known mode", mode)
}
