// Package ledgerread turns one ledger into a treasury snapshot. It reads only through the ledger gateway's public API.
package ledgerread

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/bank/treasury-risk/internal/risk"
)

// Account codes (Appendix B).
const (
	codeCustomer = 1001
	codeSuspense = 2005
)

// Asset describes one monitored asset's ledger and its bank-side mirror.
type Asset struct {
	Code     string
	Ledger   uint32
	PoolCode uint32 // 3003 for the sponsor pool (ZAR), 5001 for the partner mirror (USD)
}

// Reader reads snapshots.
type Reader struct {
	Client ledgerv1connect.LedgerServiceClient
	Entity string
	Now    func() time.Time
}

// Snapshot reads every account of the asset's ledger and reduces it to treasury figures.
func (r Reader) Snapshot(ctx context.Context, a Asset) (risk.Snapshot, error) {
	ids := []string{}
	codes := map[string]uint32{}
	cursor := ""
	for {
		resp, err := r.Client.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{
			EntityCode: r.Entity, Ledger: a.Ledger, AfterAccountId: cursor, Limit: 500,
		}))
		if err != nil {
			return risk.Snapshot{}, fmt.Errorf("ledgerread: list: %w", err)
		}
		for _, acc := range resp.Msg.Accounts {
			ids = append(ids, acc.AccountId)
			codes[acc.AccountId] = acc.Code
		}
		if resp.Msg.NextAfterAccountId == "" {
			break
		}
		cursor = resp.Msg.NextAfterAccountId
	}

	snap := risk.Snapshot{Asset: a.Code, At: r.now()}
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		resp, err := r.Client.GetBalances(ctx, connect.NewRequest(&ledgerv1.GetBalancesRequest{
			EntityCode: r.Entity, AccountIds: ids[start:end],
		}))
		if err != nil {
			return risk.Snapshot{}, fmt.Errorf("ledgerread: balances: %w", err)
		}
		for _, b := range resp.Msg.Balances {
			dp, err := strconv.ParseInt(b.DebitsPosted, 10, 64)
			if err != nil {
				return risk.Snapshot{}, fmt.Errorf("ledgerread: debits: %w", err)
			}
			cp, err := strconv.ParseInt(b.CreditsPosted, 10, 64)
			if err != nil {
				return risk.Snapshot{}, fmt.Errorf("ledgerread: credits: %w", err)
			}
			switch code := codes[b.AccountId]; code {
			case codeCustomer:
				bal := cp - dp
				snap.Deposits += bal
				if bal > snap.LargestDeposit {
					snap.LargestDeposit = bal
					snap.LargestAccount = b.AccountId
				}
			case codeSuspense:
				snap.Suspense += cp - dp
			case a.PoolCode:
				snap.Pool += dp - cp
			}
		}
	}
	return snap, nil
}

func (r Reader) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
