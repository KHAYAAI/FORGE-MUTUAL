// Package ledgerview reads a consistent view of one ledger from the ledger gateway for reconciliation. It reads
// only through the gateway's public API, never the store.
package ledgerview

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
)

const (
	pageSize    = 500
	maxPages    = 10000
	balanceSize = 500
)

// Reader reads snapshots from one entity's ledger.
type Reader struct {
	Client ledgerv1connect.LedgerServiceClient
	Entity string
}

// Snapshot is the ledger's state at the moment it was read. Digest covers the account set and every posted balance,
// so two snapshots with equal digests describe the same books.
type Snapshot struct {
	Totals   recon.Totals
	Digest   [32]byte
	Accounts int
}

// Snapshot reads every account of the ledger and its posted balances.
func (r Reader) Snapshot(ctx context.Context, ledger uint32) (Snapshot, error) {
	dir, err := r.directory(ctx, ledger)
	if err != nil {
		return Snapshot{}, err
	}
	ids := make([]string, 0, len(dir))
	for id := range dir {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	snap := Snapshot{Totals: recon.NewTotals(), Accounts: len(ids)}
	var lines []string
	for start := 0; start < len(ids); start += balanceSize {
		end := min(start+balanceSize, len(ids))
		resp, err := r.Client.GetBalances(ctx, connect.NewRequest(&ledgerv1.GetBalancesRequest{
			EntityCode: r.Entity, AccountIds: ids[start:end],
		}))
		if err != nil {
			return Snapshot{}, fmt.Errorf("ledgerview: balances: %w", err)
		}
		got := map[string]*ledgerv1.AccountBalance{}
		for _, b := range resp.Msg.Balances {
			got[b.AccountId] = b
		}
		for _, id := range ids[start:end] {
			b, ok := got[id]
			if !ok {
				return Snapshot{}, fmt.Errorf("ledgerview: no balance returned for account %s", id)
			}
			dp, err := strconv.ParseInt(b.DebitsPosted, 10, 64)
			if err != nil {
				return Snapshot{}, fmt.Errorf("ledgerview: debits for %s: %w", id, err)
			}
			cp, err := strconv.ParseInt(b.CreditsPosted, 10, 64)
			if err != nil {
				return Snapshot{}, fmt.Errorf("ledgerview: credits for %s: %w", id, err)
			}
			code := dir[id].Code
			if code > 1<<16-1 {
				return Snapshot{}, fmt.Errorf("ledgerview: account %s has code %d, above the chart's range", id, code)
			}
			if err := snap.Totals.Add(uint16(code), dp, cp); err != nil {
				return Snapshot{}, err
			}
			lines = append(lines, fmt.Sprintf("%s|%d|%s|%d|%d|%s|%s", id, dir[id].Code, dir[id].Asset,
				dp, cp, b.DebitsPending, b.CreditsPending))
		}
	}
	sort.Strings(lines)
	snap.Digest = sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return snap, nil
}

// directory pages every account of the ledger. It returns account ID to directory entry.
func (r Reader) directory(ctx context.Context, ledger uint32) (map[string]*ledgerv1.DirectoryAccount, error) {
	out := map[string]*ledgerv1.DirectoryAccount{}
	cursor := ""
	for page := 0; page < maxPages; page++ {
		resp, err := r.Client.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{
			EntityCode: r.Entity, Ledger: ledger, AfterAccountId: cursor, Limit: pageSize,
		}))
		if err != nil {
			return nil, fmt.Errorf("ledgerview: list accounts: %w", err)
		}
		for _, a := range resp.Msg.Accounts {
			if a.Ledger != ledger {
				return nil, fmt.Errorf("ledgerview: account %s is on ledger %d, not %d", a.AccountId, a.Ledger, ledger)
			}
			if _, dup := out[a.AccountId]; dup {
				return nil, fmt.Errorf("ledgerview: account %s listed twice", a.AccountId)
			}
			out[a.AccountId] = a
		}
		if resp.Msg.NextAfterAccountId == "" {
			return out, nil
		}
		cursor = resp.Msg.NextAfterAccountId
	}
	return nil, errors.New("ledgerview: account listing did not terminate")
}
