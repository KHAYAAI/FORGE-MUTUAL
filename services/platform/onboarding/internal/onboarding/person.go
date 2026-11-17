package onboarding

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
)

// PersonInput starts the onboarding of one individual.
type PersonInput struct {
	OnboardingID     string // canonical UUID; also the business transaction id in the ledger
	RequestHash      string
	LegalName        string
	NationalID       string
	Country          string
	Actor            string
	ApplicantSubject string // the identity provider's subject of the applicant, when they applied themselves
	// KYCTimeoutSeconds overrides KYCTimeout when positive. Tests only.
	KYCTimeoutSeconds int64
}

// asInput is the business Input that the shared activities and the verdict wait read: they use the onboarding ID,
// the actor and the timeout, and nothing about a company.
func (in PersonInput) asInput() Input {
	return Input{OnboardingID: in.OnboardingID, Actor: in.Actor, KYCTimeoutSeconds: in.KYCTimeoutSeconds, LegalName: in.LegalName}
}

// OnboardPerson onboards an individual: the person as a prospect, their consents, identity verification (the same
// signed verdicts and review queue as a business), sanctions and PEP screening, then a personal ZAR account, and
// activation. A decline closes the party and creates nothing in the ledger.
func OnboardPerson(ctx workflow.Context, in PersonInput) (Status, error) {
	st := Status{OnboardingID: in.OnboardingID, RequestHash: in.RequestHash, State: StateRunning, Kind: "person"}
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
		if err := workflow.ExecuteActivity(actx, acts.CloseParties, st.PersonPartyIDs).Get(actx, nil); err != nil {
			return fail(err)
		}
		st.Steps = append(st.Steps, "parties_closed")
		return st, nil
	}

	var person string
	if err := workflow.ExecuteActivity(actx, acts.CreatePerson, in).Get(actx, &person); err != nil {
		return fail(err)
	}
	st.PersonPartyIDs = []string{person}
	st.Steps = append(st.Steps, "prospects_created")

	if err := workflow.ExecuteActivity(actx, acts.RecordPersonConsents, in, person).Get(actx, nil); err != nil {
		return fail(err)
	}
	st.Steps = append(st.Steps, "consents_recorded")

	if err := workflow.ExecuteActivity(actx, acts.StartVerifications, in.asInput(), ProspectsResult{PersonIDs: []string{person}}).Get(actx, nil); err != nil {
		return fail(err)
	}
	st.Steps = append(st.Steps, "verifications_started")

	if reason, err := awaitVerdicts(ctx, actx, in.asInput(), []string{person}); err != nil {
		return fail(err)
	} else if reason != "" {
		return decline(reason)
	}
	st.Steps = append(st.Steps, "kyc_approved")

	var hits []string
	if err := workflow.ExecuteActivity(actx, acts.Screen, in.asInput()).Get(actx, &hits); err != nil {
		return decline("screening_unavailable")
	}
	if len(hits) > 0 {
		return decline("screening_match")
	}
	st.Steps = append(st.Steps, "screened")

	st.Tier = "standard"
	if err := workflow.ExecuteActivity(actx, acts.SetTier, person, st.Tier).Get(actx, nil); err != nil {
		return fail(err)
	}
	var accounts []string
	if err := workflow.ExecuteActivity(actx, acts.CreatePersonAccount, in, person).Get(actx, &accounts); err != nil {
		return fail(err)
	}
	st.AccountIDs = accounts
	st.Steps = append(st.Steps, "accounts_created")

	if err := workflow.ExecuteActivity(actx, acts.ActivateParties, st.PersonPartyIDs).Get(actx, nil); err != nil {
		return fail(err)
	}
	st.Steps = append(st.Steps, "activated")
	st.State = StateCompleted
	return st, nil
}

// CreatePerson creates the individual as a PROSPECT party. The key derives from the onboarding ID, so a retry returns
// the same party.
func (a *Activities) CreatePerson(ctx context.Context, in PersonInput) (string, error) {
	p, err := a.Party.CreateParty(ctx, connect.NewRequest(&partyv1.CreatePartyRequest{
		IdempotencyKey: in.OnboardingID + ":person:0",
		Type:           partyv1.PartyType_PARTY_TYPE_PERSON,
		LegalName:      in.LegalName,
		NationalId:     in.NationalID,
		Country:        in.Country,
		Actor:          in.Actor,
	}))
	if err != nil {
		return "", retryable(err)
	}
	return p.Msg.Party.PartyId, nil
}

// RecordPersonConsents records the individual's agreement to the terms and to identity verification.
func (a *Activities) RecordPersonConsents(ctx context.Context, in PersonInput, person string) error {
	for _, g := range []struct {
		purpose         string
		scopes, sources []string
	}{
		{"onboarding.terms", []string{"account.opening"}, []string{"onboarding_form"}},
		{"kyc.processing", []string{"identity.verification"}, []string{"identity_provider"}},
	} {
		_, err := a.Consent.Grant(ctx, connect.NewRequest(&consentv1.GrantRequest{
			IdempotencyKey: in.OnboardingID + ":consent:" + g.purpose + ":" + person,
			PartyId:        person, Purpose: g.purpose, Scopes: g.scopes, DataSources: g.sources,
			EvidenceRef: "onboarding:" + in.OnboardingID, Actor: in.Actor,
		}))
		if err != nil {
			return retryable(err)
		}
	}
	return nil
}

// CreatePersonAccount opens the individual's ZAR account. The key is the onboarding ID, so a retry returns it.
func (a *Activities) CreatePersonAccount(ctx context.Context, in PersonInput, person string) ([]string, error) {
	res, err := a.Ledger.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: in.OnboardingID + ":accounts",
		BusinessTxnId:  in.OnboardingID,
		EntityCode:     "BANK",
		Accounts: []*ledgerv1.AccountSpec{{
			OwnerPartyId: person, Asset: "ZAR", Code: 1001, ProductId: "personal-zar-current",
			DebitsMustNotExceedCredits: true, Insured: a.InsuredDeposits,
		}},
		Actor:  "onboarding",
		Reason: "personal onboarding " + in.OnboardingID,
	}))
	if err != nil {
		return nil, retryable(err)
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("ledger refused account creation: %v", res.Msg.RejectionCodes), ErrTypeValidation, nil)
	}
	return res.Msg.AccountIds, nil
}
