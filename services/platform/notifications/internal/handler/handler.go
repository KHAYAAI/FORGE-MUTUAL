// Package handler turns platform events into messages, inside the consumer's transaction.
package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/store"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/templates"
)

// Handle renders the messages for one event and enqueues them in the same transaction that records the event.
func Handle(ctx context.Context, tx pgx.Tx, env *eventsv1.EventEnvelope) error {
	msgs, err := templates.Render(env)
	if errors.Is(err, templates.ErrNotForUs) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("handler: %w", err)
	}
	for _, m := range msgs {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		if err := store.Enqueue(ctx, tx, env.EventId, id, m); err != nil {
			return err
		}
	}
	return nil
}
