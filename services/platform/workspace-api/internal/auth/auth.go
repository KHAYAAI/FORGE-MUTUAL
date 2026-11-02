// Package auth turns a request into a session.
//
// Only a development authenticator is built. It trusts two headers, so it is for local use and tests. It refuses to run
// in production. There is no production authenticator yet, so the service does not start without one configured.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/khayaai/forge-mutual/services/platform/workspace-api/internal/api"
)

// DevHeaders trusts X-Forge-Party (a canonical UUID) and X-Forge-Actor (a name, up to 64 characters).
type DevHeaders struct{}

var errNoSession = errors.New("no session")

// Session implements api.Authenticator.
func (DevHeaders) Session(r *http.Request) (api.Session, error) {
	party := strings.TrimSpace(r.Header.Get("X-Forge-Party"))
	actor := strings.TrimSpace(r.Header.Get("X-Forge-Actor"))
	if !api.ValidParty(party) || actor == "" || len(actor) > 64 {
		return api.Session{}, errNoSession
	}
	return api.Session{Party: party, Actor: actor}, nil
}

// Select returns the authenticator for a mode. The mode comes from WORKSPACE_AUTH and the environment from FORGE_ENV.
func Select(mode, env string) (api.Authenticator, error) {
	switch mode {
	case "dev":
		if env == "production" {
			return nil, errors.New("dev header authentication is refused in production")
		}
		return DevHeaders{}, nil
	case "":
		return nil, errors.New("WORKSPACE_AUTH is required; the only mode built is dev, and a production authenticator is not built yet")
	}
	return nil, fmt.Errorf("WORKSPACE_AUTH %q is not a known mode", mode)
}
