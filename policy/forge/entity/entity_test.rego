package forge.entity

import rego.v1

test_same_entity_transfer_allowed if {
	decision.allow with input as {"entity_code": "BANK", "debit_entity": "BANK", "credit_entity": "BANK", "code": 10}
}

test_cross_entity_transfer_denied if {
	not decision.allow with input as {"entity_code": "BANK", "debit_entity": "BANK", "credit_entity": "DAST", "code": 10}
}

test_person_entity_without_approval_denied if {
	d := decision with input as {"entity_code": "BANK", "debit_entity": "BANK", "credit_entity": "BANK", "code": 63}
	d.allow == false
	d.reason == "person_entity_not_approved"
}

test_person_entity_with_approval_and_relationship_allowed if {
	decision.allow with input as {
		"entity_code": "BANK", "debit_entity": "BANK", "credit_entity": "BANK", "code": 63,
		"policy_approved": true, "person_relationship_verified": true,
	}
}

test_person_entity_approval_without_relationship_denied if {
	not decision.allow with input as {
		"entity_code": "BANK", "debit_entity": "BANK", "credit_entity": "BANK", "code": 63,
		"policy_approved": true, "person_relationship_verified": false,
	}
}
