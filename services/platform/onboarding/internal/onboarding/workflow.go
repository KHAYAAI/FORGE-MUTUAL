// Package onboarding is the business onboarding workflow (ARCHITECTURE.md 11.3). It is a Temporal workflow:
// deterministic, with every side effect in an activity. Each activity is safe to retry.
package onboarding

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Signal and query names.
const (
	SignalKYCResult = "kyc-result"
	QueryStatus     = "status"
)

// KYCTimeout is how long onboarding waits for every verification before declining.
const KYCTimeout = 7 * 24 * time.Hour

// Terminal states reported in Status.State.
const (
	StateRunning   = "RUNNING"
	StateCompleted = "COMPLETED"
	StateDeclined  = "DECLINED"
	StateFailed    = "FAILED"
)

// DirectorInput is a person named on the application.
type DirectorInput struct {
	LegalName    string
	NationalID   string
	OwnershipBps uint32
	IsUBO        bool
}

// Input starts one onboarding.
type Input struct {
	OnboardingID       string // canonical UUID; also the business transaction id in the ledger
	RequestHash        string
	LegalName          string
	RegistrationNumber string
	Country            string
	SectorCode         string
	Directors          []DirectorInput
	Actor              string
	// KYCTimeoutSeconds overrides KYCTimeout when positive. The RPC never sets it. Tests use it to
	// exercise the real timer on a dev server without waiting seven days.
	KYCTimeoutSeconds int64
}

// KYCSignal is delivered by the webhook receiver when a provider reports a verdict.
type KYCSignal struct {
	SubjectPartyID string
	Verdict        string // APPROVED or DECLINED
	SessionID      string
}

// Status is the workflow's state. It is returned as the final result and by the status query.
type Status struct {
	OnboardingID   string
	RequestHash    string
	State          string
	Steps          []string
	CompanyPartyID string
	PersonPartyIDs []string
	AccountIDs     []string
	Tier           string
	DeclineReason  string
}

// Activity options: retries are bounded, and an activity that cannot succeed fails the workflow
// rather than retrying forever.
var activityOptions = temporal.RetryPolicy{
	InitialInterval:    time.Second,
	BackoffCoefficient: 2,
	MaximumInterval:    time.Minute,
	MaximumAttempts:    5,
	NonRetryableErrorTypes: []string{
		ErrTypeNotConfigured,
		ErrTypeValidation,
	},
}

// OnboardBusiness runs the onboarding saga. A decline before accounts are created leaves nothing in the
// ledger, and closes the parties. An error that exhausts its retries marks the run FAILED and returns the
// error, so the state shows how far it got.
func OnboardBusiness(ctx workflow.Context, in Input) (Status, error) {
	st := Status{OnboardingID: in.OnboardingID, RequestHash: in.RequestHash, State: StateRunning}
	if err := workflow.SetQueryHandler(ctx, QueryStatus, func() (Status, error) { return st, nil }); err != nil {
		return st, err
	}
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &activityOptions,
	})
	var acts *Activities

	fail := func(err error) (Status, error) {
		st.State = StateFailed
		st.DeclineReason = err.Error()
		return st, err
	}
	decline := func(reason string) (Status, error) {
		st.State = StateDeclined
		st.DeclineReason = reason
		if err := workflow.ExecuteActivity(actx, acts.CloseParties, st.AllParties()).Get(actx, nil); err != nil {
			return fail(err)
		}
		st.Steps = append(st.Steps, "parties_closed")
		return st, nil
	}

	// 1. Create the company and each director as prospects. Idempotent by key.
	var pro ProspectsResult
	if err := workflow.ExecuteActivity(actx, acts.CreateProspects, in).Get(actx, &pro); err != nil {
		return fail(err)
	}
	st.CompanyPartyID, st.PersonPartyIDs = pro.CompanyID, pro.PersonIDs
	st.Steps = append(st.Steps, "prospects_created")

	// 1b. Link each director to the company. These relationships are what the policy's
	// person-to-entity rule (transfer code 63) checks.
	if err := workflow.ExecuteActivity(actx, acts.CreateRelationships, in, pro).Get(actx, nil); err != nil {
		return fail(err)
	}
	st.Steps = append(st.Steps, "relationships_created")

	// 2. Record the consents that cover the checks that follow.
	if err := workflow.ExecuteActivity(actx, acts.RecordConsents, in, pro).Get(actx, nil); err != nil {
		return fail(err)
	}
	st.Steps = append(st.Steps, "consents_recorded")

	// 3. Move parties to KYC_PENDING and start the verifications.
	if err := workflow.ExecuteActivity(actx, acts.StartVerifications, in, pro).Get(actx, nil); err != nil {
		return fail(err)
	}
	st.Steps = append(st.Steps, "verifications_started")

	// 4. Wait for a verdict on every subject. Any decline, or the timeout, declines the application.
	pending := map[string]bool{st.CompanyPartyID: true}
	for _, id := range st.PersonPartyIDs {
		pending[id] = true
	}
	signals := workflow.GetSignalChannel(ctx, SignalKYCResult)
	kycTimeout := KYCTimeout
	if in.KYCTimeoutSeconds > 0 {
		kycTimeout = time.Duration(in.KYCTimeoutSeconds) * time.Second
	}
	timer := workflow.NewTimer(ctx, kycTimeout)
	for len(pending) > 0 {
		var sig KYCSignal
		timedOut := false
		sel := workflow.NewSelector(ctx)
		sel.AddReceive(signals, func(c workflow.ReceiveChannel, _ bool) { c.Receive(ctx, &sig) })
		sel.AddFuture(timer, func(workflow.Future) { timedOut = true })
		sel.Select(ctx)
		if timedOut {
			return decline("kyc_timeout")
		}
		switch sig.Verdict {
		case "DECLINED":
			return decline("kyc_declined:" + sig.SubjectPartyID)
		case "APPROVED":
			delete(pending, sig.SubjectPartyID)
		}
	}
	st.Steps = append(st.Steps, "kyc_approved")

	// 5. Sanctions and PEP screening. An unavailable list declines: it never passes.
	var hits []string
	if err := workflow.ExecuteActivity(actx, acts.Screen, in).Get(actx, &hits); err != nil {
		return decline("screening_unavailable")
	}
	if len(hits) > 0 {
		return decline("screening_match")
	}
	st.Steps = append(st.Steps, "screened")

	// 6. Sector eligibility from policy-service. Only listed sectors pass (ADR-0012 open).
	var sector SectorResult
	if err := workflow.ExecuteActivity(actx, acts.EvaluateSector, in).Get(actx, &sector); err != nil {
		return decline("sector_check_unavailable")
	}
	if !sector.Allowed {
		return decline("sector_not_eligible:" + sector.Reason)
	}
	st.Steps = append(st.Steps, "sector_eligible")

	// 7. Tier, then the company's ledger account. These are the first money-side effects.
	st.Tier = AssessTier(in)
	if err := workflow.ExecuteActivity(actx, acts.SetTier, st.CompanyPartyID, st.Tier).Get(actx, nil); err != nil {
		return fail(err)
	}
	var accounts []string
	if err := workflow.ExecuteActivity(actx, acts.CreateAccounts, in, st.CompanyPartyID).Get(actx, &accounts); err != nil {
		return fail(err)
	}
	st.AccountIDs = accounts
	st.Steps = append(st.Steps, "accounts_created")

	// 8. Activate.
	if err := workflow.ExecuteActivity(actx, acts.ActivateParties, st.AllParties()).Get(actx, nil); err != nil {
		return fail(err)
	}
	st.Steps = append(st.Steps, "activated")
	st.State = StateCompleted
	return st, nil
}

func (s Status) AllParties() []string {
	return append([]string{s.CompanyPartyID}, s.PersonPartyIDs...)
}

// ErrType names are the non-retryable error types activities may return.
const (
	ErrTypeNotConfigured = "NotConfigured"
	ErrTypeValidation    = "Validation"
)
