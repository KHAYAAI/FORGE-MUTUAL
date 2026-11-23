package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/treasury-risk/internal/risk"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TREASURY_TEST_DSN")
	if dsn == "" {
		t.Skip("TREASURY_TEST_DSN not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err := s.pool.Exec(ctx, `truncate treasury_snapshot, treasury_alert, outbox`); err != nil {
		t.Fatal(err)
	}
	return s
}

func outboxRows(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), `select count(*) from outbox where topic = $1`, TopicAlert).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// An alert is raised once and emits one event. It stays raised quietly while it holds, and clears with one more event.
func TestAlertEmitsOneEventPerTransition(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	at := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	cur := risk.Snapshot{Asset: "ZAR", At: at, Deposits: 10000, LargestDeposit: 3000, LargestAccount: "a", Pool: 10000}
	conc := risk.Alert{Kind: risk.KindConcentration, Asset: "ZAR", Severity: risk.SeverityWarning, Detail: "first", MetricBps: 3000, ThresholdBps: 2500}

	if err := s.Commit(ctx, cur, []risk.Alert{conc}); err != nil {
		t.Fatal(err)
	}
	if n := outboxRows(t, s); n != 1 {
		t.Fatalf("raise emitted %d events, want 1", n)
	}

	conc.Detail = "second"
	cur.At = at.Add(time.Minute)
	if err := s.Commit(ctx, cur, []risk.Alert{conc}); err != nil {
		t.Fatal(err)
	}
	if n := outboxRows(t, s); n != 1 {
		t.Fatalf("a held alert must not emit again: %d events", n)
	}
	var detail string
	if err := s.pool.QueryRow(ctx, `select detail from treasury_alert where kind = $1`, risk.KindConcentration).Scan(&detail); err != nil || detail != "second" {
		t.Fatalf("held alert must update its detail: %q, %v", detail, err)
	}

	cur.At = at.Add(2 * time.Minute)
	if err := s.Commit(ctx, cur, nil); err != nil {
		t.Fatal(err)
	}
	if n := outboxRows(t, s); n != 2 {
		t.Fatalf("clearing must emit once more: %d events", n)
	}
	open, _ := s.RaisedAlerts(ctx)
	if len(open) != 0 {
		t.Fatalf("no alert should remain raised: %v", open)
	}
}

// Previous returns the snapshot at or before a time, which the outflow rule compares against.
func TestPreviousReturnsTheLatestAtOrBefore(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	base := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	for i, d := range []int64{9000, 8000} {
		cur := risk.Snapshot{Asset: "ZAR", At: base.Add(time.Duration(i) * 24 * time.Hour), Deposits: d, Pool: d}
		if err := s.Commit(ctx, cur, nil); err != nil {
			t.Fatal(err)
		}
	}
	prev, err := s.Previous(ctx, "ZAR", base.Add(25*time.Hour))
	if err != nil || prev == nil || prev.Deposits != 8000 {
		t.Fatalf("previous = %+v, %v", prev, err)
	}
	none, err := s.Previous(ctx, "ZAR", base.Add(-time.Hour))
	if err != nil || none != nil {
		t.Fatalf("nothing before the first snapshot: %+v, %v", none, err)
	}
}
