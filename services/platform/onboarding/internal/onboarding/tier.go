package onboarding

// ControllingBps is the ownership at which a director is treated as controlling the company, which moves
// the company to the enhanced tier. The threshold is a placeholder: the risk owner must confirm it.
const ControllingBps = 5000

// AssessTier returns the tier label for an application. policy/tiers.json holds the limits for each label.
func AssessTier(in Input) string {
	for _, d := range in.Directors {
		if d.OwnershipBps >= ControllingBps {
			return "enhanced"
		}
	}
	return "standard"
}
