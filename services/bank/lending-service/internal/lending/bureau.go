package lending

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Bureau is a credit bureau. No bureau is chosen yet (VERIFY-BUREAU); the simulator stands in, and production refuses it.
type Bureau interface {
	// Check asks about one person. An error means no answer, and the application is refused rather than assessed blind.
	Check(ctx context.Context, party string) (BureauResult, error)
	Simulated() bool
}

// BureauResult is what the bureau says.
type BureauResult struct {
	Reference string
	Score     int
	Adverse   bool // an adverse listing, judgment or default
}

// Consents says whether a party has agreed to a use of their data.
type Consents interface {
	Allowed(ctx context.Context, party, purpose, scope string) (bool, error)
}

// BureauPurpose is the consent purpose for a credit enquiry.
const BureauPurpose = "credit_bureau_check"

// SimBureau is the development bureau. Its answers come from the party id: a party whose id ends "-adverse" has an adverse record.
type SimBureau struct{}

func (SimBureau) Simulated() bool { return true }

func (SimBureau) Check(_ context.Context, party string) (BureauResult, error) {
	h := sha256.Sum256([]byte("sim-bureau:" + party))
	r := BureauResult{Reference: "SIM-" + hex.EncodeToString(h[:4]), Score: 450 + int(h[0])%350}
	if strings.HasSuffix(party, "-adverse") {
		r.Adverse, r.Score = true, 380
	}
	return r, nil
}
