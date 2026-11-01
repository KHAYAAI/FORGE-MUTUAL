// Package recon makes the ledger authoritative for the read model. It lists every account in an entity, records each
// account's owner, product and insured status, and sets each balance to the ledger's posted figure. A balance that
// differed is counted as drift: the projection missed something, and it is corrected.
package recon

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/store"
)

// Reconciler reads one entity's ledger and corrects the read model.
type Reconciler struct {
	Ledger ledgerv1connect.LedgerServiceClient
	Entity string
	Store  *store.Store
	Now    func() time.Time
}

// Result is what one run found.
type Result struct {
	Accounts int
	Drifted  int
}

const page = 500

// Run reconciles every account of the entity. It fails, and records nothing, if the ledger cannot be read in full.
func (r *Reconciler) Run(ctx context.Context) (Result, error) {
	accounts, err := r.listAll(ctx)
	if err != nil {
		return Result{}, err
	}
	pool := r.Store.Pool()
	for _, a := range accounts {
		if _, err := pool.Exec(ctx, `
			insert into cv_account (account_id, entity, asset, ledger, code, owner_party_id, product_id, insured)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (account_id) do update set
			  asset = excluded.asset, ledger = excluded.ledger, code = excluded.code,
			  owner_party_id = excluded.owner_party_id, product_id = excluded.product_id, insured = excluded.insured`,
			a.AccountId, r.Entity, a.Asset, int64(a.Ledger), int32(a.Code), a.OwnerPartyId, a.ProductId, a.Insured); err != nil {
			return Result{}, fmt.Errorf("recon: account %s: %w", a.AccountId, err)
		}
	}

	res := Result{Accounts: len(accounts)}
	ids := make([]string, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.AccountId)
	}
	for start := 0; start < len(ids); start += page {
		end := min(start+page, len(ids))
		bal, err := r.Ledger.GetBalances(ctx, connect.NewRequest(&ledgerv1.GetBalancesRequest{
			EntityCode: r.Entity, AccountIds: ids[start:end],
		}))
		if err != nil {
			return Result{}, fmt.Errorf("recon: balances: %w", err)
		}
		if len(bal.Msg.Balances) != end-start {
			return Result{}, errors.New("recon: the ledger returned fewer balances than accounts")
		}
		for _, b := range bal.Msg.Balances {
			debit, err := strconv.ParseInt(b.DebitsPosted, 10, 64)
			if err != nil {
				return Result{}, fmt.Errorf("recon: debits: %w", err)
			}
			credit, err := strconv.ParseInt(b.CreditsPosted, 10, 64)
			if err != nil {
				return Result{}, fmt.Errorf("recon: credits: %w", err)
			}
			drifted, err := r.Store.SetLedgerBalance(ctx, b.AccountId, debit, credit)
			if err != nil {
				return Result{}, err
			}
			if drifted {
				res.Drifted++
			}
		}
	}
	if _, err := pool.Exec(ctx, `insert into cv_recon_run (finished_at, accounts_checked, drifted) values ($1, $2, $3)`,
		r.now().UTC(), res.Accounts, res.Drifted); err != nil {
		return Result{}, fmt.Errorf("recon: record run: %w", err)
	}
	return res, nil
}

func (r *Reconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Reconciler) listAll(ctx context.Context) ([]*ledgerv1.DirectoryAccount, error) {
	var out []*ledgerv1.DirectoryAccount
	cursor := ""
	for i := 0; i < 100000; i++ {
		resp, err := r.Ledger.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{
			EntityCode: r.Entity, Ledger: 0, AfterAccountId: cursor, Limit: 1000,
		}))
		if err != nil {
			return nil, fmt.Errorf("recon: list accounts: %w", err)
		}
		out = append(out, resp.Msg.Accounts...)
		if resp.Msg.NextAfterAccountId == "" {
			return out, nil
		}
		cursor = resp.Msg.NextAfterAccountId
	}
	return nil, errors.New("recon: account listing did not terminate")
}
