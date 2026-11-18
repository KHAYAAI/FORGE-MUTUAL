// Package screening is the sanctions and PEP check that onboarding calls before activation
// (ARCHITECTURE.md 15.1). This build uses a fixture list with a freshness rule. The real compliance
// engine (P1-06) replaces it. A stale list fails closed.
package screening

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrStale is returned when the list is older than the maximum age. Screening must not pass on a stale list.
var ErrStale = errors.New("screening: list is stale")

// Result is the outcome of screening a set of names.
type Result struct {
	Hits []string // names that matched an entry on the list
}

// Screener checks names.
type Screener interface {
	Screen(ctx context.Context, names []string) (Result, error)
}

// Fixture screens against an in-memory list of normalised names, and refuses to screen when the list
// is older than MaxAge.
type Fixture struct {
	list   map[string]bool
	asOf   time.Time
	maxAge time.Duration
	now    func() time.Time
}

// NewFixture returns a screener over names. asOf is when the list was current.
func NewFixture(names []string, asOf time.Time, maxAge time.Duration, now func() time.Time) (*Fixture, error) {
	if maxAge <= 0 {
		return nil, errors.New("screening: maxAge must be positive")
	}
	f := &Fixture{list: map[string]bool{}, asOf: asOf, maxAge: maxAge, now: now}
	for _, n := range names {
		f.list[normalise(n)] = true
	}
	return f, nil
}

// Screen implements Screener.
func (f *Fixture) Screen(_ context.Context, names []string) (Result, error) {
	if f.now().Sub(f.asOf) > f.maxAge {
		return Result{}, fmt.Errorf("%w: as of %s", ErrStale, f.asOf.Format(time.RFC3339))
	}
	var hits []string
	for _, n := range names {
		if f.list[normalise(n)] {
			hits = append(hits, n)
		}
	}
	return Result{Hits: hits}, nil
}

// normalise folds case and spacing so "Jane  Doe" and "jane doe" match.
func normalise(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
