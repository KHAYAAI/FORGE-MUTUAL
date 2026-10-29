package onboarding

import "testing"

func TestControllingOwnerMovesToEnhancedTier(t *testing.T) {
	if got := AssessTier(Input{Directors: []DirectorInput{{OwnershipBps: 2500}, {OwnershipBps: 5000}}}); got != "enhanced" {
		t.Fatalf("tier = %s", got)
	}
	if got := AssessTier(Input{Directors: []DirectorInput{{OwnershipBps: 4999}}}); got != "standard" {
		t.Fatalf("tier = %s", got)
	}
	if got := AssessTier(Input{}); got != "standard" {
		t.Fatalf("no directors: tier = %s", got)
	}
}
