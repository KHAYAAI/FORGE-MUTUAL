package forge.assets

import rego.v1

test_bank_may_hold_zar if {
	decision.allow with input as {"entity_code": "BANK", "asset": "ZAR"}
}

test_bank_may_hold_usd if {
	decision.allow with input as {"entity_code": "BANK", "asset": "USD"}
}

test_bank_may_not_hold_stablecoin if {
	not decision.allow with input as {"entity_code": "BANK", "asset": "USDC"}
}

test_digital_asset_entity_holds_nothing_in_v1 if {
	not decision.allow with input as {"entity_code": "DAST", "asset": "ZAR"}
}

test_unknown_entity_denied if {
	not decision.allow with input as {"entity_code": "NOPE", "asset": "ZAR"}
}
