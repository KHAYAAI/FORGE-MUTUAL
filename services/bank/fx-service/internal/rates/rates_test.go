package rates

import (
	"testing"
	"time"
)

func TestParseScaledIsExact(t *testing.T) {
	cases := map[string]int64{"18.25": 1_825_000_000, "18": 1_800_000_000, "0.00000001": 1, "17.1234": 1_712_340_000}
	for in, want := range cases {
		got, err := ParseScaled(in)
		if err != nil || got != want {
			t.Fatalf("ParseScaled(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "x", "1.000000001", "-3", "0"} {
		if _, err := ParseScaled(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

// A rate older than the limit is refused. So is one stamped in the future, which signals a clock fault.
func TestStaleAndFutureRatesAreRefused(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if err := Fresh(now.Add(-5*time.Minute), now, time.Hour); err != nil {
		t.Fatalf("a fresh rate: %v", err)
	}
	if err := Fresh(now.Add(-2*time.Hour), now, time.Hour); err != ErrStale {
		t.Fatalf("a stale rate: %v", err)
	}
	if err := Fresh(now.Add(time.Hour), now, time.Hour); err != ErrStale {
		t.Fatalf("a future rate: %v", err)
	}
}
