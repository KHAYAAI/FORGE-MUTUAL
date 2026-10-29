package engine

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/tester"

	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/bundle"
)

// policyDir is the repository's policy bundle, relative to this package.
const policyDir = "../../../../../policy"

// TestRegoRules runs every *_test.rego in the bundle with OPA's tester. The data comes from the
// same loader the service uses, so a rule that passes here sees the same tiers, sectors and
// entity assets it will see in production. (tester.Run does not load the root JSON files as data.)
func TestRegoRules(t *testing.T) {
	ctx := context.Background()
	b, err := bundle.Load(policyDir, nil, true)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	mods := map[string]*ast.Module{}
	for name, src := range b.Modules {
		m, err := ast.ParseModule(name, src)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		mods[name] = m
	}
	err = filepath.WalkDir(policyDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.rego") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		m, err := ast.ParseModule(path, string(src))
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		mods[path] = m
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := tester.NewRunner().SetStore(inmem.NewFromObject(b.Data)).Run(ctx, mods)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for r := range results {
		count++
		if !r.Pass() {
			t.Errorf("%s.%s failed (error: %v)", r.Package, r.Name, r.Error)
		}
	}
	if count == 0 {
		t.Fatal("no rego tests found")
	}
}

// copyBundle copies the bundle's signed files into a fresh directory, so tests can sign it.
func copyBundle(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(policyDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(policyDir, path)
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, src)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func signedEngine(t *testing.T) *Engine {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := copyBundle(t)
	if err := bundle.Sign(dir, "test-2026.10.29", priv); err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Load(dir, []ed25519.PublicKey{pub}, false)
	if err != nil {
		t.Fatalf("load signed bundle: %v", err)
	}
	e, err := Compile(context.Background(), b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return e
}

func limitsInput(amount string) map[string]any {
	return map[string]any{
		"party":        map[string]any{"status": "PARTY_STATUS_ACTIVE", "tier": "standard"},
		"screening":    map[string]any{"hit": false},
		"recon":        map[string]any{"break_active": false},
		"asset":        "ZAR",
		"amount_minor": amount,
		"velocity":     map[string]any{"day_total_minor": "0"},
	}
}

func TestDecisions(t *testing.T) {
	e := signedEngine(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		point string
		input map[string]any
		want  Decision
	}{
		{"limits small payment", "limits", limitsInput("1000"), Decision{Allow: true, Reason: "ok"}},
		{"limits above stepup", "limits", limitsInput("20000000"), Decision{Allow: true, StepUp: true, Reason: "above_stepup"}},
		{"limits above per-txn", "limits", limitsInput("999999999999"), Decision{Reason: "default_deny"}},
		{"limits empty input fails closed", "limits", map[string]any{}, Decision{Reason: "default_deny"}},
		{"assets BANK holds ZAR", "assets", map[string]any{"entity_code": "BANK", "asset": "ZAR"}, Decision{Allow: true, Reason: "ok"}},
		{"assets BANK refuses stablecoin", "assets", map[string]any{"entity_code": "BANK", "asset": "USDC"}, Decision{Reason: "default_deny"}},
		{"entity same-entity transfer", "entity", map[string]any{"entity_code": "BANK", "debit_entity": "BANK", "credit_entity": "BANK", "code": 10}, Decision{Allow: true, Reason: "ok"}},
		{"entity cross-entity refused", "entity", map[string]any{"entity_code": "BANK", "debit_entity": "BANK", "credit_entity": "DAST", "code": 10}, Decision{Reason: "default_deny"}},
		{"recon no break", "recon", map[string]any{"recon": map[string]any{"break_active": false}, "amount_minor": "5"}, Decision{Allow: true, Reason: "no_break"}},
		{"recon break holds large payment", "recon", map[string]any{"recon": map[string]any{"break_active": true, "threshold_minor": "100"}, "amount_minor": "101"}, Decision{Reason: "recon_break_active"}},
		{"sector software allowed", "sector", map[string]any{"sector_code": "software_saas"}, Decision{Allow: true, Reason: "allowed"}},
		{"sector defence denied", "sector", map[string]any{"sector_code": "defence"}, Decision{Reason: "sector_not_eligible"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := e.Evaluate(ctx, tc.point, tc.input)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestUnknownPointIsAnError(t *testing.T) {
	e := signedEngine(t)
	if _, err := e.Evaluate(context.Background(), "payments", map[string]any{}); err == nil {
		t.Fatal("unknown decision point must be an error")
	}
	if Known("payments") || !Known("limits") {
		t.Fatal("Known disagrees with the decision points")
	}
}

func TestVersionIsReported(t *testing.T) {
	if got := signedEngine(t).Version(); got != "test-2026.10.29" {
		t.Fatalf("version = %q", got)
	}
}
