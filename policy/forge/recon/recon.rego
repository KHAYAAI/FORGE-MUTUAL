package forge.recon

import rego.v1

# Owner decision: the strict rule. While a reconciliation break is open, every outbound payment is held, whatever its
# amount. The gate is open only when the reconciliation service reports no break (invariant 9). There is no threshold:
# a difference that does not reconcile must not be paid through, however small.
#
# Missing or non-boolean state denies. Amounts are not consulted.

default decision := {"allow": false, "stepup": false, "reason": "default_deny"}

decision := {"allow": true, "stepup": false, "reason": "no_break"} if {
	input.recon.break_active == false
}

decision := {"allow": false, "stepup": false, "reason": "recon_break_active"} if {
	input.recon.break_active == true
}
