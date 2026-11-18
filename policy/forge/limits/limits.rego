package forge.limits

import rego.v1

# Hard limits by party tier (ARCHITECTURE.md section 10). Figures in tiers.json are placeholders
# until the risk owner confirms them (docs/QUESTIONS.md). Missing data denies.
#
# Amounts arrive as decimal strings (invariant 5: integers, and no float in transport). They are
# converted with to_number here, so the comparison is exact.

default decision := {"allow": false, "stepup": false, "reason": "default_deny"}

tier := data.tiers[input.party.tier]

amount := to_number(input.amount_minor)

# Every condition is explicit: a missing field must not pass as "false" (fail closed).
eligible if {
	input.party.status == "PARTY_STATUS_ACTIVE"
	input.screening.hit == false
	input.recon.break_active == false
}

within_limits if {
	amount <= tier.per_txn[input.asset]
	to_number(input.velocity.day_total_minor) + amount <= tier.daily[input.asset]
}

decision := {"allow": true, "stepup": true, "reason": "above_stepup"} if {
	eligible
	within_limits
	amount > tier.stepup[input.asset]
}

decision := {"allow": true, "stepup": false, "reason": "ok"} if {
	eligible
	within_limits
	amount <= tier.stepup[input.asset]
}
