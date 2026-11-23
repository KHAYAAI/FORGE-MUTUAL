// Package recon compares the general ledger with the ledger's balances, per GL account. It refuses to compare while
// journals are still queued: a comparison taken mid-drain would show a break that is not real.
package recon

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/chart"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/fineract"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/store"
)

// LedgerIDs maps each asset to its ledger.
var LedgerIDs = map[string]uint32{"ZAR": 710, "USD": 840}

// Result is one run.
type Result struct {
	Checked int
	Breaks  int
	Skipped string
	Detail  []string
}

// Recon runs comparisons.
type Recon struct {
	Pool   *pgxpool.Pool
	Fin    *fineract.Client
	Ledger ledgerv1connect.LedgerServiceClient
	Entity string
	GL     map[string]int64
	Now    func() time.Time
}

// Run compares every mirrored GL account with the ledger, and records the run.
func (r *Recon) Run(ctx context.Context) (Result, error) {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	pending, err := store.PendingCount(ctx, r.Pool)
	if err != nil {
		return Result{}, err
	}
	if pending > 0 {
		res := Result{Skipped: fmt.Sprintf("queue not drained: %d journals pending", pending)}
		return res, store.RecordRun(ctx, r.Pool, now(), 0, 0, res.Skipped, "")
	}

	var res Result
	for _, asset := range chart.Assets {
		for _, e := range chart.Codes {
			gl, ok := r.GL[chart.GLCode(e.Code, asset)]
			if !ok {
				return res, fmt.Errorf("recon: no GL account for %s", chart.GLCode(e.Code, asset))
			}
			ledgerBal, err := r.ledgerBalance(ctx, LedgerIDs[asset], e.Code, e.Debit)
			if err != nil {
				return res, err
			}
			glBal, err := r.Fin.Balance(ctx, gl, e.Debit)
			if err != nil {
				return res, fmt.Errorf("recon: fineract balance %s: %w", chart.GLCode(e.Code, asset), err)
			}
			res.Checked++
			if ledgerBal != glBal {
				res.Breaks++
				res.Detail = append(res.Detail, fmt.Sprintf("%s ledger %d fineract %d (difference %d)",
					chart.GLCode(e.Code, asset), ledgerBal, glBal, ledgerBal-glBal))
			}
		}
	}
	detail := strings.Join(res.Detail, "; ")
	return res, store.RecordRun(ctx, r.Pool, now(), res.Checked, res.Breaks, "", detail)
}

// ledgerBalance sums the posted balances of every ledger account with a code, in minor units. A debit-normal code is
// debits minus credits; a credit-normal code is credits minus debits.
func (r *Recon) ledgerBalance(ctx context.Context, ledger uint32, code uint16, debitNormal bool) (int64, error) {
	var total int64
	cursor := ""
	for {
		resp, err := r.Ledger.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{
			EntityCode: r.Entity, Ledger: ledger, Code: uint32(code), AfterAccountId: cursor, Limit: 500,
		}))
		if err != nil {
			return 0, fmt.Errorf("recon: list %d: %w", code, err)
		}
		if len(resp.Msg.Accounts) > 0 {
			ids := make([]string, 0, len(resp.Msg.Accounts))
			for _, a := range resp.Msg.Accounts {
				ids = append(ids, a.AccountId)
			}
			bal, err := r.Ledger.GetBalances(ctx, connect.NewRequest(&ledgerv1.GetBalancesRequest{EntityCode: r.Entity, AccountIds: ids}))
			if err != nil {
				return 0, fmt.Errorf("recon: balances: %w", err)
			}
			for _, b := range bal.Msg.Balances {
				dp, err := strconv.ParseInt(b.DebitsPosted, 10, 64)
				if err != nil {
					return 0, err
				}
				cp, err := strconv.ParseInt(b.CreditsPosted, 10, 64)
				if err != nil {
					return 0, err
				}
				if debitNormal {
					total += dp - cp
				} else {
					total += cp - dp
				}
			}
		}
		if resp.Msg.NextAfterAccountId == "" {
			return total, nil
		}
		cursor = resp.Msg.NextAfterAccountId
	}
}
