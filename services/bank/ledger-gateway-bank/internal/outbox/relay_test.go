package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/memrepo"
)

type recorder struct {
	failOnce bool
	sent     []string
}

func (r *recorder) Publish(_ context.Context, _ string, _, value []byte) error {
	if r.failOnce {
		r.failOnce = false
		return errors.New("broker unavailable")
	}
	r.sent = append(r.sent, string(value))
	return nil
}

func TestFailedPublishLeavesEventsForRetry(t *testing.T) {
	repo := memrepo.New()
	id, _ := ledger.NewID(testTime())
	events := []gateway.OutboxEvent{
		{EventID: "e1", Topic: "t", Key: []byte("k"), Payload: []byte("one")},
		{EventID: "e2", Topic: "t", Key: []byte("k"), Payload: []byte("two")},
	}
	if _, _, err := repo.Begin(context.Background(), "c", "key", []byte("h"), []ledger.ID{id}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(context.Background(), "c", "key", []byte("r"), nil, events); err != nil {
		t.Fatal(err)
	}

	rec := &recorder{failOnce: true}
	relay := &Relay{Repo: repo, Publisher: rec, BatchSize: 10, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	if _, err := relay.Once(context.Background()); err == nil {
		t.Fatal("want error from failed publish")
	}
	n, err := relay.Once(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("retry: n=%d err=%v", n, err)
	}
	if len(rec.sent) != 2 || rec.sent[0] != "one" || rec.sent[1] != "two" {
		t.Fatalf("sent = %v", rec.sent)
	}
	if n, _ := relay.Once(context.Background()); n != 0 {
		t.Fatalf("outbox not drained: %d", n)
	}
}
