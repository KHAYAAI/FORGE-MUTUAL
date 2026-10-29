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
