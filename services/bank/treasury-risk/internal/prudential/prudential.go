// Package prudential computes Forge's capital adequacy, liquidity coverage and stress results from a position read off
// the ledger. It is pure and works in integers (minor units, basis points).
//
// The method is a simplified standardised approach in the shape the Banks Act regulations and Directive 6 of 2017
// prescribe: risk-weighted assets for credit, market (foreign-exchange) and operational risk; a total capital ratio and
// a leverage ratio against minimums; and a 30-day liquidity coverage ratio with run-off rates by kind of deposit. Every
// weight, rate and minimum here is PROPOSED (docs/THRESHOLDS.md, thresholds 22 to 25) until treasury sets it and the
// returns (BA 200, BA 300) define it exactly. It answers "roughly where do we stand, and what breaks first", not the
// regulatory figure.
package prudential

import "math/big"

// Position is what the ledger holds for one reporting date, in rand minor units. Foreign balances are converted to rand
// before they get here.
type Position struct {
	Capital int64 // qualifying capital: own funds raised (configured until the ledger holds equity)

	// Assets.
	Cash           int64 // claims on the sponsor bank and partner banks (pools and mirrors)
	BusinessLoans  int64 // amounts owed on loans and overdrafts to businesses
	PersonalLoans  int64 // amounts owed by individuals
	UndrawnOverdft int64 // overdraft limits not drawn (a commitment)
	AcquirerDue    int64 // card takings due from the acquirer

	// Deposits (liabilities), by kind.
	CurrentRetail   int64 // personal current accounts
	CurrentBusiness int64 // business current accounts
	Savings         int64 // instant-access savings
	NoticeDue30     int64 // notice deposits whose notice falls due within 30 days
	NoticeLonger    int64 // notice deposits not due within 30 days
	ForeignDeposits int64 // dollar, euro and pound deposits, in rand

	LargestDepositor int64 // the largest single customer's deposits

	// Inflows due within 30 days.
	InstalmentsDue30 int64

	// Market risk: the net open foreign-currency position, in rand (absolute value).
	NetOpenFX int64

	// Operational risk: average annual gross income over three years (configured until there is history).
	GrossIncome int64
}

// Params are the weights, rates and minimums. Defaults are PROPOSED.
type Params struct {
	RWCash, RWBusinessLoan, RWPersonalLoan, RWAcquirer, CCFUndrawn int64 // risk weights and credit conversion, in bps
	OperationalBps                                                 int64 // share of gross income charged (basic indicator: 15%)
	MinTotalCapitalBps, MinLeverageBps, MinLCRBps                  int64
	RunoffRetail, RunoffBusiness, RunoffSavings, RunoffForeign     int64 // 30-day run-off rates, bps
	InflowBps                                                      int64 // share of contractual inflows counted
}

// DefaultParams are PROPOSED values in the shape of the Basel standardised approach.
func DefaultParams() Params {
	return Params{
		RWCash: 2000, RWBusinessLoan: 10000, RWPersonalLoan: 7500, RWAcquirer: 2000, CCFUndrawn: 1000,
		OperationalBps:     1500,
		MinTotalCapitalBps: 1150, MinLeverageBps: 400, MinLCRBps: 10000,
		RunoffRetail: 1000, RunoffBusiness: 4000, RunoffSavings: 1000, RunoffForeign: 4000,
		InflowBps: 5000,
	}
}

// Report is the result.
type Report struct {
	CreditRWA, MarketRWA, OperationalRWA, TotalRWA int64
	TotalCapitalBps                                int64 // capital / RWA
	LeverageExposure                               int64
	LeverageBps                                    int64 // capital / exposure
	HQLA, Outflows30, Inflows30, NetOutflows30     int64
	LCRBps                                         int64 // HQLA / net outflows
	Breaches                                       []string
	Stress                                         []Scenario
}

// Scenario is one stress test's outcome.
type Scenario struct {
	Name            string
	Description     string
	LiquidityLeft   int64 // HQLA after the scenario's outflows; negative means it runs out
	TotalCapitalBps int64 // capital ratio after the scenario's losses
	Survives        bool
}

func bps(v, b int64) int64 {
	return new(big.Int).Quo(new(big.Int).Mul(big.NewInt(v), big.NewInt(b)), big.NewInt(10000)).Int64()
}

// ratio is part/whole in basis points, rounded down; a zero whole gives the largest value (no exposure, no risk).
func ratio(part, whole int64) int64 {
	if whole <= 0 {
		return 1 << 40
	}
	return new(big.Int).Quo(new(big.Int).Mul(big.NewInt(part), big.NewInt(10000)), big.NewInt(whole)).Int64()
}

func (p Params) creditRWA(pos Position) int64 {
	return bps(pos.Cash, p.RWCash) + bps(pos.BusinessLoans, p.RWBusinessLoan) + bps(pos.PersonalLoans, p.RWPersonalLoan) +
		bps(pos.AcquirerDue, p.RWAcquirer) + bps(bps(pos.UndrawnOverdft, p.CCFUndrawn), p.RWBusinessLoan)
}

func (p Params) outflows(pos Position) int64 {
	return bps(pos.CurrentRetail, p.RunoffRetail) + bps(pos.CurrentBusiness, p.RunoffBusiness) + bps(pos.Savings, p.RunoffSavings) +
		pos.NoticeDue30 + bps(pos.ForeignDeposits, p.RunoffForeign) + bps(pos.UndrawnOverdft, p.CCFUndrawn)
}

// Compute builds the report, with the standard stress scenarios.
func Compute(pos Position, p Params) Report {
	r := Report{CreditRWA: p.creditRWA(pos), MarketRWA: pos.NetOpenFX}
	// Operational risk RWA under the basic indicator approach: 12.5 x (15% of gross income).
	r.OperationalRWA = bps(pos.GrossIncome, p.OperationalBps) * 125 / 10
	r.TotalRWA = r.CreditRWA + r.MarketRWA + r.OperationalRWA
	r.TotalCapitalBps = ratio(pos.Capital, r.TotalRWA)
	r.LeverageExposure = pos.Cash + pos.BusinessLoans + pos.PersonalLoans + pos.AcquirerDue + bps(pos.UndrawnOverdft, p.CCFUndrawn)
	r.LeverageBps = ratio(pos.Capital, r.LeverageExposure)

	r.HQLA = pos.Cash
	r.Outflows30 = p.outflows(pos)
	r.Inflows30 = min(bps(pos.InstalmentsDue30, p.InflowBps), r.Outflows30*3/4) // inflows capped at 75% of outflows
	r.NetOutflows30 = r.Outflows30 - r.Inflows30
	r.LCRBps = ratio(r.HQLA, r.NetOutflows30)

	if r.TotalCapitalBps < p.MinTotalCapitalBps {
		r.Breaches = append(r.Breaches, "TOTAL_CAPITAL_RATIO")
	}
	if r.LeverageBps < p.MinLeverageBps {
		r.Breaches = append(r.Breaches, "LEVERAGE_RATIO")
	}
	if r.LCRBps < p.MinLCRBps {
		r.Breaches = append(r.Breaches, "LIQUIDITY_COVERAGE")
	}
	r.Stress = Stress(pos, p)
	return r
}

// Stress runs the standard scenarios. A scenario survives when liquidity stays positive and the capital ratio stays
// at or above the minimum.
func Stress(pos Position, p Params) []Scenario {
	run := func(name, desc string, outflow, loss int64) Scenario {
		s := Scenario{Name: name, Description: desc, LiquidityLeft: pos.Cash - outflow}
		after := pos
		after.Capital -= loss
		s.TotalCapitalBps = ratio(max(after.Capital, 0), p.creditRWA(after)+pos.NetOpenFX+bps(pos.GrossIncome, p.OperationalBps)*125/10)
		s.Survives = s.LiquidityLeft >= 0 && s.TotalCapitalBps >= p.MinTotalCapitalBps
		return s
	}
	base := p.outflows(pos)
	loans := pos.BusinessLoans + pos.PersonalLoans
	return []Scenario{
		run("RUN_MODERATE", "30 days of outflows at the standard run-off rates", base, 0),
		run("RUN_SEVERE", "Twice the standard run-off rates: a loss of confidence", 2*base, 0),
		run("LARGEST_DEPOSITOR", "The largest single depositor leaves at once, on top of standard outflows", base+pos.LargestDepositor, 0),
		run("CREDIT_LOSS_10", "10% of loans are lost", 0, loans/10),
		run("CREDIT_LOSS_25", "25% of loans are lost, and deposits run at the standard rates", base, loans/4),
	}
}
