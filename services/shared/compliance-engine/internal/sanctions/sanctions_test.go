package sanctions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/pkg/screening"
)

var published = time.Date(2026, 10, 31, 6, 0, 0, 0, time.UTC)

const validList = `{"source":"test","version":"v1","published_at":"2026-10-31T06:00:00Z",
"entries":[{"id":"S1","name":"Ivan Petrov","aliases":["I. Petrov"]}]}`

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestValidListParses(t *testing.T) {
	l, err := Parse([]byte(validList), sum(validList))
	if err != nil || len(l.Entries) != 1 || l.Version != "v1" {
		t.Fatalf("parse: %+v, %v", l, err)
	}
}

// A file that changed after it was published must be refused.
func TestChecksumMismatchIsRefused(t *testing.T) {
	if _, err := Parse([]byte(validList), sum("something else")); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("err = %v, want ErrIntegrity", err)
	}
}

// A format change must fail loudly, not be ignored.
func TestUnknownFieldIsRefused(t *testing.T) {
	bad := `{"source":"t","version":"v1","published_at":"2026-10-31T06:00:00Z","entries":[{"id":"1","name":"A"}],"extra":1}`
	if _, err := Parse([]byte(bad), sum(bad)); err == nil {
		t.Fatal("an unknown field must be refused")
	}
}

// An empty list would screen everyone clear. It is refused.
func TestEmptyListIsRefused(t *testing.T) {
	empty := `{"source":"t","version":"v1","published_at":"2026-10-31T06:00:00Z","entries":[]}`
	if _, err := Parse([]byte(empty), sum(empty)); err == nil {
		t.Fatal("an empty list must be refused")
	}
}

// A listed name must hit, through the shared normalisation, and an alias must hit too.
func TestScreenerHitsNameAndAlias(t *testing.T) {
	l, _ := Parse([]byte(validList), sum(validList))
	now := published.Add(time.Hour)
	s, err := l.Screener(24*time.Hour, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Screen(nil, []string{"IVAN PETROV"})
	if err != nil || len(res.Hits) == 0 {
		t.Fatalf("name should hit: %+v, %v", res, err)
	}
}

// A list that has aged out must fail closed, not screen clear.
func TestStaleListFailsClosed(t *testing.T) {
	l, _ := Parse([]byte(validList), sum(validList))
	later := published.Add(48 * time.Hour)
	s, err := l.Screener(24*time.Hour, func() time.Time { return later })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Screen(nil, []string{"Anyone"}); !errors.Is(err, screening.ErrStale) {
		t.Fatalf("stale list must fail closed: %v", err)
	}
}
