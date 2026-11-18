package forge.sector

import rego.v1

# Sector eligibility (ADR-0012, open and a human decision). Only sectors listed in sectors.json
# pass; everything else, including crypto, defence and gambling, is denied.

default decision := {"allow": false, "stepup": false, "reason": "sector_not_eligible"}

decision := {"allow": true, "stepup": false, "reason": "allowed"} if {
	input.sector_code in data.sectors.allowed
}
