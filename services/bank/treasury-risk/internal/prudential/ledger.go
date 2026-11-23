package prudential

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	treasuryv1 "github.com/khayaai/forge-mutual/pkg/api/forge/treasury/v1"
)

// Ledgers read, by asset.
var Ledgers = map[string]uint32{"ZAR": 710, "USD": 840, "EUR": 978, "GBP": 826}

// Reader builds a Position from the ledger and serves the report (treasuryv1connect.TreasuryServiceHandler).
type Reader struct {
	Client      ledgerv1connect.LedgerServiceClient
	Entity      string
	Rates       map[string]int64 // ZAR per unit of each foreign currency, scaled by 10^8
	Capital     int64            // configured qualifying capital
	GrossIncome int64            // configured average gross income
	Params      Params
	Now         func() time.Time
}

// Limitations are what the position does not yet see; the report lists them so nobody mistakes it for a return.
var Limitations = []string{
	"Capital and gross income are configured figures, not read from the ledger (no equity accounts yet).",
	"Undrawn overdraft limits and instalments due are not read from lending-service, so commitments and inflows are zero.",
	"All notice deposits are treated as not due within 30 days; notices given are not read from deposits-service.",
	"All loans are treated as business exposures at a 100% weight.",
	"Weights, run-off rates and minimums are PROPOSED, not the BA 200 and BA 300 definitions.",
}

func toZAR(minor int64, asset string, rates map[string]int64) (int64, error) {
	if asset == "ZAR" {
		return minor, nil
	}
	r, ok := rates[asset]
	if !ok || r <= 0 {
		return 0, fmt.Errorf("prudential: no rate for %s", asset)
	}
	return new(big.Int).Quo(new(big.Int).Mul(big.NewInt(minor), big.NewInt(r)), big.NewInt(100_000_000)).Int64(), nil
}

// Position reads every account of every ledger and classifies its balance.
func (r Reader) Position(ctx context.Context) (Position, int64, int64, error) {
	pos := Position{Capital: r.Capital, GrossIncome: r.GrossIncome}
	var deposits, loans int64
	byOwner := map[string]int64{}
	open := map[string]int64{} // per foreign currency: its mirror less its deposits, in rand
	for asset, ledger := range Ledgers {
		type acc struct {
			code           uint32
			product, owner string
		}
		accs := map[string]acc{}
		var ids []string
		cursor := ""
		for {
			res, err := r.Client.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{EntityCode: r.Entity, Ledger: ledger, AfterAccountId: cursor, Limit: 500}))
			if err != nil {
				return Position{}, 0, 0, fmt.Errorf("prudential: list %s: %w", asset, err)
			}
			for _, a := range res.Msg.Accounts {
				accs[a.AccountId] = acc{a.Code, a.ProductId, a.OwnerPartyId}
				ids = append(ids, a.AccountId)
			}
			if res.Msg.NextAfterAccountId == "" {
				break
			}
			cursor = res.Msg.NextAfterAccountId
		}
		for start := 0; start < len(ids); start += 500 {
			res, err := r.Client.GetBalances(ctx, connect.NewRequest(&ledgerv1.GetBalancesRequest{EntityCode: r.Entity, AccountIds: ids[start:min(start+500, len(ids))]}))
			if err != nil {
				return Position{}, 0, 0, fmt.Errorf("prudential: balances %s: %w", asset, err)
			}
			for _, b := range res.Msg.Balances {
				dp, _ := strconv.ParseInt(b.DebitsPosted, 10, 64)
				cp, _ := strconv.ParseInt(b.CreditsPosted, 10, 64)
				a := accs[b.AccountId]
				credit, err := toZAR(cp-dp, asset, r.Rates)
				if err != nil {
					return Position{}, 0, 0, err
				}
				switch a.code {
				case 3003, 5001: // the sponsor pool and partner mirrors: what the banks hold for us
					pos.Cash -= credit
					if asset != "ZAR" {
						open[asset] -= credit
					}
				case 1020:
					pos.BusinessLoans -= credit
					loans -= credit
				case 3020:
					pos.AcquirerDue += max(-credit, 0)
				case 1001:
					deposits += credit
					byOwner[a.owner] += credit
					switch {
					case asset != "ZAR":
						pos.ForeignDeposits += credit
						open[asset] -= credit
					case a.product == "savings":
						pos.Savings += credit
					case strings.HasPrefix(a.product, "notice-"):
						pos.NoticeLonger += credit
					case strings.Contains(a.product, "personal"):
						pos.CurrentRetail += credit
					default:
						pos.CurrentBusiness += credit
					}
				}
			}
		}
	}
	for owner, v := range byOwner {
		if owner != "" && v > pos.LargestDepositor {
			pos.LargestDepositor = v
		}
	}
	for _, v := range open {
		if v < 0 {
			v = -v
		}
		pos.NetOpenFX += v
	}
	return pos, deposits, loans, nil
}

// GetPrudentialReport implements treasuryv1connect.TreasuryServiceHandler.
func (r Reader) GetPrudentialReport(ctx context.Context, _ *connect.Request[treasuryv1.GetPrudentialReportRequest]) (*connect.Response[treasuryv1.GetPrudentialReportResponse], error) {
	pos, deposits, loans, err := r.Position(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	rep := Compute(pos, r.Params)
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	s := func(v int64) string { return strconv.FormatInt(v, 10) }
	out := &treasuryv1.GetPrudentialReportResponse{
		AsOf: now().UTC().Format(time.RFC3339), CapitalMinor: s(pos.Capital),
		CreditRwaMinor: s(rep.CreditRWA), MarketRwaMinor: s(rep.MarketRWA), OperationalRwaMinor: s(rep.OperationalRWA), TotalRwaMinor: s(rep.TotalRWA),
		TotalCapitalBps: rep.TotalCapitalBps, MinTotalCapitalBps: r.Params.MinTotalCapitalBps,
		LeverageExposureMinor: s(rep.LeverageExposure), LeverageBps: rep.LeverageBps, MinLeverageBps: r.Params.MinLeverageBps,
		HqlaMinor: s(rep.HQLA), NetOutflows_30DMinor: s(rep.NetOutflows30), LcrBps: rep.LCRBps, MinLcrBps: r.Params.MinLCRBps,
		Breaches: rep.Breaches, Limitations: Limitations, DepositsMinor: s(deposits), LoansMinor: s(loans), LargestDepositorMinor: s(pos.LargestDepositor),
	}
	for _, sc := range rep.Stress {
		out.Stress = append(out.Stress, &treasuryv1.StressScenario{Name: sc.Name, Description: sc.Description,
			LiquidityLeftMinor: s(sc.LiquidityLeft), TotalCapitalBps: sc.TotalCapitalBps, Survives: sc.Survives})
	}
	return connect.NewResponse(out), nil
}
