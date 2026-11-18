package onboarding

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
)

// statusParties records which parties were moved, and refuses an ID that is not set, as the party service does.
type statusParties struct {
	partyv1connect.PartyServiceClient
	t     *testing.T
	moved []string
}

func (s *statusParties) GetParty(_ context.Context, r *connect.Request[partyv1.GetPartyRequest]) (*connect.Response[partyv1.GetPartyResponse], error) {
	if r.Msg.PartyId == "" {
		s.t.Errorf("GetParty asked for an empty party ID")
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	return connect.NewResponse(&partyv1.GetPartyResponse{Party: &partyv1.Party{PartyId: r.Msg.PartyId}}), nil
}

func (s *statusParties) SetStatus(_ context.Context, r *connect.Request[partyv1.SetStatusRequest]) (*connect.Response[partyv1.SetStatusResponse], error) {
	s.moved = append(s.moved, r.Msg.PartyId)
	return connect.NewResponse(&partyv1.SetStatusResponse{}), nil
}

// A person's onboarding has no company. On the stack the verification step asked the party service about an empty
// company ID, was refused, and every personal application stopped there.
func TestAPersonsVerificationStartsOnlyThePerson(t *testing.T) {
	parties, provider := &statusParties{t: t}, &countingProvider{}
	a := &Activities{Party: parties, KYC: provider, Sessions: newMemSessions()}
	if err := a.StartVerifications(context.Background(), Input{OnboardingID: "onb-p", Actor: "a"}, ProspectsResult{PersonIDs: []string{"person-1"}}); err != nil {
		t.Fatalf("StartVerifications: %v", err)
	}
	if len(parties.moved) != 1 || parties.moved[0] != "person-1" {
		t.Fatalf("parties moved to KYC_PENDING = %v, want only person-1", parties.moved)
	}
	if len(provider.started) != 1 {
		t.Fatalf("verifications started = %d, want 1", len(provider.started))
	}
}
