package forge.limits

import rego.v1

active := {"status": "PARTY_STATUS_ACTIVE", "tier": "standard"}

clear := {"hit": false}

quiet := {"break_active": false}

test_small_payment_is_allowed if {
	d := decision with input as {
		"party": active, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "1000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == true
	d.stepup == false
}

test_above_stepup_is_allowed_with_stepup if {
	d := decision with input as {
		"party": active, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "20000000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == true
	d.stepup == true
}

test_above_per_txn_limit_is_denied if {
	d := decision with input as {
		"party": active, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "999999999999", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_daily_limit_is_denied if {
	d := decision with input as {
		"party": active, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "1000", "velocity": {"day_total_minor": "999999999999"},
	}
	d.allow == false
}

test_exact_precision_above_float_range_is_denied if {
	# 9007199254740993 cannot be represented as a float64. The per-transaction limit still applies.
	d := decision with input as {
		"party": active, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "9007199254740993", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_screening_hit_is_denied if {
	d := decision with input as {
		"party": active, "screening": {"hit": true}, "recon": quiet,
		"asset": "ZAR", "amount_minor": "1000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_missing_screening_fails_closed if {
	d := decision with input as {
		"party": active, "recon": quiet,
		"asset": "ZAR", "amount_minor": "1000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_recon_break_is_denied if {
	d := decision with input as {
		"party": active, "screening": clear, "recon": {"break_active": true},
		"asset": "ZAR", "amount_minor": "1000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_unknown_tier_is_denied if {
	d := decision with input as {
		"party": {"status": "PARTY_STATUS_ACTIVE", "tier": "made-up"}, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "1000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_unknown_asset_is_denied if {
	d := decision with input as {
		"party": active, "screening": clear, "recon": quiet,
		"asset": "EUR", "amount_minor": "1000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_non_numeric_amount_is_denied if {
	d := decision with input as {
		"party": active, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "ten", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}

test_inactive_party_is_denied if {
	d := decision with input as {
		"party": {"status": "PARTY_STATUS_FROZEN", "tier": "standard"}, "screening": clear, "recon": quiet,
		"asset": "ZAR", "amount_minor": "1000", "velocity": {"day_total_minor": "0"},
	}
	d.allow == false
}
