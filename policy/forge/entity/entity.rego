package forge.entity

import rego.v1

# Cross-entity rules (invariant 7) and person-to-entity transfers (invariant 12, transfer code 63).

default decision := {"allow": false, "stepup": false, "reason": "default_deny"}

same_entity if {
	input.debit_entity == input.entity_code
	input.credit_entity == input.entity_code
}

decision := {"allow": true, "stepup": false, "reason": "ok"} if {
	same_entity
	input.code != 63
}

# Code 63 needs both an explicit policy approval and a verified relationship between the person
# and the company (a director or UBO relationship in party-service).
decision := {"allow": true, "stepup": false, "reason": "person_entity_approved"} if {
	same_entity
	input.code == 63
	input.policy_approved == true
	input.person_relationship_verified == true
}

decision := {"allow": false, "stepup": false, "reason": "person_entity_not_approved"} if {
	same_entity
	input.code == 63
	not input.policy_approved == true
}
