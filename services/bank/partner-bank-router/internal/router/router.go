// Package router chooses the partner bank for a foreign-currency operation (ARCHITECTURE.md 12.3). It is pure: health
// comes in as outcomes, and a choice comes out. A partner that is down is never chosen. A payment that has already gone
// to a partner stays with that partner, because a second partner could pay the same money again.
package router

import (
	"errors"
	"sort"
)

// State is a partner's health.
type State string

// Health states.
const (
	Healthy  State = "HEALTHY"
	Degraded State = "DEGRADED"
	Down     State = "DOWN"
)

// Thresholds. They are placeholders (VERIFY) until operations sets them from observed partner behaviour.
const (
	DownAfterConsecutive = 3
	DegradedBelowBps     = 9000 // success rate below 90 percent is degraded
	DownBelowBps         = 5000 // below 50 percent is down
	MinSamples           = 5    // with fewer outcomes than this, a partner is healthy: no evidence against it yet
)

// Window is a partner's recent outcomes, oldest first.
type Window struct {
	Outcomes []bool // true is a success
}

// ConsecutiveFailures counts failures at the end of the window.
func (w Window) ConsecutiveFailures() int {
	n := 0
	for i := len(w.Outcomes) - 1; i >= 0 && !w.Outcomes[i]; i-- {
		n++
	}
	return n
}

// SuccessBps is the success rate in basis points, or 10000 with no samples.
func (w Window) SuccessBps() int64 {
	if len(w.Outcomes) == 0 {
		return 10000
	}
	ok := 0
	for _, o := range w.Outcomes {
		if o {
			ok++
		}
	}
	return int64(ok) * 10000 / int64(len(w.Outcomes))
}

// State classifies the window.
func (w Window) State() State {
	if w.ConsecutiveFailures() >= DownAfterConsecutive {
		return Down
	}
	if len(w.Outcomes) < MinSamples {
		return Healthy
	}
	switch rate := w.SuccessBps(); {
	case rate < DownBelowBps:
		return Down
	case rate < DegradedBelowBps:
		return Degraded
	}
	return Healthy
}

// Candidate is a partner that can serve an asset.
type Candidate struct {
	Name    string
	Asset   string
	CostBps int64
	Window  Window
}

// ErrNoPartner is returned when no partner can take the operation. The operation is refused, not sent elsewhere.
var ErrNoPartner = errors.New("router: no partner bank is available for this operation")

// Choose returns the partner for an asset: a healthy one if there is one, else a degraded one, never a down one. Among
// those, the cheapest wins; ties go to the healthier, then to the name, so the choice is stable.
func Choose(asset string, cands []Candidate) (Candidate, error) {
	var usable []Candidate
	for _, c := range cands {
		if c.Asset != asset {
			continue
		}
		if c.Window.State() == Down {
			continue
		}
		usable = append(usable, c)
	}
	if len(usable) == 0 {
		return Candidate{}, ErrNoPartner
	}
	sort.SliceStable(usable, func(i, j int) bool {
		hi, hj := usable[i].Window.State() == Healthy, usable[j].Window.State() == Healthy
		if hi != hj {
			return hi
		}
		if usable[i].CostBps != usable[j].CostBps {
			return usable[i].CostBps < usable[j].CostBps
		}
		return usable[i].Name < usable[j].Name
	})
	return usable[0], nil
}
