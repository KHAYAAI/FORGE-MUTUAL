package forge.recon

import rego.v1

test_no_break_allows if {
	decision.allow with input as {"recon": {"break_active": false}}
}

# The strict rule: a break holds every payment, including the smallest.
test_break_holds_every_amount if {
	d := decision with input as {"recon": {"break_active": true}, "amount_minor": "1"}
	d.allow == false
	d.reason == "recon_break_active"
}

test_break_holds_large_amounts_too if {
	d := decision with input as {"recon": {"break_active": true}, "amount_minor": "999999999"}
	d.allow == false
}

test_missing_break_state_denies if {
	not decision.allow with input as {"amount_minor": "1"}
}

test_non_boolean_break_state_denies if {
	not decision.allow with input as {"recon": {"break_active": "false"}}
}
