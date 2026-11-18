// Package feed turns the platform's money events into monitoring events, and opens compliance cases from what the rules
// find. It is the link that was missing: without it no case ever opens.
//
// Two sources feed the rules. A settled payment is an outgoing movement; a credit booked to a customer is an incoming one.
// Both carry the party, so the rules can judge a party's history. Events are recorded and evaluated in the transaction
// the event bus gives the handler, so a case commits with the event that raised it, and a redelivered event changes nothing.
package feed

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/cases"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
)

// Topics the feed reads.
const (
	TopicPaymentSettled = "forge.bank.payment.settled.v1"
	TopicCreditBooked   = "forge.bank.inbound.credit.booked.v1"
)

// Window is how much history a party's rules look at. The longest rule window is the structuring window, seven days; a
// month leaves room for the rules to change without losing the evidence they need.
const Window = 30 * 24 * time.Hour

//go:embed schema.sql
var schema string

// Open applies the monitoring-event schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("feed: schema: %w", err)
	}
	return nil
}

// Feed records monitoring events and evaluates the rules for the party each one concerns.
type Feed struct {
	Cfg monitor.Config
	Now func() time.Time
}

// Handle is the event bus handler. It runs inside the transaction that records the event.
func (f *Feed) Handle(ctx context.Context, tx pgx.Tx, env *eventsv1.EventEnvelope) error {
	now := f.Now
	if now == nil {
		now = time.Now
	}
	ev, ok, err := ToMonitorEvent(env, now())
	if err != nil {
		return err
	}
	if !ok {
		return nil // an event of a kind the rules do not read
	}
	if _, err := tx.Exec(ctx, `
		insert into monitor_event (event_id, party_id, kind, at, amount_minor, beneficiary)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (event_id) do nothing`,
		ev.ID, ev.PartyID, ev.Kind, ev.At.UTC(), ev.AmountMinor, ev.Beneficiary); err != nil {
		return fmt.Errorf("feed: record: %w", err)
	}

	history, err := history(ctx, tx, ev.PartyID, now().Add(-Window))
	if err != nil {
		return err
	}
	findings := monitor.Evaluate(history, f.Cfg)
	if _, err := cases.OpenFromFindingsTx(ctx, tx, findings, "system", now()); err != nil {
		return fmt.Errorf("feed: open cases: %w", err)
	}
	return nil
}

func history(ctx context.Context, tx pgx.Tx, party string, since time.Time) ([]monitor.Event, error) {
	rows, err := tx.Query(ctx, `
		select event_id, party_id, kind, at, amount_minor, beneficiary
		from monitor_event where party_id = $1 and at >= $2 order by at, event_id`, party, since.UTC())
	if err != nil {
		return nil, fmt.Errorf("feed: history: %w", err)
	}
	defer rows.Close()
	var out []monitor.Event
	for rows.Next() {
		var e monitor.Event
		if err := rows.Scan(&e.ID, &e.PartyID, &e.Kind, &e.At, &e.AmountMinor, &e.Beneficiary); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ToMonitorEvent maps an envelope to the rules' event. The bool is false for an event the rules do not read, such as a
// payment that was only requested. An event with no time of its own is dated now.
func ToMonitorEvent(env *eventsv1.EventEnvelope, now time.Time) (monitor.Event, bool, error) {
	at := now
	if env.OccurredAt != nil {
		at = env.OccurredAt.AsTime()
	}
	switch env.EventType {
	case TopicPaymentSettled:
		p := &eventsv1.PaymentEvent{}
		if err := events.Decode(env, TopicPaymentSettled, p); err != nil {
			return monitor.Event{}, false, err
		}
		if p.Status != "SETTLED" {
			return monitor.Event{}, false, nil
		}
		amount, err := amount(p.AmountMinor)
		if err != nil {
			return monitor.Event{}, false, err
		}
		return monitor.Event{
			ID: env.EventId, PartyID: p.PartyId, Kind: monitor.KindPaymentOut, At: at,
			AmountMinor: amount, Beneficiary: p.BeneficiaryKey,
		}, true, nil
	case TopicCreditBooked:
		c := &eventsv1.InboundCreditBooked{}
		if err := events.Decode(env, TopicCreditBooked, c); err != nil {
			return monitor.Event{}, false, err
		}
		if c.BookedAt != "" {
			if t, err := time.Parse(time.RFC3339, c.BookedAt); err == nil {
				at = t
			}
		}
		amount, err := amount(c.AmountMinor)
		if err != nil {
			return monitor.Event{}, false, err
		}
		return monitor.Event{ID: env.EventId, PartyID: c.PartyId, Kind: monitor.KindDepositIn, At: at, AmountMinor: amount}, true, nil
	}
	return monitor.Event{}, false, nil
}

func amount(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("feed: amount must be a positive whole number of minor units")
	}
	return n, nil
}
