package forge.assets

import rego.v1

# Which assets each entity may hold (assets_by_entity.json). Default deny.

default decision := {"allow": false, "stepup": false, "reason": "default_deny"}

decision := {"allow": true, "stepup": false, "reason": "ok"} if {
	input.asset in data.assets_by_entity[input.entity_code]
}
