package forge.sector

import rego.v1

test_listed_sector_allowed if {
	decision.allow with input as {"sector_code": "software_saas"}
}

test_unlisted_sector_denied if {
	not decision.allow with input as {"sector_code": "defence"}
}

test_missing_sector_denied if {
	not decision.allow with input as {}
}
