// Package rates supplies the market rate for a quote. A rate that is too old is refused: a quote on a stale rate would
// misprice the customer. The static source is for development; a real market-rate provider is not chosen (VERIFY).
package rates

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrStale is returned when the rate is older than the freshness limit.
var ErrStale = errors.New("rates: the market rate is too old to quote on")

// Source gives the current mid rate, ZAR per USD, scaled by 10^8.
type Source interface {
	Mid(ctx context.Context) (rate int64, asOf time.Time, err error)
}

// Static is a fixed rate with a timestamp. It is for development and tests.
type Static struct {
	Rate int64
	AsOf time.Time
}

// Mid implements Source.
func (s Static) Mid(context.Context) (int64, time.Time, error) { return s.Rate, s.AsOf, nil }

// Fresh checks that a rate is no older than maxAge at now.
func Fresh(asOf, now time.Time, maxAge time.Duration) error {
	if now.Sub(asOf) > maxAge || asOf.After(now.Add(time.Minute)) {
		return ErrStale
	}
	return nil
}

// ParseScaled reads a decimal rate such as "18.25" into a rate scaled by 10^8, without floating point.
func ParseScaled(s string) (int64, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	if len(frac) > 8 {
		return 0, fmt.Errorf("rates: %q has more than eight decimal places", s)
	}
	frac += strings.Repeat("0", 8-len(frac))
	var w, f int64
	if _, err := fmt.Sscan(whole, &w); err != nil {
		return 0, fmt.Errorf("rates: %q is not a rate", s)
	}
	if _, err := fmt.Sscan(frac, &f); err != nil || w < 0 || f < 0 {
		return 0, fmt.Errorf("rates: %q is not a rate", s)
	}
	v := w*100_000_000 + f
	if v <= 0 {
		return 0, fmt.Errorf("rates: %q must be positive", s)
	}
	return v, nil
}
