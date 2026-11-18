package forge.banklayer_test

import rego.v1

import data.forge.banklayer

sponsor := {
	"route": "ROUTE_SPONSOR",
	"legal_form": "LEGAL_FORM_NONE",
	"direct_clearing": false,
	"tokenised_deposits": false,
	"member_shares": false,
	"legal_book_of_record": "BOOK_SPONSOR",
}

own_licence := {
	"route": "ROUTE_OWN_LICENCE",
	"legal_form": "LEGAL_FORM_BANK",
	"direct_clearing": true,
	"tokenised_deposits": false,
	"member_shares": false,
	"legal_book_of_record": "BOOK_FORGE",
}

test_sponsor_cannot_clear_directly if {
	not banklayer.decision.allow with input as {"feature": "direct_clearing", "capabilities": sponsor, "adr_0020_accepted": true}
}

test_own_licence_needs_adr_0020 if {
	not banklayer.decision.allow with input as {"feature": "direct_clearing", "capabilities": own_licence, "adr_0020_accepted": false}
}

test_own_licence_with_adr_0020_may_clear if {
	banklayer.decision.allow with input as {"feature": "direct_clearing", "capabilities": own_licence, "adr_0020_accepted": true}
}

test_tokenised_deposits_stay_off_on_sponsor if {
	not banklayer.decision.allow with input as {"feature": "tokenised_deposits", "capabilities": sponsor, "adr_0020_accepted": true}
}

test_member_shares_need_a_mutual if {
	not banklayer.decision.allow with input as {"feature": "member_shares", "capabilities": own_licence, "adr_0020_accepted": true}
}

test_sponsor_capabilities_are_consistent if {
	banklayer.decision.allow with input as {"feature": "capabilities", "capabilities": sponsor}
}

test_sponsor_claiming_clearing_is_inconsistent if {
	bad := object.union(sponsor, {"direct_clearing": true})
	not banklayer.decision.allow with input as {"feature": "capabilities", "capabilities": bad}
}

test_sponsor_claiming_forge_book_is_inconsistent if {
	bad := object.union(sponsor, {"legal_book_of_record": "BOOK_FORGE"})
	not banklayer.decision.allow with input as {"feature": "capabilities", "capabilities": bad}
}

test_unknown_feature_is_denied if {
	not banklayer.decision.allow with input as {"feature": "anything", "capabilities": own_licence, "adr_0020_accepted": true}
}
