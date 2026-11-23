package banklayer

import (
	"fmt"
	"strings"
)

// Profile is what the law asks of a legal form. Forge plans to be a mutual bank and keeps the bank route open
// (ADR-0020); every difference between the two lives here, as data, so the rest of the code never branches on it.
//
// Figures are from the regulatory register (LIC-01, LIC-02, LIC-07, MR-03, BR-07, MR-06). They are not legal advice:
// a regulatory lawyer must confirm each against the current Act, regulations and Prudential Authority practice.
type Profile struct {
	Form LegalForm `json:"legal_form"`
	Name string    `json:"name"`
	Act  string    `json:"act"`

	// MinInitialCapitalMinor is the minimum qualifying capital in ZAR cents before the PA will register the entity.
	MinInitialCapitalMinor int64 `json:"min_initial_capital_minor"`

	// Owners: who owns the entity and how they vote.
	Owners          string `json:"owners"`
	MinFounders     int    `json:"min_founders"`      // signatories to the founding documents
	VotingRule      string `json:"voting_rule"`       // ONE_MEMBER_ONE_VOTE or BY_SHARES
	MemberShares    bool   `json:"member_shares"`     // depositor-members hold shares in the bank
	OwnerApprovalAt int    `json:"owner_approval_at"` // percent of ownership above which PA permission is needed; 0 if not applicable

	// Board: the minimum number of directors who must be employees.
	MinEmployeeDirectors int `json:"min_employee_directors"`

	// Forms filed with the PA.
	FitAndProperForm string   `json:"fit_and_proper_form"`
	ApplicationForms []string `json:"application_forms"`
	Returns          []string `json:"returns"`

	// Can it raise outside equity (venture capital) and be sold?
	OutsideEquity bool `json:"outside_equity"`

	// VERIFY lists what has not been confirmed against primary text.
	Verify []string `json:"verify"`
}

// R in cents.
const rand = int64(100)

var profiles = map[LegalForm]Profile{
	LegalFormMutual: {
		Form: LegalFormMutual, Name: "Mutual bank", Act: "Mutual Banks Act 124 of 1993",
		MinInitialCapitalMinor: 10_000_000 * rand,
		Owners:                 "Depositor-members, who hold member shares",
		MinFounders:            7, VotingRule: "ONE_MEMBER_ONE_VOTE", MemberShares: true, OwnerApprovalAt: 0,
		MinEmployeeDirectors: 2,
		FitAndProperForm:     "DI 020",
		ApplicationForms:     []string{"DI 100", "DI 110", "DI 200", "DI 300", "DI 400", "DI 430", "DI 700"},
		Returns: []string{"DI 100", "DI 110", "DI 200", "DI 300", "DI 310", "DI 400", "DI 401", "DI 402", "DI 403",
			"DI 410", "DI 420", "DI 430", "DI 500", "DI 510", "DI 520", "DI 600", "DI 700", "DI 900"},
		OutsideEquity: false,
		Verify: []string{
			"R10 million minimum: the 1993 regulations read here may have been amended; confirm the current figure",
			"Voting rule: the constitution sets it within the Act; one member one vote is PROPOSED",
			"Employee directors: MR-03 read from the 1993 gazette",
		},
	},
	LegalFormBank: {
		Form: LegalFormBank, Name: "Bank", Act: "Banks Act 94 of 1990",
		MinInitialCapitalMinor: 250_000_000 * rand,
		Owners:                 "Shareholders of a public company (Companies Act 71 of 2008)",
		MinFounders:            1, VotingRule: "BY_SHARES", MemberShares: false, OwnerApprovalAt: 15,
		MinEmployeeDirectors: 0,
		FitAndProperForm:     "BA 020",
		ApplicationForms:     []string{"BA 100", "BA 110", "BA 120", "BA 130", "BA 300", "BA 350", "BA 700"},
		Returns: []string{"BA 100", "BA 110", "BA 120", "BA 125", "BA 130", "BA 200", "BA 210", "BA 300", "BA 325",
			"BA 330", "BA 340", "BA 350", "BA 400", "BA 500", "BA 600", "BA 700", "BA 900", "BA 930", "BA 950"},
		OutsideEquity: true,
		Verify: []string{
			"R250 million minimum: confirm the current figure with the PA",
			"Ownership permissions at 15% and above (Banks Act s.37): confirm the thresholds",
			"The return list is from the register; scope the full set and each directive",
		},
	},
}

// Planned is the legal form Forge intends to apply for. The bank route stays available.
const Planned = LegalFormMutual

// ProfileFor returns the profile of a legal form.
func ProfileFor(f LegalForm) (Profile, error) {
	p, ok := profiles[f]
	if !ok {
		return Profile{}, fmt.Errorf("banklayer: no profile for legal form %q (want %s or %s)", f, LegalFormMutual, LegalFormBank)
	}
	p.ApplicationForms = append([]string(nil), p.ApplicationForms...)
	p.Returns = append([]string(nil), p.Returns...)
	p.Verify = append([]string(nil), p.Verify...)
	return p, nil
}

// ParseLegalForm reads "mutual" or "bank" (or the full constant). Empty means the planned form.
func ParseLegalForm(s string) (LegalForm, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "":
		return Planned, nil
	case "MUTUAL", string(LegalFormMutual):
		return LegalFormMutual, nil
	case "BANK", string(LegalFormBank):
		return LegalFormBank, nil
	}
	return "", fmt.Errorf("banklayer: unknown legal form %q (want mutual or bank)", s)
}

// Profiles returns both forms, planned first, for side-by-side comparison.
func Profiles() []Profile {
	m, _ := ProfileFor(LegalFormMutual)
	b, _ := ProfileFor(LegalFormBank)
	return []Profile{m, b}
}

// OwnLicenceCapabilities is the route once Forge holds its own licence in the given form.
func OwnLicenceCapabilities(f LegalForm) (Capabilities, error) {
	p, err := ProfileFor(f)
	if err != nil {
		return Capabilities{}, err
	}
	return Capabilities{Route: RouteOwnLicence, LegalForm: f, MemberShares: p.MemberShares, LegalBookOfRecord: BookForge}, nil
}
