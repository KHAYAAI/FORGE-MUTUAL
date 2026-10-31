package bootstrap_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sponsorhttp"
)

func loadRepoConfig(t *testing.T) bootstrap.Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "banklayer.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := bootstrap.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRepoConfigOpensSponsorClient(t *testing.T) {
	srv := httptest.NewServer(sponsorhttp.Handler(sim.New(nil)))
	t.Cleanup(srv.Close)
	env := map[string]string{"SPONSOR_BASE_URL": srv.URL}
	bl, err := bootstrap.Open(context.Background(), loadRepoConfig(t), bootstrap.Options{
		Getenv: func(k string) string { return env[k] },
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if bl.Capabilities().Route != banklayer.RouteSponsor {
		t.Fatalf("route = %q", bl.Capabilities().Route)
	}
}

func TestOwnLicenceIsRefused(t *testing.T) {
	c := bootstrap.Config{Route: "own_licence", Implementation: "sponsor"}
	if _, err := bootstrap.Open(context.Background(), c, bootstrap.Options{}); !errors.Is(err, bootstrap.ErrNotBuilt) {
		t.Fatalf("err = %v, want ErrNotBuilt", err)
	}
}

func TestSimulatorRefusedUnlessAllowed(t *testing.T) {
	c := bootstrap.Config{Route: "sponsor", Implementation: "sim"}
	if _, err := bootstrap.Open(context.Background(), c, bootstrap.Options{}); err == nil {
		t.Fatal("simulator started without AllowSimulation")
	}
	if _, err := bootstrap.Open(context.Background(), c, bootstrap.Options{AllowSimulation: true}); err != nil {
		t.Fatalf("simulator with AllowSimulation: %v", err)
	}
}

func TestUnknownKeysAreRejected(t *testing.T) {
	if _, err := bootstrap.Load([]byte("route: sponsor\nimplementaton: sim\n")); err == nil {
		t.Fatal("a typo in a key must not be ignored")
	}
}

func TestMissingEnvIsRefused(t *testing.T) {
	c := bootstrap.Config{Route: "sponsor", Implementation: "sponsor"}
	c.Sponsor.BaseURLEnv = "SPONSOR_BASE_URL"
	if _, err := bootstrap.Open(context.Background(), c, bootstrap.Options{Getenv: func(string) string { return "" }}); err == nil {
		t.Fatal("sponsor client started with no base URL")
	}
}
