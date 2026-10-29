package onboarding

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/protobuf/types/known/structpb"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	policyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/screening"
)

// ProspectsResult carries the party IDs created in the first step.
type ProspectsResult struct {
	CompanyID string
	PersonIDs []string
}

// SectorResult is the policy decision on the company's sector.
type SectorResult struct {
	Allowed bool
	Reason  string
}

// Activities holds the clients and providers the workflow's side effects use. Each method is one
// activity; every method is idempotent with respect to its inputs.
type Activities struct {
	Party           partyv1connect.PartyServiceClient
	Consent         consentv1connect.ConsentServiceClient
	Policy          policyv1connect.PolicyServiceClient
	Ledger          ledgerv1connect.LedgerServiceClient
	KYC             kyc.Provider
	Screener        screening.Screener
	InsuredDeposits bool   // whether customer deposit accounts are marked insured (VERIFY CODI)
	PolicyEntity    string // entity code for policy decisions made on behalf of onboarding
}

// CreateProspects creates the company and each director as PROSPECT parties. The keys are derived from
// the onboarding ID, so a retry returns the same parties.
func (a *Activities) CreateProspects(ctx context.Context, in Input) (ProspectsResult, error) {
	var out ProspectsResult
	co, err := a.Party.CreateParty(ctx, connect.NewRequest(&partyv1.CreatePartyRequest{
		IdempotencyKey:     in.OnboardingID + ":company",
		Type:               partyv1.PartyType_PARTY_TYPE_COMPANY,
		LegalName:          in.LegalName,
		RegistrationNumber: in.RegistrationNumber,
		Country:            in.Country,
		Actor:              in.Actor,
	}))
	if err != nil {
		return out, retryable(err)
	}
	out.CompanyID = co.Msg.Party.PartyId
	for i, d := range in.Directors {
		p, err := a.Party.CreateParty(ctx, connect.NewRequest(&partyv1.CreatePartyRequest{
			IdempotencyKey: in.OnboardingID + ":person:" + strconv.Itoa(i),
			Type:           partyv1.PartyType_PARTY_TYPE_PERSON,
			LegalName:      d.LegalName,
			NationalId:     d.NationalID,
			Country:        in.Country,
			Actor:          in.Actor,
		}))
		if err != nil {
			return out, retryable(err)
		}
		out.PersonIDs = append(out.PersonIDs, p.Msg.Party.PartyId)
	}
	return out, nil
}

// CreateRelationships links each director to the company as a director, and as a shareholder or UBO when the
// application gives them a share. The keys derive from the onboarding ID, so a retry is a replay.
func (a *Activities) CreateRelationships(ctx context.Context, in Input, pro ProspectsResult) error {
	if len(pro.PersonIDs) != len(in.Directors) {
		return temporal.NewNonRetryableApplicationError("director count does not match created parties", ErrTypeValidation, nil)
	}
	for i, d := range in.Directors {
		person := pro.PersonIDs[i]
		idx := strconv.Itoa(i)
		if err := a.addRelationship(ctx, in.OnboardingID+":rel:director:"+idx, person, pro.CompanyID,
			partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, 0); err != nil {
			return err
		}
		if d.OwnershipBps == 0 {
			continue
		}
		rt := partyv1.RelationshipType_RELATIONSHIP_TYPE_SHAREHOLDER
		if d.IsUBO {
			rt = partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO
		}
		if err := a.addRelationship(ctx, in.OnboardingID+":rel:"+rt.String()+":"+idx, person, pro.CompanyID, rt, d.OwnershipBps); err != nil {
			return err
		}
	}
	return nil
}

func (a *Activities) addRelationship(ctx context.Context, key, from, to string, rt partyv1.RelationshipType, bps uint32) error {
	_, err := a.Party.AddRelationship(ctx, connect.NewRequest(&partyv1.AddRelationshipRequest{
		IdempotencyKey: key, FromPartyId: from, ToPartyId: to, Type: rt, PercentageBps: bps, Actor: "onboarding",
	}))
	return retryable(err)
}

// RecordConsents records the consents the checks rely on. The company agrees to the terms and to
// registry and director checks; each director agrees to identity verification.
func (a *Activities) RecordConsents(ctx context.Context, in Input, pro ProspectsResult) error {
	type grant struct {
		party, purpose  string
		scopes, sources []string
	}
	evidence := "onboarding:" + in.OnboardingID
	grants := []grant{
		{pro.CompanyID, "onboarding.terms", []string{"account.opening"}, []string{"onboarding_form"}},
		{pro.CompanyID, "kyb.processing", []string{"company.registry", "company.directors"}, []string{"company_registry"}},
	}
	for _, id := range pro.PersonIDs {
		grants = append(grants, grant{id, "kyc.processing", []string{"identity.verification"}, []string{"identity_provider"}})
	}
	for _, g := range grants {
		_, err := a.Consent.Grant(ctx, connect.NewRequest(&consentv1.GrantRequest{
			IdempotencyKey: in.OnboardingID + ":consent:" + g.purpose + ":" + g.party,
			PartyId:        g.party,
			Purpose:        g.purpose,
			Scopes:         g.scopes,
			DataSources:    g.sources,
			EvidenceRef:    evidence,
			Actor:          in.Actor,
		}))
		if err != nil {
			return retryable(err)
		}
	}
	return nil
}

// StartVerifications moves every party to KYC_PENDING and starts its verification with the provider.
func (a *Activities) StartVerifications(ctx context.Context, in Input, pro ProspectsResult) error {
	if err := a.setStatus(ctx, pro.CompanyID, partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING, in.Actor); err != nil {
		return err
	}
	if _, err := a.KYC.StartBusinessVerification(ctx, in.OnboardingID, pro.CompanyID); err != nil {
		return providerErr(err)
	}
	for _, id := range pro.PersonIDs {
		if err := a.setStatus(ctx, id, partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING, in.Actor); err != nil {
			return err
		}
		if _, err := a.KYC.StartPersonVerification(ctx, in.OnboardingID, id); err != nil {
			return providerErr(err)
		}
	}
	return nil
}

// Screen checks the company and every director against the sanctions and PEP list. It returns the names
// that matched.
func (a *Activities) Screen(ctx context.Context, in Input) ([]string, error) {
	names := []string{in.LegalName}
	for _, d := range in.Directors {
		names = append(names, d.LegalName)
	}
	res, err := a.Screener.Screen(ctx, names)
	if err != nil {
		return nil, retryable(err)
	}
	return res.Hits, nil
}

// EvaluateSector asks policy-service whether the company's sector is eligible.
func (a *Activities) EvaluateSector(ctx context.Context, in Input) (SectorResult, error) {
	input, err := structpb.NewStruct(map[string]any{"sector_code": in.SectorCode})
	if err != nil {
		return SectorResult{}, err
	}
	res, err := a.Policy.Evaluate(ctx, connect.NewRequest(&policyv1.EvaluateRequest{
		DecisionPoint: "sector",
		EntityCode:    a.PolicyEntity,
		RequestId:     in.OnboardingID,
		Input:         input,
	}))
	if err != nil {
		return SectorResult{}, retryable(err)
	}
	return SectorResult{Allowed: res.Msg.Allow, Reason: res.Msg.Reason}, nil
}

// SetTier sets the company's tier label. Setting the same tier again changes nothing.
func (a *Activities) SetTier(ctx context.Context, companyID, tier string) error {
	_, err := a.Party.SetTier(ctx, connect.NewRequest(&partyv1.SetTierRequest{PartyId: companyID, Tier: tier, Actor: "onboarding"}))
	return retryable(err)
}

// CreateAccounts opens the company's ZAR current account in the ledger. The ledger's idempotency key is
// the onboarding ID, so a retry returns the same account.
func (a *Activities) CreateAccounts(ctx context.Context, in Input, companyID string) ([]string, error) {
	res, err := a.Ledger.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: in.OnboardingID + ":accounts",
		BusinessTxnId:  in.OnboardingID,
		EntityCode:     "BANK",
		Accounts: []*ledgerv1.AccountSpec{{
			OwnerPartyId:               companyID,
			Asset:                      "ZAR",
			Code:                       1001, // CUSTOMER_DEPOSIT
			ProductId:                  "business-zar-current",
			DebitsMustNotExceedCredits: true,
			Insured:                    a.InsuredDeposits,
		}},
		Actor:  "onboarding",
		Reason: "business onboarding " + in.OnboardingID,
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

// ActivateParties moves every party from KYC_PENDING to ACTIVE.
func (a *Activities) ActivateParties(ctx context.Context, partyIDs []string) error {
	for _, id := range partyIDs {
		if err := a.setStatus(ctx, id, partyv1.PartyStatus_PARTY_STATUS_ACTIVE, "onboarding"); err != nil {
			return err
		}
	}
	return nil
}

// CloseParties closes every party that is not already closed. Used when an application is declined.
func (a *Activities) CloseParties(ctx context.Context, partyIDs []string) error {
	for _, id := range partyIDs {
		if err := a.setStatus(ctx, id, partyv1.PartyStatus_PARTY_STATUS_CLOSED, "onboarding"); err != nil {
			return err
		}
	}
	return nil
}

// setStatus moves a party to a status, or does nothing if it is already there. That makes a retried
// activity safe, because the party service refuses a transition to the current status.
func (a *Activities) setStatus(ctx context.Context, partyID string, to partyv1.PartyStatus, actor string) error {
	got, err := a.Party.GetParty(ctx, connect.NewRequest(&partyv1.GetPartyRequest{PartyId: partyID}))
	if err != nil {
		return retryable(err)
	}
	if got.Msg.Party.Status == to {
		return nil
	}
	_, err = a.Party.SetStatus(ctx, connect.NewRequest(&partyv1.SetStatusRequest{
		PartyId: partyID, ToStatus: to, Reason: "onboarding", Actor: actor,
	}))
	return retryable(err)
}

// retryable keeps transient failures retryable and marks client errors non-retryable, so a request
// that is wrong is not sent again.
func retryable(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		switch ce.Code() {
		case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeAlreadyExists,
			connect.CodeNotFound, connect.CodePermissionDenied, connect.CodeUnauthenticated:
			return temporal.NewNonRetryableApplicationError(ce.Message(), ErrTypeValidation, err)
		}
	}
	return err
}

func providerErr(err error) error {
	if errors.Is(err, kyc.ErrNotConfigured) {
		return temporal.NewNonRetryableApplicationError("verification provider is not configured", ErrTypeNotConfigured, err)
	}
	return err
}
