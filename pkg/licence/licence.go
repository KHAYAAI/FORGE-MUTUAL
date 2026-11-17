// Package licence keeps a regulated product switched off in production until a person with the authority to do so
// has signed it off. Savings and notice deposits need treasury and regulatory approval; lending needs a credit
// policy, National Credit Act registration and capital; foreign-currency accounts and inbound SWIFT need an
// authorised-dealer arrangement and SARB reporting; merchant acquiring needs an acquiring bank. The software for
// each is built and tested; this package decides whether it may take a customer's money.
//
// In development every product is available, so it can be built and tested. In production (FORGE_ENV=production)
// a product is available only when FORGE_APPROVAL_<PRODUCT> names the decision that approved it, for example
// FORGE_APPROVAL_SAVINGS=DEC-0031, a decision recorded in docs/DECISIONS.md. Anything else refuses the product.
package licence

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Product is a regulated product.
type Product string

// The products this package gates, and who must sign each off.
const (
	Savings        Product = "savings"         // treasury and regulator: interest-bearing deposits
	NoticeDeposits Product = "notice_deposits" // treasury and regulator: fixed notice periods
	Overdraft      Product = "overdraft"       // credit committee, NCA registration, capital
	TermLoans      Product = "term_loans"      // credit committee, NCA registration, capital
	RevenueBased   Product = "revenue_based"   // credit committee and legal: revenue-based financing
	EURAccounts    Product = "eur_accounts"    // authorised dealer and partner bank
	GBPAccounts    Product = "gbp_accounts"    // authorised dealer and partner bank
	InboundSWIFT   Product = "inbound_swift"   // authorised dealer; SARB cross-border reporting
	Acquiring      Product = "acquiring"       // acquiring bank, card schemes, PCI
)

// All lists every gated product.
var All = []Product{Savings, NoticeDeposits, Overdraft, TermLoans, RevenueBased, EURAccounts, GBPAccounts, InboundSWIFT, Acquiring}

// ErrNotApproved is returned for a product that production may not offer.
var ErrNotApproved = errors.New("licence: this product is not approved for use")

var decisionRef = regexp.MustCompile(`^DEC-[0-9]{4}$`)

// Gate answers whether each product is available. Build it once at start-up with FromEnv.
type Gate struct {
	production bool
	approved   map[Product]string
}

// EnvName is the variable that carries a product's approval.
func EnvName(p Product) string { return "FORGE_APPROVAL_" + strings.ToUpper(string(p)) }

// FromEnv reads the environment. A malformed approval is an error, so a typo stops the service at start rather than
// quietly leaving a product off (or, worse, on).
func FromEnv(getenv func(string) string) (Gate, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	g := Gate{production: getenv("FORGE_ENV") == "production", approved: map[Product]string{}}
	for _, p := range All {
		v := strings.TrimSpace(getenv(EnvName(p)))
		if v == "" {
			continue
		}
		if !decisionRef.MatchString(v) {
			return Gate{}, fmt.Errorf("licence: %s must name a decision such as DEC-0031, not %q", EnvName(p), v)
		}
		g.approved[p] = v
	}
	return g, nil
}

// Allow returns nil when the product may be used, and ErrNotApproved otherwise.
func (g Gate) Allow(p Product) error {
	if !g.production {
		return nil
	}
	if _, ok := g.approved[p]; ok {
		return nil
	}
	return fmt.Errorf("%w: %s (set %s to the approving decision)", ErrNotApproved, p, EnvName(p))
}

// Approval returns the decision that approved a product, and whether there is one. Development has none.
func (g Gate) Approval(p Product) (string, bool) {
	ref, ok := g.approved[p]
	return ref, ok
}

// Status describes each product, for a service's start-up log.
func (g Gate) Status() map[Product]string {
	out := map[Product]string{}
	for _, p := range All {
		switch ref, ok := g.approved[p]; {
		case ok:
			out[p] = "approved by " + ref
		case g.production:
			out[p] = "off: not approved"
		default:
			out[p] = "development"
		}
	}
	return out
}
