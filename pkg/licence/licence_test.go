package licence

import (
	"errors"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDevelopmentAllowsEveryProduct(t *testing.T) {
	g, err := FromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range All {
		if err := g.Allow(p); err != nil {
			t.Errorf("%s refused in development: %v", p, err)
		}
	}
}

func TestProductionRefusesAProductNobodyApproved(t *testing.T) {
	g, err := FromEnv(env(map[string]string{"FORGE_ENV": "production", "FORGE_APPROVAL_SAVINGS": "DEC-0031"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Allow(Savings); err != nil {
		t.Fatalf("approved savings refused: %v", err)
	}
	for _, p := range All {
		if p == Savings {
			continue
		}
		if err := g.Allow(p); !errors.Is(err, ErrNotApproved) {
			t.Errorf("%s: err = %v, want ErrNotApproved", p, err)
		}
	}
	if ref, ok := g.Approval(Savings); !ok || ref != "DEC-0031" {
		t.Fatalf("approval = %q, %v", ref, ok)
	}
}

func TestAMalformedApprovalStopsStartUp(t *testing.T) {
	for _, v := range []string{"yes", "true", "DEC-31", "dec-0031", "DEC-0031; DROP"} {
		if _, err := FromEnv(env(map[string]string{"FORGE_APPROVAL_OVERDRAFT": v})); err == nil {
			t.Errorf("approval %q accepted", v)
		}
	}
}
