// Package projection queues a journal for every committed ledger transfer that core banking mirrors. It writes only to
// the bridge's own tables, inside the consumer's transaction, so the queue and the event record commit together.
package projection

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/eventbus"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/chart"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/store"
)

// TopicTransferCommitted is the ledger's topic for every committed transfer.
const TopicTransferCommitted = "forge.bank.ledger.transfer.committed.v1"

// Resolver finds an account's code and asset. A miss must try the ledger's directory again, so a newly opened account is
// not an error.
type Resolver interface {
	Resolve(ctx context.Context, accountID string) (store.AccountInfo, error)
}

// Handler returns the eventbus handler for ledger transfers.
func Handler(res Resolver) eventbus.Handler {
	return func(ctx context.Context, tx pgx.Tx, env *eventsv1.EventEnvelope) error {
		var t eventsv1.TransferCommitted
		if err := events.Decode(env, TopicTransferCommitted, &t); err != nil {
			return fmt.Errorf("projection: %w", err)
		}
		if t.Pending {
			return nil // a hold is not a posting; its post event is what reaches the books
		}
		amount, err := strconv.ParseInt(t.AmountMinor, 10, 64)
		if err != nil || amount <= 0 {
			return fmt.Errorf("projection: amount %q is not a positive integer", t.AmountMinor)
		}
		debit, err := res.Resolve(ctx, t.DebitAccountId)
		if err != nil {
			return fmt.Errorf("projection: debit account: %w", err) // retried: the directory may be stale
		}
		credit, err := res.Resolve(ctx, t.CreditAccountId)
		if err != nil {
			return fmt.Errorf("projection: credit account: %w", err)
		}
		if debit.Asset != t.Asset || credit.Asset != t.Asset {
			return store.Queue(ctx, tx, t.TransferId, t.Asset, amount, debit.Code, credit.Code, env.OccurredAt.AsTime(),
				store.StatusFailed, "accounts are in a different asset from the transfer")
		}
		if _, ok := chart.Lookup(debit.Code); !ok {
			return store.Queue(ctx, tx, t.TransferId, t.Asset, amount, debit.Code, credit.Code, env.OccurredAt.AsTime(),
				store.StatusFailed, fmt.Sprintf("ledger code %d is not mirrored in core banking", debit.Code))
		}
		if _, ok := chart.Lookup(credit.Code); !ok {
			return store.Queue(ctx, tx, t.TransferId, t.Asset, amount, debit.Code, credit.Code, env.OccurredAt.AsTime(),
				store.StatusFailed, fmt.Sprintf("ledger code %d is not mirrored in core banking", credit.Code))
		}
		return store.Queue(ctx, tx, t.TransferId, t.Asset, amount, debit.Code, credit.Code, env.OccurredAt.AsTime(),
			store.StatusPending, "")
	}
}

// LedgerResolver reads the cache first and asks the ledger's directory on a miss.
type LedgerResolver struct {
	Lookup  func(ctx context.Context, accountID string) (store.AccountInfo, error) // the cache
	Refresh func(ctx context.Context) error                                        // the directory, into the cache
}

// Resolve implements Resolver.
func (r LedgerResolver) Resolve(ctx context.Context, accountID string) (store.AccountInfo, error) {
	info, err := r.Lookup(ctx, accountID)
	if err == nil {
		return info, nil
	}
	if !errors.Is(err, store.ErrUnknownAccount) {
		return store.AccountInfo{}, err
	}
	if err := r.Refresh(ctx); err != nil {
		return store.AccountInfo{}, err
	}
	return r.Lookup(ctx, accountID)
}
