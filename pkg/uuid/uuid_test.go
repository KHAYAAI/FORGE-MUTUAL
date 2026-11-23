package uuid

import (
	"regexp"
	"testing"
)

func TestNewV4Format(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, err := NewV4()
		if err != nil || !re.MatchString(id) {
			t.Fatalf("bad uuid %q: %v", id, err)
		}
		if seen[id] {
			t.Fatal("duplicate uuid")
		}
		seen[id] = true
	}
}

func TestNewV5IsStableAndNamed(t *testing.T) {
	ns := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	a, err := NewV5(ns, "onboarding-key-1")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewV5(ns, "onboarding-key-1")
	c, _ := NewV5(ns, "onboarding-key-2")
	if a != b || a == c {
		t.Fatalf("v5 must be stable per name: %s %s %s", a, b, c)
	}
	// Version 5 and the RFC variant are set.
	if a[14] != '5' || (a[19] != '8' && a[19] != '9' && a[19] != 'a' && a[19] != 'b') {
		t.Fatalf("bad version or variant: %s", a)
	}
	if _, err := Parse(a); err != nil {
		t.Fatal(err)
	}
}
