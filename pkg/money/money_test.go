package money

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestParseRejectsNonIntegerForms(t *testing.T) {
	for _, s := range []string{"", "12.50", "1e3", "+5", "0x10", " 5"} {
		if _, err := Parse("ZAR", s); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("Parse(%q): want ErrInvalidAmount, got %v", s, err)
		}
	}
}

func TestUnknownAssetRejected(t *testing.T) {
	if _, err := New("BTC", 1); !errors.Is(err, ErrUnknownAsset) {
		t.Fatalf("want ErrUnknownAsset, got %v", err)
	}
}

func TestAssetMismatch(t *testing.T) {
	a, _ := New("ZAR", 1)
	b, _ := New("USD", 1)
	if _, err := a.Add(b); !errors.Is(err, ErrAssetMismatch) {
		t.Fatalf("want ErrAssetMismatch, got %v", err)
	}
}

func TestOverflowDetected(t *testing.T) {
	a, _ := New("ZAR", math.MaxInt64)
	b, _ := New("ZAR", 1)
	if _, err := a.Add(b); !errors.Is(err, ErrOverflow) {
		t.Fatalf("Add: want ErrOverflow, got %v", err)
	}
	n, _ := New("ZAR", math.MinInt64)
	if _, err := n.Sub(b); !errors.Is(err, ErrOverflow) {
		t.Fatalf("Sub: want ErrOverflow, got %v", err)
	}
}

// Property: addition then subtraction returns the original value, and
// the sum of random legs equals the sum computed in a different order (conservation).
func TestPropertyConservation(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		var legs []Money
		var total Money
		total, _ = New("ZAR", 0)
		for j := 0; j < 8; j++ {
			m, _ := New("ZAR", r.Int63n(1_000_000)-500_000)
			legs = append(legs, m)
			total, _ = total.Add(m)
		}
		var rev Money
		rev, _ = New("ZAR", 0)
		for k := len(legs) - 1; k >= 0; k-- {
			rev, _ = rev.Add(legs[k])
		}
		if total.Minor() != rev.Minor() {
			t.Fatalf("conservation broken: %d vs %d", total.Minor(), rev.Minor())
		}
		back, _ := total.Sub(total)
		if back.Minor() != 0 {
			t.Fatalf("x - x != 0")
		}
	}
}

func TestNonNegative(t *testing.T) {
	m, _ := New("USD", -1)
	if err := m.NonNegative(); !errors.Is(err, ErrNegativeAmount) {
		t.Fatalf("want ErrNegativeAmount, got %v", err)
	}
}
