package forge.recon

import rego.v1

# While a sponsor reconciliation break is open, payments above the threshold are held (invariant 9).
# Amounts are decimal strings, as in forge.limits.

default decision := {"allow": false, "stepup": false, "reason": "default_deny"}

decision := {"allow": true, "stepup": false, "reason": "no_break"} if {
	input.recon.break_active == false
}

decision := {"allow": true, "stepup": false, "reason": "below_threshold"} if {
	input.recon.break_active == true
	to_number(input.amount_minor) <= to_number(input.recon.threshold_minor)
}

decision := {"allow": false, "stepup": false, "reason": "recon_break_active"} if {
	input.recon.break_active == true
	to_number(input.amount_minor) > to_number(input.recon.threshold_minor)
}
