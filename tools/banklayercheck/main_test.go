package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A service that branches on the route must fail the check.
func TestPlantedRouteReadFails(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/payments/x.go", `package payments
func f(c Caps) bool { return c.Route == "ROUTE_SPONSOR" }
`)
	v, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || !strings.Contains(v[0], "reads the licence route") {
		t.Fatalf("want one route violation, got %v", v)
	}
}

// A service that names the route constant must fail the check.
func TestPlantedRouteConstantFails(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/x/x.go", `package x
import "github.com/khayaai/forge-mutual/pkg/banklayer"
var r = banklayer.RouteOwnLicence
`)
	v, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 {
		t.Fatalf("want one violation, got %v", v)
	}
}

// pkg/banklayer may read the route; capabilities used as data are allowed everywhere.
func TestBankLayerAndCapabilityDataAllowed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pkg/banklayer/x.go", `package banklayer
func f(c Capabilities) bool { return c.Route == RouteSponsor }
`)
	write(t, root, "services/bank/y/y.go", `package y
import "github.com/khayaai/forge-mutual/pkg/banklayer"
var c = banklayer.SponsorCapabilities()
var legal = c.LegalBookOfRecord
`)
	v, err := check(root)
	if err != nil || len(v) != 0 {
		t.Fatalf("want no violations, got %v (err %v)", v, err)
	}
}

// Repository root check: the real tree must be clean.
func TestRepositoryIsClean(t *testing.T) {
	v, err := check("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Fatalf("repository has route reads outside pkg/banklayer: %v", v)
	}
}
