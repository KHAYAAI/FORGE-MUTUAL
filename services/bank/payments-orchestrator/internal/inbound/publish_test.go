package inbound

import (
	"context"
	"testing"
	"time"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
)

// A credit booked to a customer is published once, with the party and the amount. A credit held in suspense belongs to
// no party, so it is not published, and a repeat ingest publishes nothing more.
func TestCustomerCreditIsPublishedOnceAndSuspenseIsNot(t *testing.T) {
	ctx := context.Background()
	w, _, bank, pool := setup(t)
	if _, err := pool.Exec(ctx, `truncate outbox`); err != nil {
		t.Fatal(err)
	}
	if err := w.RegisterReference(ctx, "FORGE-CUST-2", "ZAR", "cust-acc-2", "party-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.InjectInbound("ZAR", 7000, "FORGE-CUST-2", false); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.InjectInbound("ZAR", 900, "NOT-REGISTERED", false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := w.Ingest(ctx, "ZAR", time.Hour); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := pool.Query(ctx, `select payload from outbox where topic = $1 order by seq`, TopicCreditBooked)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []*eventsv1.InboundCreditBooked
	for rows.Next() {
		var p []byte
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		env, err := events.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		ev := &eventsv1.InboundCreditBooked{}
		if err := events.Decode(env, TopicCreditBooked, ev); err != nil {
			t.Fatal(err)
		}
		got = append(got, ev)
	}
	if len(got) != 1 {
		t.Fatalf("published %d credits, want exactly 1 (the customer's)", len(got))
	}
	if got[0].PartyId != "party-2" || got[0].AccountId != "cust-acc-2" || got[0].AmountMinor != "7000" || got[0].Asset != "ZAR" {
		t.Fatalf("event = %+v", got[0])
	}
}
