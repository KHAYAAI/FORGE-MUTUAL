package memrepo

import (
	"context"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

func TestPurgeRemovesOnlyOldCommittedRecords(t *testing.T) {
	ctx := context.Background()
	repo := New()
	now := time.Unix(1_800_000_000, 0)
	repo.now = func() time.Time { return now }
	id, _ := ledger.NewID(now)

	if _, _, err := repo.Begin(ctx, "c", "done", []byte("h"), []ledger.ID{id}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(ctx, "c", "done", []byte("r"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Begin(ctx, "c", "stuck", []byte("h"), []ledger.ID{id}); err != nil {
		t.Fatal(err)
	}

	n, err := repo.PurgeRecords(ctx, now.Add(time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("purge: n=%d err=%v", n, err)
	}
	// The committed record is gone, so a replay would start over. Retention must outlast the
	// client retry window; that is why the cutoff is a configured value.
	if _, created, _ := repo.Begin(ctx, "c", "done", []byte("h"), []ledger.ID{id}); !created {
		t.Fatal("purged record should be re-created")
	}
	// A PENDING record is never purged, however old.
	rec, created, _ := repo.Begin(ctx, "c", "stuck", []byte("h"), nil)
	if created || rec.Status != gateway.StatusPending {
		t.Fatalf("pending record must survive purge: created=%v status=%s", created, rec.Status)
	}
}
