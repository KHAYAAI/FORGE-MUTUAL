package prudential

import (
	"context"
	"sort"
	"strconv"
	"time"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	treasuryv1 "github.com/khayaai/forge-mutual/pkg/api/forge/treasury/v1"
)

// Deposit is one customer deposit account, for the single customer view.
type Deposit struct {
	Party   string
	Asset   string
	Insured bool
	Balance int64 // in the account's own currency, minor units
}

// Depositor is one depositor's position.
type Depositor struct {
	Party                        string
	Accounts                     int
	Eligible, Covered, Uncovered int64 // rand minor units
}

// SingleCustomerView aggregates each depositor's deposits: rand deposits flagged insured are eligible, covered up to
// the limit per depositor; everything else (foreign currency, unflagged, or above the limit) is uncovered.
func SingleCustomerView(ds []Deposit, limit int64, rates map[string]int64) ([]Depositor, error) {
	by := map[string]*Depositor{}
	var other = map[string]int64{}
	for _, d := range ds {
		if d.Party == "" || d.Balance <= 0 {
			continue
		}
		p := by[d.Party]
		if p == nil {
			p = &Depositor{Party: d.Party}
			by[d.Party] = p
		}
		p.Accounts++
		if d.Asset == "ZAR" && d.Insured {
			p.Eligible += d.Balance
			continue
		}
		v, err := toZAR(d.Balance, d.Asset, rates)
		if err != nil {
			return nil, err
		}
		other[d.Party] += v
	}
	out := make([]Depositor, 0, len(by))
	for _, p := range by {
		p.Covered = min(p.Eligible, limit)
		p.Uncovered = p.Eligible - p.Covered + other[p.Party]
		out = append(out, *p)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Party < out[b].Party })
	return out, nil
}

// CoverLimit is the deposit insurance cover per depositor, in rand minor units. A placeholder (threshold 26): R100 000.
const CoverLimit = 100_000_00

// GetSingleCustomerView implements treasuryv1connect.TreasuryServiceHandler.
func (r Reader) GetSingleCustomerView(ctx context.Context, _ *connect.Request[treasuryv1.GetSingleCustomerViewRequest]) (*connect.Response[treasuryv1.GetSingleCustomerViewResponse], error) {
	var ds []Deposit
	for asset, ledger := range Ledgers {
		type acc struct {
			owner   string
			insured bool
		}
		accs := map[string]acc{}
		var ids []string
		cursor := ""
		for {
			res, err := r.Client.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{EntityCode: r.Entity, Ledger: ledger, Code: 1001, AfterAccountId: cursor, Limit: 500}))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnavailable, err)
			}
			for _, a := range res.Msg.Accounts {
				accs[a.AccountId] = acc{a.OwnerPartyId, a.Insured}
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
				return nil, connect.NewError(connect.CodeUnavailable, err)
			}
			for _, b := range res.Msg.Balances {
				dp, _ := strconv.ParseInt(b.DebitsPosted, 10, 64)
				cp, _ := strconv.ParseInt(b.CreditsPosted, 10, 64)
				a := accs[b.AccountId]
				ds = append(ds, Deposit{Party: a.owner, Asset: asset, Insured: a.insured, Balance: cp - dp})
			}
		}
	}
	view, err := SingleCustomerView(ds, CoverLimit, r.Rates)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	s := func(v int64) string { return strconv.FormatInt(v, 10) }
	out := &treasuryv1.GetSingleCustomerViewResponse{AsOf: time.Now().UTC().Format(time.RFC3339), CoverLimitMinor: s(CoverLimit)}
	var cov, unc int64
	for _, d := range view {
		cov += d.Covered
		unc += d.Uncovered
		out.Depositors = append(out.Depositors, &treasuryv1.DepositorView{PartyId: d.Party, Accounts: uint32(d.Accounts),
			InsuredEligibleMinor: s(d.Eligible), CoveredMinor: s(d.Covered), UncoveredMinor: s(d.Uncovered)})
	}
	out.TotalCoveredMinor, out.TotalUncoveredMinor = s(cov), s(unc)
	return connect.NewResponse(out), nil
}
