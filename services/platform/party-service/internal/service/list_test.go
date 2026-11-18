package service

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
)

// Listing pages through every party once, in order, filtered by status, with identity numbers masked.
func TestListPartiesPagesThroughEveryPartyOnce(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		mustCreate(t, s, company(fmt.Sprint("c", i), fmt.Sprint("Company ", i), fmt.Sprintf("2026/00000%d/07", i)))
	}
	p := mustCreate(t, s, person("p", "Thandi Dube", "8001015009087"))
	seen := map[string]bool{}
	after := ""
	for pages := 0; pages < 10; pages++ {
		res, err := s.ListParties(ctx, connect.NewRequest(&partyv1.ListPartiesRequest{AfterPartyId: after, Limit: 2}))
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range res.Msg.Parties {
			if seen[x.PartyId] {
				t.Fatalf("%s listed twice", x.PartyId)
			}
			seen[x.PartyId] = true
			if x.PartyId == p.PartyId && x.NationalIdMasked == "8001015009087" {
				t.Fatal("an identity number was listed unmasked")
			}
		}
		if res.Msg.NextAfterPartyId == "" {
			break
		}
		after = res.Msg.NextAfterPartyId
	}
	if len(seen) != 6 {
		t.Fatalf("listed %d parties, want 6", len(seen))
	}
	res, _ := s.ListParties(ctx, connect.NewRequest(&partyv1.ListPartiesRequest{Statuses: []partyv1.PartyStatus{partyv1.PartyStatus_PARTY_STATUS_ACTIVE}, Limit: 100}))
	if len(res.Msg.Parties) != 0 {
		t.Fatalf("prospects listed as active: %d", len(res.Msg.Parties))
	}
	if _, err := s.ListParties(ctx, connect.NewRequest(&partyv1.ListPartiesRequest{Limit: 0})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("limit 0: %v", err)
	}
}
