package sender

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/crypt"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/handler"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/store"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/templates"
)

// These tests need NOTIFICATIONS_TEST_DSN, a throwaway database. Tables are truncated.
func setup(t *testing.T) (*pgxpool.Pool, crypt.Key) {
	t.Helper()
	dsn := os.Getenv("NOTIFICATIONS_TEST_DSN")
	if dsn == "" {
		t.Skip("NOTIFICATIONS_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate notif_message, notif_contact`); err != nil {
		t.Fatal(err)
	}
	k, err := crypt.ParseKey(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return pool, k
}

func settled(t *testing.T, eventID, party string) *eventsv1.EventEnvelope {
	t.Helper()
	_, data, err := events.Wrap(events.Params{EventID: eventID, Type: templates.TopicSettled, Entity: "BANK", Producer: "payments"},
		&eventsv1.PaymentEvent{PaymentId: "pay-" + eventID, PartyId: party, AmountMinor: "15000", Asset: "ZAR", FromAccountId: "acc"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := events.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func enqueue(t *testing.T, pool *pgxpool.Pool, env *eventsv1.EventEnvelope) {
	t.Helper()
	if err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error { return handler.Handle(context.Background(), tx, env) }); err != nil {
		t.Fatal(err)
	}
}

func seal(t *testing.T, pool *pgxpool.Pool, k crypt.Key, party, channel, address string) {
	t.Helper()
	sealed, err := k.Seal([]byte(address))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetContact(context.Background(), pool, party, channel, sealed); err != nil {
		t.Fatal(err)
	}
}

// A settled payment reaches the party on both channels, with the address opened only at send time.
func TestSettledPaymentIsDelivered(t *testing.T) {
	pool, k := setup(t)
	ctx := context.Background()
	seal(t, pool, k, "party-1", templates.ChannelEmail, "thandi@example.com")
	seal(t, pool, k, "party-1", templates.ChannelSMS, "+27821234567")
	enqueue(t, pool, settled(t, "evt-1", "party-1"))

	file := filepath.Join(t.TempDir(), "outbox.jsonl")
	s := &Sender{Pool: pool, Key: k, Provider: &FileSink{Path: file}}
	sent, failed, err := s.Pass(ctx)
	if err != nil || sent != 2 || failed != 0 {
		t.Fatalf("pass: sent=%d failed=%d err=%v", sent, failed, err)
	}
	got, _ := os.ReadFile(file)
	if !strings.Contains(string(got), "thandi@example.com") || !strings.Contains(string(got), "ZAR 150.00") {
		t.Fatalf("delivered content = %s", got)
	}
	st, _ := store.Status(ctx, pool, "evt-1")
	if st[templates.ChannelEmail] != store.StatusSent || st[templates.ChannelSMS] != store.StatusSent {
		t.Fatalf("status = %v", st)
	}
}

// A party with no SMS contact gets the email and a visible SKIPPED record for the SMS, not silence.
func TestMissingContactIsRecordedAsSkipped(t *testing.T) {
	pool, k := setup(t)
	seal(t, pool, k, "party-2", templates.ChannelEmail, "u@example.com")
	enqueue(t, pool, settled(t, "evt-2", "party-2"))
	st, _ := store.Status(context.Background(), pool, "evt-2")
	if st[templates.ChannelEmail] != store.StatusPending || st[templates.ChannelSMS] != store.StatusSkipped {
		t.Fatalf("status = %v, want email pending and sms skipped", st)
	}
}

// The same event enqueued twice creates one message per channel.
func TestDuplicateEventEnqueuesOnce(t *testing.T) {
	pool, k := setup(t)
	seal(t, pool, k, "party-3", templates.ChannelEmail, "v@example.com")
	seal(t, pool, k, "party-3", templates.ChannelSMS, "+27820000000")
	env := settled(t, "evt-3", "party-3")
	enqueue(t, pool, env)
	enqueue(t, pool, env)
	var n int
	_ = pool.QueryRow(context.Background(), `select count(*) from notif_message where event_id = 'evt-3'`).Scan(&n)
	if n != 2 {
		t.Fatalf("messages = %d, want 2", n)
	}
}

type flaky struct {
	mu    sync.Mutex
	calls int
}

func (f *flaky) Send(context.Context, string, string, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return errors.New("provider unavailable")
}

// A provider that keeps failing is retried up to the attempt limit, then the message is FAILED and stays visible.
func TestRepeatedFailureEndsInFailed(t *testing.T) {
	pool, k := setup(t)
	ctx := context.Background()
	seal(t, pool, k, "party-4", templates.ChannelEmail, "w@example.com")
	seal(t, pool, k, "party-4", templates.ChannelSMS, "+27823334444")
	enqueue(t, pool, settled(t, "evt-4", "party-4"))

	p := &flaky{}
	s := &Sender{Pool: pool, Key: k, Provider: p, MaxAttempts: 3}
	for i := 0; i < 5; i++ {
		if _, _, err := s.Pass(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if p.calls != 6 {
		t.Fatalf("provider calls = %d, want 6 (two messages, three attempts each)", p.calls)
	}
	st, _ := store.Status(ctx, pool, "evt-4")
	if st[templates.ChannelEmail] != store.StatusFailed || st[templates.ChannelSMS] != store.StatusFailed {
		t.Fatalf("status = %v, want both FAILED", st)
	}
}
