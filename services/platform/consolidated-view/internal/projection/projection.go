// Package projection applies committed ledger transfers to the read model, one event at a time, inside the consumer's
// transaction.
package projection

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/store"
)

// TopicTransferCommitted is the ledger's topic for every committed transfer (ARCHITECTURE.md Appendix C).
const TopicTransferCommitted = "forge.bank.ledger.transfer.committed.v1"

// Handle applies one event. It is the eventbus handler for this service.
func Handle(ctx context.Context, tx pgx.Tx, env *eventsv1.EventEnvelope) error {
	var t eventsv1.TransferCommitted
	if err := events.Decode(env, TopicTransferCommitted, &t); err != nil {
		return fmt.Errorf("projection: %w", err)
	}
	amount, err := strconv.ParseInt(t.AmountMinor, 10, 64)
	if err != nil {
		return fmt.Errorf("projection: amount %q: %w", t.AmountMinor, err)
	}
	return store.Project(ctx, tx, store.Transfer{
		ID: t.TransferId, Asset: t.Asset, Debit: t.DebitAccountId, Credit: t.CreditAccountId,
		Amount: amount, Code: t.Code, Pending: t.Pending, At: env.OccurredAt.AsTime(),
	})
}
