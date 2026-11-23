// Package fakes provides an in-memory implementation of the onboarding activities. It has the same method
// names and signatures as onboarding.Activities, so a Temporal worker registers it in place of the real
// activities. The workflow itself is unchanged. Failure knobs let tests exercise retries.
//
// Used by the dev-server tests only; never wired into the service binary.
package fakes

import (
	"context"
	"errors"
	"sync"

	"go.temporal.io/sdk/temporal"

	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
)

// Party IDs the fake returns, so tests can name the subjects of a verification.
const (
	CompanyID = "company-1"
	PersonOne = "person-1"
	PersonTwo = "person-2"
)

// Recorder counts activity calls. It is kept outside Activities because Temporal registers every exported
// method of the activity struct, and a counter method would be rejected as an invalid activity.
type Recorder struct {
	mu    sync.Mutex
	calls map[string]int
}

// Calls returns how many times the named activity has run.
func (r *Recorder) Calls(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[name]
}

func (r *Recorder) note(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[name]++
	return r.calls[name]
}

// Activities is the fake. Configure the exported fields before the worker starts.
type Activities struct {
	SectorAllowed   bool
	Hits            []string
	ScreenErr       error
	AccountFailures int  // the first N CreateAccounts calls fail with a retryable error
	AccountFatal    bool // CreateAccounts fails with a non-retryable error

	// Rec counts calls. It is a field, not a method, so Temporal does not register it.
	Rec *Recorder
}

// New returns a fake that allows the sector when sectorAllowed is true.
func New(sectorAllowed bool) *Activities {
	return &Activities{SectorAllowed: sectorAllowed, Rec: &Recorder{calls: map[string]int{}}}
}

func (a *Activities) CreateProspects(_ context.Context, _ onboarding.Input) (onboarding.ProspectsResult, error) {
	a.Rec.note("CreateProspects")
	return onboarding.ProspectsResult{CompanyID: CompanyID, PersonIDs: []string{PersonOne, PersonTwo}}, nil
}

func (a *Activities) CreateRelationships(_ context.Context, _ onboarding.Input, _ onboarding.ProspectsResult) error {
	a.Rec.note("CreateRelationships")
	return nil
}

func (a *Activities) RecordConsents(_ context.Context, _ onboarding.Input, _ onboarding.ProspectsResult) error {
	a.Rec.note("RecordConsents")
	return nil
}

func (a *Activities) StartVerifications(_ context.Context, _ onboarding.Input, _ onboarding.ProspectsResult) error {
	a.Rec.note("StartVerifications")
	return nil
}

func (a *Activities) Screen(_ context.Context, _ onboarding.Input) ([]string, error) {
	a.Rec.note("Screen")
	return a.Hits, a.ScreenErr
}

func (a *Activities) EvaluateSector(_ context.Context, _ onboarding.Input) (onboarding.SectorResult, error) {
	a.Rec.note("EvaluateSector")
	if a.SectorAllowed {
		return onboarding.SectorResult{Allowed: true, Reason: "allowed"}, nil
	}
	return onboarding.SectorResult{Allowed: false, Reason: "sector_not_eligible"}, nil
}

func (a *Activities) SetTier(_ context.Context, _, _ string) error {
	a.Rec.note("SetTier")
	return nil
}

func (a *Activities) CreateAccounts(_ context.Context, _ onboarding.Input, _ string) ([]string, error) {
	n := a.Rec.note("CreateAccounts")
	if a.AccountFatal {
		return nil, temporal.NewNonRetryableApplicationError("ledger refuses account", "Validation", errors.New("fatal"))
	}
	if n <= a.AccountFailures {
		return nil, errors.New("ledger temporarily unavailable")
	}
	return []string{"account-1"}, nil
}

func (a *Activities) ActivateParties(_ context.Context, _ []string) error {
	a.Rec.note("ActivateParties")
	return nil
}

func (a *Activities) CloseParties(_ context.Context, _ []string) error {
	a.Rec.note("CloseParties")
	return nil
}
