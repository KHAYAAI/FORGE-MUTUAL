package prudential

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	treasuryv1 "github.com/khayaai/forge-mutual/pkg/api/forge/treasury/v1"
)

// CodeNames are the Appendix B account codes.
var CodeNames = map[uint32]string{
	1001: "CUSTOMER_DEPOSIT", 1010: "CUSTOMER_OVERDRAFT", 1020: "CUSTOMER_LOAN", 2001: "OPERATING", 2002: "FEES_INCOME",
	2003: "FX_POOL", 2004: "FX_POSITION", 2005: "SUSPENSE", 2006: "WRITE_OFF", 2007: "INTEREST_EXPENSE", 2008: "INTEREST_INCOME",
	3001: "RAIL_SETTLEMENT_EFT", 3002: "RAIL_SETTLEMENT_PAYSHAP", 3003: "SPONSOR_POOLED", 3010: "AD_PARTNER_SETTLEMENT",
	3020: "ACQUIRER_SETTLEMENT", 5001: "PARTNER_BANK_MIRROR", 5002: "REWARDS_POOL",
}

// Balance is one account's posted totals.
type Balance struct {
	Asset           string
	Code            uint32
	Debits, Credits int64
}

// TBLine is one line of the trial balance.
type TBLine struct {
	Asset           string
	Code            uint32
	Accounts        int
	Debits, Credits int64
}

// Net is credits less debits.
func (l TBLine) Net() int64 { return l.Credits - l.Debits }

// SheetLine is one line of the management balance sheet, in rand.
type SheetLine struct {
	Section, Label string
	Rand           int64
}

// TrialBalance sums balances by asset and code, and checks that each ledger nets to zero.
func TrialBalance(bs []Balance) (lines []TBLine, nets map[string]int64) {
	idx := map[string]int{}
	nets = map[string]int64{}
	for _, b := range bs {
		k := fmt.Sprintf("%s/%d", b.Asset, b.Code)
		i, ok := idx[k]
		if !ok {
			i = len(lines)
			idx[k] = i
			lines = append(lines, TBLine{Asset: b.Asset, Code: b.Code})
		}
		lines[i].Accounts++
		lines[i].Debits += b.Debits
		lines[i].Credits += b.Credits
		nets[b.Asset] += b.Credits - b.Debits
	}
	sort.Slice(lines, func(a, b int) bool {
		if lines[a].Asset != lines[b].Asset {
			return lines[a].Asset < lines[b].Asset
		}
		return lines[a].Code < lines[b].Code
	})
	return lines, nets
}

// section places an account code on the balance sheet.
func section(code uint32) (string, string) {
	switch code {
	case 3001, 3002, 3003, 3010, 5001:
		return "ASSETS", "Cash at the sponsor and partner banks"
	case 1020, 1010:
		return "ASSETS", "Loans and overdrafts"
	case 3020:
		return "ASSETS", "Card takings due from the acquirer"
	case 1001:
		return "LIABILITIES", "Customer deposits"
	}
	return "INTERNAL", CodeNames[code]
}

// BalanceSheet converts the trial balance to rand and groups it. Assets are shown as positive amounts owed to Forge,
// liabilities as positive amounts Forge owes; internal accounts (income, expense, positions, suspense) with their sign.
func BalanceSheet(lines []TBLine, rates map[string]int64) ([]SheetLine, int64, int64, int64, error) {
	agg := map[[2]string]int64{}
	var order [][2]string
	var assets, liabilities, internal int64
	for _, l := range lines {
		net, err := toZAR(l.Net(), l.Asset, rates)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		sec, label := section(l.Code)
		if sec == "LIABILITIES" && l.Asset != "ZAR" {
			label = "Customer deposits in foreign currency"
		}
		k := [2]string{sec, label}
		if _, ok := agg[k]; !ok {
			order = append(order, k)
		}
		switch sec {
		case "ASSETS":
			agg[k] -= net
			assets -= net
		case "LIABILITIES":
			agg[k] += net
			liabilities += net
		default:
			agg[k] += net
			internal += net
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		rank := map[string]int{"ASSETS": 0, "LIABILITIES": 1, "INTERNAL": 2}
		return rank[order[a][0]] < rank[order[b][0]]
	})
	out := make([]SheetLine, 0, len(order))
	for _, k := range order {
		out = append(out, SheetLine{Section: k[0], Label: k[1], Rand: agg[k]})
	}
	return out, assets, liabilities, internal, nil
}

// balances reads every account's posted totals, on every ledger.
func (r Reader) balances(ctx context.Context) ([]Balance, error) {
	var out []Balance
	for asset, ledger := range Ledgers {
		codes := map[string]uint32{}
		var ids []string
		cursor := ""
		for {
			res, err := r.Client.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{EntityCode: r.Entity, Ledger: ledger, AfterAccountId: cursor, Limit: 500}))
			if err != nil {
				return nil, err
			}
			for _, a := range res.Msg.Accounts {
				codes[a.AccountId] = a.Code
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
				return nil, err
			}
			for _, b := range res.Msg.Balances {
				dp, _ := strconv.ParseInt(b.DebitsPosted, 10, 64)
				cp, _ := strconv.ParseInt(b.CreditsPosted, 10, 64)
				out = append(out, Balance{Asset: asset, Code: codes[b.AccountId], Debits: dp, Credits: cp})
			}
		}
	}
	return out, nil
}

// GetTrialBalance implements treasuryv1connect.TreasuryServiceHandler.
func (r Reader) GetTrialBalance(ctx context.Context, _ *connect.Request[treasuryv1.GetTrialBalanceRequest]) (*connect.Response[treasuryv1.GetTrialBalanceResponse], error) {
	bs, err := r.balances(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	lines, nets := TrialBalance(bs)
	sheet, assets, liabilities, internal, err := BalanceSheet(lines, r.Rates)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	s := func(v int64) string { return strconv.FormatInt(v, 10) }
	out := &treasuryv1.GetTrialBalanceResponse{AsOf: now().UTC().Format(time.RFC3339), TotalAssetsMinor: s(assets), TotalLiabilitiesMinor: s(liabilities), InternalMinor: s(internal)}
	for _, l := range lines {
		out.Lines = append(out.Lines, &treasuryv1.TrialBalanceLine{Asset: l.Asset, Code: l.Code, Name: CodeNames[l.Code], Accounts: uint32(l.Accounts),
			DebitsMinor: s(l.Debits), CreditsMinor: s(l.Credits), NetMinor: s(l.Net())})
	}
	assetsSeen := make([]string, 0, len(nets))
	for a := range nets {
		assetsSeen = append(assetsSeen, a)
	}
	sort.Strings(assetsSeen)
	for _, a := range assetsSeen {
		out.Checks = append(out.Checks, &treasuryv1.LedgerCheck{Asset: a, NetMinor: s(nets[a]), Balanced: nets[a] == 0})
	}
	for _, l := range sheet {
		out.BalanceSheet = append(out.BalanceSheet, &treasuryv1.BalanceSheetLine{Section: l.Section, Label: l.Label, RandMinor: s(l.Rand)})
	}
	return connect.NewResponse(out), nil
}
