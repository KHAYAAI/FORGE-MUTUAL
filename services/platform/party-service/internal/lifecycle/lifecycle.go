// Package lifecycle holds the party status transitions (ARCHITECTURE.md 11.3):
// PROSPECT -> KYC_PENDING -> ACTIVE -> RESTRICTED | FROZEN -> CLOSED. Anything not listed is refused.
package lifecycle

import (
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
)

var allowed = map[partyv1.PartyStatus][]partyv1.PartyStatus{
	partyv1.PartyStatus_PARTY_STATUS_PROSPECT:    {partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING, partyv1.PartyStatus_PARTY_STATUS_CLOSED},
	partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING: {partyv1.PartyStatus_PARTY_STATUS_ACTIVE, partyv1.PartyStatus_PARTY_STATUS_CLOSED},
	partyv1.PartyStatus_PARTY_STATUS_ACTIVE:      {partyv1.PartyStatus_PARTY_STATUS_RESTRICTED, partyv1.PartyStatus_PARTY_STATUS_FROZEN, partyv1.PartyStatus_PARTY_STATUS_CLOSED},
	partyv1.PartyStatus_PARTY_STATUS_RESTRICTED:  {partyv1.PartyStatus_PARTY_STATUS_ACTIVE, partyv1.PartyStatus_PARTY_STATUS_FROZEN, partyv1.PartyStatus_PARTY_STATUS_CLOSED},
	partyv1.PartyStatus_PARTY_STATUS_FROZEN:      {partyv1.PartyStatus_PARTY_STATUS_ACTIVE, partyv1.PartyStatus_PARTY_STATUS_CLOSED},
	partyv1.PartyStatus_PARTY_STATUS_CLOSED:      nil,
}

// Allowed reports whether a party may move from one status to another.
func Allowed(from, to partyv1.PartyStatus) bool {
	for _, next := range allowed[from] {
		if next == to {
			return true
		}
	}
	return false
}

// relationshipRules encodes founder and company separation (invariant 12): which party kinds may
// stand in which relationship. A company can never be a director, UBO, signatory or employee.
type rule struct {
	from, to   []partyv1.PartyType
	needsShare bool // shareholder and UBO carry a percentage
}

var relationshipRules = map[partyv1.RelationshipType]rule{
	partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR:    {from: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_PERSON}, to: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_COMPANY, partyv1.PartyType_PARTY_TYPE_FUND}},
	partyv1.RelationshipType_RELATIONSHIP_TYPE_SIGNATORY:   {from: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_PERSON}, to: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_COMPANY, partyv1.PartyType_PARTY_TYPE_FUND}},
	partyv1.RelationshipType_RELATIONSHIP_TYPE_EMPLOYEE:    {from: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_PERSON}, to: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_COMPANY}},
	partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO:         {from: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_PERSON}, to: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_COMPANY, partyv1.PartyType_PARTY_TYPE_FUND}, needsShare: true},
	partyv1.RelationshipType_RELATIONSHIP_TYPE_SHAREHOLDER: {from: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_PERSON, partyv1.PartyType_PARTY_TYPE_COMPANY, partyv1.PartyType_PARTY_TYPE_FUND}, to: []partyv1.PartyType{partyv1.PartyType_PARTY_TYPE_COMPANY}, needsShare: true},
}

// CheckRelationship returns an error message, or "" when the pairing is allowed.
func CheckRelationship(rt partyv1.RelationshipType, from, to partyv1.PartyType, bps uint32) string {
	r, ok := relationshipRules[rt]
	if !ok {
		return "relationship type is required"
	}
	if !contains(r.from, from) {
		return "source party type may not hold this relationship"
	}
	if !contains(r.to, to) {
		return "target party type may not be the subject of this relationship"
	}
	if r.needsShare && bps == 0 {
		return "percentage_bps is required for this relationship"
	}
	if !r.needsShare && bps != 0 {
		return "percentage_bps is only valid for shareholder and UBO relationships"
	}
	return ""
}

func contains[T comparable](xs []T, x T) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
