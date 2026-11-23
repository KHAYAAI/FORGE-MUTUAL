// Package bootstrap chooses the BankLayer implementation from configuration (ARCHITECTURE.md 5.2, rule 3).
// It is the only code that reads the route, and it refuses what is not built or not allowed.
package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sponsorhttp"
	"gopkg.in/yaml.v3"
)

// Config mirrors config/banklayer.yaml. Secrets are never in the file; the file names the environment variable.
type Config struct {
	Route          string `yaml:"route"`
	Implementation string `yaml:"implementation"`
	Sponsor        struct {
		BaseURLEnv string `yaml:"base_url_env"`
	} `yaml:"sponsor"`
}

// Options carry things the file must not decide.
type Options struct {
	Getenv func(string) string
	// AllowSimulation must be true for the simulator to start. Production never sets it.
	AllowSimulation bool
}

// ErrNotBuilt is returned for routes whose implementation is not built.
var ErrNotBuilt = errors.New("bootstrap: route not built")

// Load parses the file strictly: unknown keys are an error, so a typo cannot silently change the route.
func Load(data []byte) (Config, error) {
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("bootstrap: parse config: %w", err)
	}
	return c, nil
}

// Open returns the configured BankLayer. The own-licence route is refused until ADR-0020 is accepted.
func Open(ctx context.Context, c Config, o Options) (banklayer.BankLayer, error) {
	switch c.Route {
	case "sponsor":
	case "own_licence":
		return nil, fmt.Errorf("%w: own_licence waits for ADR-0020", ErrNotBuilt)
	default:
		return nil, fmt.Errorf("bootstrap: unknown route %q", c.Route)
	}
	switch c.Implementation {
	case "sim":
		if !o.AllowSimulation {
			return nil, errors.New("bootstrap: the simulator is refused unless simulation is explicitly allowed")
		}
		return sim.New(nil), nil
	case "sponsor":
		if c.Sponsor.BaseURLEnv == "" || o.Getenv == nil {
			return nil, errors.New("bootstrap: sponsor needs base_url_env and an environment lookup")
		}
		base := o.Getenv(c.Sponsor.BaseURLEnv)
		if base == "" {
			return nil, fmt.Errorf("bootstrap: %s is not set", c.Sponsor.BaseURLEnv)
		}
		client, err := sponsorhttp.NewClient(ctx, base, nil)
		if err != nil {
			return nil, err
		}
		return client, nil
	default:
		return nil, fmt.Errorf("bootstrap: unknown implementation %q", c.Implementation)
	}
}
