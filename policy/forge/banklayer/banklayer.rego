package forge.banklayer

import rego.v1

# The bank layer's route is data (ARCHITECTURE.md 5.2). These rules decide which route-specific features an action
# may use. They read Capabilities from input.capabilities and never read a route to branch code. Each rule is gated
# on input.feature, so at most one rule can match and the result is never ambiguous.

default decision := {"allow": false, "reason": "default_deny"}

# Own-licence features need the own-licence route, the feature switched on, and ADR-0020 accepted.
decision := {"allow": true, "reason": "own_licence_feature_permitted"} if {
	input.feature == "direct_clearing"
	input.capabilities.route == "ROUTE_OWN_LICENCE"
	input.capabilities.direct_clearing == true
	input.adr_0020_accepted == true
}

decision := {"allow": true, "reason": "own_licence_feature_permitted"} if {
	input.feature == "tokenised_deposits"
	input.capabilities.route == "ROUTE_OWN_LICENCE"
	input.capabilities.tokenised_deposits == true
	input.adr_0020_accepted == true
}

# Member shares exist only in a mutual, and only after ADR-0020.
decision := {"allow": true, "reason": "member_shares_permitted"} if {
	input.feature == "member_shares"
	input.capabilities.legal_form == "LEGAL_FORM_MUTUAL"
	input.capabilities.member_shares == true
	input.adr_0020_accepted == true
}

# Capabilities must be internally consistent. A sponsor route that claims own-licence features fails here.
decision := {"allow": true, "reason": "capabilities_consistent"} if {
	input.feature == "capabilities"
	capabilities_consistent
}

decision := {"allow": false, "reason": "capabilities_inconsistent"} if {
	input.feature == "capabilities"
	not capabilities_consistent
}

capabilities_consistent if {
	input.capabilities.route == "ROUTE_SPONSOR"
	input.capabilities.legal_form == "LEGAL_FORM_NONE"
	input.capabilities.legal_book_of_record == "BOOK_SPONSOR"
	input.capabilities.direct_clearing == false
	input.capabilities.tokenised_deposits == false
	input.capabilities.member_shares == false
}
