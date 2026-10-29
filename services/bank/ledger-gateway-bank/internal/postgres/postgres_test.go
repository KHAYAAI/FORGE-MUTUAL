package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gatewaytest"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// TestGatewaySuitePostgres runs the shared behaviour suite against PostgreSQL.
// It needs LEDGER_TEST_DSN, for example postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable.
// Tables are truncated before each subtest. Do not point this at a database you care about.
func TestGatewaySuitePostgres(t *testing.T) {
	dsn := os.Getenv("LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("LEDGER_TEST_DSN not set")
	}
	ctx := context.Background()
	repo, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(repo.Close)

	gatewaytest.Run(t, func(t *testing.T) gatewaytest.Env {
		if _, err := repo.pool.Exec(ctx, `truncate idempotency_record, account_directory, outbox`); err != nil {
			t.Fatal(err)
		}
		return gatewaytest.Env{Repo: repo, Store: ledger.NewMemStore(nil)}
	})
}

// TestPendingRecordSurvivesRestart simulates a crash between the ledger call and Complete:
// the record is PENDING with its IDs, the process restarts with a new pool, and the retry
// must get back the same IDs.
func TestPendingRecordSurvivesRestart(t *testing.T) {
	dsn := os.Getenv("LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("LEDGER_TEST_DSN not set")
	}
	ctx := context.Background()
	first, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.pool.Exec(ctx, `truncate idempotency_record`); err != nil {
		t.Fatal(err)
	}
	id1, _ := ledger.NewID(time.Unix(1_800_000_000, 0))
	id2, _ := ledger.NewID(time.Unix(1_800_000_001, 0))
	hash := []byte("restart-hash")
	if _, created, err := first.Begin(ctx, "svc-payments", "restart-key", hash, []ledger.ID{id1, id2}); err != nil || !created {
		t.Fatalf("begin: created=%v err=%v", created, err)
	}
	first.Close()

	second, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	rec, created, err := second.Begin(ctx, "svc-payments", "restart-key", hash, []ledger.ID{ledger.ZeroID, ledger.ZeroID})
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("record should already exist after restart")
	}
	if rec.Status != gateway.StatusPending || len(rec.IDs) != 2 || rec.IDs[0] != id1 || rec.IDs[1] != id2 {
		t.Fatalf("retry did not get the original IDs: %+v", rec)
	}
	if _, _, err := second.Begin(ctx, "svc-payments", "restart-key", []byte("other"), nil); !errors.Is(err, gateway.ErrConflict) {
		t.Fatalf("different hash after restart: want ErrConflict, got %v", err)
	}
}
