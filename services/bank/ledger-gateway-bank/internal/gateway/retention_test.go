package gateway_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/memrepo"
)

func TestRetentionRunsOnSchedule(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := memrepo.New()
	id, _ := ledger.NewID(time.Now())
	if _, _, err := repo.Begin(ctx, "c", "k", []byte("h"), []ledger.ID{id}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(ctx, "c", "k", []byte("r"), nil, nil); err != nil {
		t.Fatal(err)
	}
	// Retention of 0 with a clock one hour ahead makes every committed record eligible.
	future := func() time.Time { return time.Now().Add(time.Hour) }
	done := make(chan struct{})
	go func() {
		gateway.RunRetention(ctx, repo, 0, 10*time.Millisecond, future, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	deadline := time.After(2 * time.Second)
	for {
		_, created, _ := repo.Begin(ctx, "c", "k", []byte("h"), []ledger.ID{id})
		if created {
			break
		}
		select {
		case <-deadline:
			t.Fatal("retention did not purge the committed record")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
}
