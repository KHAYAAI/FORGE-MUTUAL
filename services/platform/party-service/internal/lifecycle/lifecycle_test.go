package lifecycle

import (
	"testing"

	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
)

const (
	person  = partyv1.PartyType_PARTY_TYPE_PERSON
	company = partyv1.PartyType_PARTY_TYPE_COMPANY
	fund    = partyv1.PartyType_PARTY_TYPE_FUND
)

func TestTransitions(t *testing.T) {
	s := partyv1.PartyStatus_PARTY_STATUS_PROSPECT
	if !Allowed(s, partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING) {
		t.Fatal("prospect -> kyc pending must be allowed")
	}
	if Allowed(s, partyv1.PartyStatus_PARTY_STATUS_ACTIVE) {
		t.Fatal("prospect -> active skips KYC and must be refused")
	}
	if Allowed(partyv1.PartyStatus_PARTY_STATUS_CLOSED, partyv1.PartyStatus_PARTY_STATUS_ACTIVE) {
		t.Fatal("closed is terminal")
	}
	if Allowed(partyv1.PartyStatus_PARTY_STATUS_FROZEN, partyv1.PartyStatus_PARTY_STATUS_RESTRICTED) {
		t.Fatal("frozen -> restricted is not a listed transition")
	}
}

func TestFounderAndCompanySeparation(t *testing.T) {
	cases := []struct {
		name     string
		rt       partyv1.RelationshipType
		from, to partyv1.PartyType
		bps      uint32
		ok       bool
	}{
		{"person directs company", partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, person, company, 0, true},
		{"company cannot direct company", partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, company, company, 0, false},
		{"company cannot be a UBO", partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO, company, company, 5000, false},
		{"person UBO needs a share", partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO, person, company, 0, false},
		{"person UBO with share", partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO, person, company, 2600, true},
		{"company shareholder of company", partyv1.RelationshipType_RELATIONSHIP_TYPE_SHAREHOLDER, company, company, 1000, true},
		{"shareholder cannot target a person", partyv1.RelationshipType_RELATIONSHIP_TYPE_SHAREHOLDER, person, person, 1000, false},
		{"director with a share is refused", partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, person, company, 100, false},
		{"unspecified type refused", partyv1.RelationshipType_RELATIONSHIP_TYPE_UNSPECIFIED, person, company, 0, false},
		{"fund may be a director target", partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, person, fund, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := CheckRelationship(tc.rt, tc.from, tc.to, tc.bps)
			if (msg == "") != tc.ok {
				t.Fatalf("CheckRelationship = %q, want ok=%v", msg, tc.ok)
			}
		})
	}
}
