package forge.recon

import rego.v1

test_no_break_allows if {
	decision.allow with input as {"recon": {"break_active": false}, "amount_minor": "999999999"}
}

test_break_below_threshold_allows if {
	decision.allow with input as {"recon": {"break_active": true, "threshold_minor": "1000"}, "amount_minor": "1000"}
}

test_break_above_threshold_holds if {
	d := decision with input as {"recon": {"break_active": true, "threshold_minor": "1000"}, "amount_minor": "1001"}
	d.allow == false
	d.reason == "recon_break_active"
}

test_missing_break_state_denies if {
	not decision.allow with input as {"amount_minor": "1"}
}
