package feed

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/cases"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
)

// These tests need PostgreSQL. Set COMPLIANCE_TEST_DSN to a throwaway database; the feed and case tables are truncated.

var base = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("COMPLIANCE_TEST_DSN")
	if dsn == "" {
		t.Skip("COMPLIANCE_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := cases.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate compliance_case, monitor_event`); err != nil {
		t.Fatal(err)
	}
	return pool
}

// settled wraps a settled payment the way payments publishes it, and opens it again as the consumer would.
func settled(t *testing.T, party string, amountMinor string, at time.Time, beneficiary string) *eventsv1.EventEnvelope {
	t.Helper()
	_, payload, err := events.Wrap(events.Params{Type: TopicPaymentSettled, Entity: "BANK", Producer: "payments-orchestrator"},
		&eventsv1.PaymentEvent{PartyId: party, AmountMinor: amountMinor, Asset: "ZAR", Status: "SETTLED", BeneficiaryKey: beneficiary})
	if err != nil {
		t.Fatal(err)
	}
	env, err := events.Open(payload)
	if err != nil {
		t.Fatal(err)
	}
	env.OccurredAt = timestamppb.New(at)
	return env
}

func handle(t *testing.T, pool *pgxpool.Pool, f *Feed, env *eventsv1.EventEnvelope) {
	t.Helper()
	if err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error {
		return f.Handle(context.Background(), tx, env)
	}); err != nil {
		t.Fatal(err)
	}
}

func openCases(t *testing.T, pool *pgxpool.Pool, party string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `select rule from compliance_case where party_id = $1 and status = 'OPEN' order by rule`, party)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// Three payments of R20,000 within an hour cross the R50,000 daily velocity limit. The case opens once, and replaying
// the same payment does not open a second one or count the payment twice.
func TestBreachOpensOneCaseAndReplayIsHarmless(t *testing.T) {
	pool := openPool(t)
	f := &Feed{Cfg: monitor.DefaultConfig(), Now: func() time.Time { return base }}
	const party = "11111111-1111-4111-8111-111111111111"
	var first *eventsv1.EventEnvelope
	for i := 0; i < 3; i++ {
		env := settled(t, party, "2000000", base.Add(time.Duration(i)*10*time.Minute), "payee-a")
		if i == 0 {
			first = env
		}
		handle(t, pool, f, env)
	}
	handle(t, pool, f, first) // redelivery

	got := openCases(t, pool, party)
	if !contains(got, monitor.RuleVelocity) {
		t.Fatalf("open cases = %v; the velocity breach must open a case", got)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `select count(*) from monitor_event where party_id = $1`, party).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("recorded %d events; a redelivered event must count once", n)
	}
	var velocity int
	if err := pool.QueryRow(context.Background(), `select count(*) from compliance_case where party_id = $1 and rule = $2`, party, monitor.RuleVelocity).Scan(&velocity); err != nil {
		t.Fatal(err)
	}
	if velocity != 1 {
		t.Fatalf("velocity cases = %d, want 1", velocity)
	}
}

// A payment that was only requested is not a movement the rules read, and nothing is recorded for it.
func TestEventsTheRulesDoNotReadAreIgnored(t *testing.T) {
	pool := openPool(t)
	f := &Feed{Cfg: monitor.DefaultConfig(), Now: func() time.Time { return base }}
	_, payload, err := events.Wrap(events.Params{Type: TopicPaymentSettled, Entity: "BANK", Producer: "payments-orchestrator"},
		&eventsv1.PaymentEvent{PartyId: "p-req", AmountMinor: "100", Asset: "ZAR", Status: "REQUESTED"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := events.Open(payload)
	if err != nil {
		t.Fatal(err)
	}
	handle(t, pool, f, env)
	var n int
	if err := pool.QueryRow(context.Background(), `select count(*) from monitor_event`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("recorded %d events for a requested payment", n)
	}
}

// Payments outside the window do not count toward a breach: history is read for the last thirty days only.
func TestOldPaymentsDoNotCountTowardABreach(t *testing.T) {
	pool := openPool(t)
	const party = "22222222-2222-4222-8222-222222222222"
	old := &Feed{Cfg: monitor.DefaultConfig(), Now: func() time.Time { return base.Add(-40 * 24 * time.Hour) }}
	for i := 0; i < 2; i++ {
		handle(t, pool, old, settled(t, party, "2000000", base.Add(-40*24*time.Hour+time.Duration(i)*time.Minute), "payee-b"))
	}
	now := &Feed{Cfg: monitor.DefaultConfig(), Now: func() time.Time { return base }}
	handle(t, pool, now, settled(t, party, "2000000", base, "payee-b"))
	if contains(openCases(t, pool, party), monitor.RuleVelocity) {
		t.Fatal("a payment forty days old must not count toward today's velocity")
	}
}

// A credit booked to a customer is an incoming movement; it is recorded with its party and amount.
func TestCreditBookedIsRecordedAsDeposit(t *testing.T) {
	pool := openPool(t)
	f := &Feed{Cfg: monitor.DefaultConfig(), Now: func() time.Time { return base }}
	_, payload, err := events.Wrap(events.Params{Type: TopicCreditBooked, Entity: "BANK", Producer: "payments-orchestrator"},
		&eventsv1.InboundCreditBooked{MovementId: "m-1", PartyId: "p-dep", AccountId: "acc", Asset: "ZAR", AmountMinor: "5000",
			BookedAt: base.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	env, err := events.Open(payload)
	if err != nil {
		t.Fatal(err)
	}
	handle(t, pool, f, env)
	var kind string
	var amount int64
	if err := pool.QueryRow(context.Background(), `select kind, amount_minor from monitor_event where party_id = 'p-dep'`).Scan(&kind, &amount); err != nil {
		t.Fatal(err)
	}
	if kind != monitor.KindDepositIn || amount != 5000 {
		t.Fatalf("recorded %s %d", kind, amount)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
