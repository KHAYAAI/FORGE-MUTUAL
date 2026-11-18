package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
)

// openTest needs RECON_TEST_DSN, a throwaway database. Tables are truncated.
func openTest(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("RECON_TEST_DSN")
	if dsn == "" {
		t.Skip("RECON_TEST_DSN not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err := s.pool.Exec(ctx, `truncate recon_investigation, recon_break, recon_run`); err != nil {
		t.Fatal(err)
	}
	return s
}

var base = time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

func matched() []recon.Result {
	return []recon.Result{
		{Check: "statement_arithmetic", Matched: true},
		{Check: "sponsor_equation", Matched: true},
	}
}

func broken(diff int64) []recon.Result {
	return []recon.Result{
		{Check: "statement_arithmetic", Matched: true},
		{Check: "sponsor_equation", Matched: false, Difference: diff, Detail: "equation off"},
	}
}

func run(asset string, at time.Time, checks []recon.Result) Run {
	return Run{Asset: asset, StartedAt: at, FinishedAt: at, Outcome: OutcomeConclusive, Checks: checks}
}

func gate(t *testing.T, s *Store, at time.Time, assets ...string) GateState {
	t.Helper()
	g, err := s.Gate(context.Background(), assets, at, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestGateClosedWithoutAnyRun(t *testing.T) {
	s := openTest(t)
	g := gate(t, s, base, "ZAR")
	if g.Open || len(g.Reasons) == 0 || !strings.Contains(g.Reasons[0], "no conclusive") {
		t.Fatalf("gate with no runs must be closed with a reason: %+v", g)
	}
}

func TestFreshMatchedRunOpensGate(t *testing.T) {
	s := openTest(t)
	if err := s.Commit(context.Background(), run("ZAR", base, matched())); err != nil {
		t.Fatal(err)
	}
	if g := gate(t, s, base.Add(time.Minute), "ZAR"); !g.Open {
		t.Fatalf("a fresh matched run must open the gate: %+v", g)
	}
}

func TestBreakClosesGateAndNamesTheCheck(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if err := s.Commit(ctx, run("ZAR", base, broken(-7))); err != nil {
		t.Fatal(err)
	}
	g := gate(t, s, base, "ZAR")
	if g.Open {
		t.Fatal("a break must close the gate")
	}
	if !strings.Contains(strings.Join(g.Reasons, " "), "sponsor_equation") {
		t.Fatalf("reason must name the failing check: %v", g.Reasons)
	}
	open, err := s.ListBreaks(ctx, true)
	if err != nil || len(open) != 1 || open[0].Check != "sponsor_equation" || open[0].DifferenceMinor != -7 {
		t.Fatalf("open breaks = %+v, %v", open, err)
	}
}

func TestMatchingRerunClearsTheBreak(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.Commit(ctx, run("ZAR", base, broken(-7)))
	if err := s.Commit(ctx, run("ZAR", base.Add(time.Minute), matched())); err != nil {
		t.Fatal(err)
	}
	if g := gate(t, s, base.Add(2*time.Minute), "ZAR"); !g.Open {
		t.Fatalf("a matching rerun must reopen the gate: %+v", g)
	}
	all, _ := s.ListBreaks(ctx, false)
	if len(all) != 1 || all[0].Status != StatusCleared || all[0].ClearedAt == nil {
		t.Fatalf("the break must stay in history as cleared: %+v", all)
	}
}

// A check that keeps failing must update one open break, not add a new row each run.
func TestRepeatMismatchUpdatesOneBreak(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.Commit(ctx, run("ZAR", base, broken(-7)))
	_ = s.Commit(ctx, run("ZAR", base.Add(time.Minute), broken(-9)))
	all, _ := s.ListBreaks(ctx, false)
	if len(all) != 1 {
		t.Fatalf("want one break row, got %d", len(all))
	}
	if all[0].DifferenceMinor != -9 || all[0].Status != StatusOpen {
		t.Fatalf("the open break must carry the latest difference: %+v", all[0])
	}
}

func TestStaleRunClosesGate(t *testing.T) {
	s := openTest(t)
	_ = s.Commit(context.Background(), run("ZAR", base, matched()))
	if g := gate(t, s, base.Add(31*time.Minute), "ZAR"); g.Open {
		t.Fatal("a run older than the limit must close the gate")
	}
}

// An inconclusive run proves nothing: it must not clear a break and must not count as a success.
func TestInconclusiveRunChangesNoBreak(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.Commit(ctx, run("ZAR", base, broken(-7)))
	inc := Run{Asset: "ZAR", StartedAt: base.Add(time.Minute), FinishedAt: base.Add(time.Minute), Outcome: OutcomeInconclusive, Checks: matched()}
	if err := s.Commit(ctx, inc); err != nil {
		t.Fatal(err)
	}
	open, _ := s.ListBreaks(ctx, true)
	if len(open) != 1 {
		t.Fatalf("an inconclusive run must not clear the break: %+v", open)
	}
	if g := gate(t, s, base.Add(2*time.Minute), "ZAR"); g.Open {
		t.Fatal("an inconclusive run must not open the gate")
	}
}

// Investigation notes are recorded and audited, but they never open the gate. A difference is cleared only by a
// later run that matches.
func TestInvestigationDoesNotOpenTheGate(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.Commit(ctx, run("ZAR", base, broken(-7)))
	open, _ := s.ListBreaks(ctx, true)
	b, err := s.Investigate(ctx, open[0].BreakID, "ops-jane", "Sponsor reversed a debit; awaiting the sponsor's file.")
	if err != nil {
		t.Fatal(err)
	}
	if b.InvestigationNote == "" || b.InvestigatedBy != "ops-jane" {
		t.Fatalf("note not stored: %+v", b)
	}
	if g := gate(t, s, base.Add(time.Minute), "ZAR"); g.Open {
		t.Fatal("an investigation note must not open the gate")
	}
	var n int
	if err := s.pool.QueryRow(ctx, `select count(*) from recon_investigation where break_id = $1`, b.BreakID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("investigation log rows = %d, %v", n, err)
	}
}

func TestInvestigateRefusesMissingBreakAndEmptyNote(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if _, err := s.Investigate(ctx, "00000000-0000-4000-8000-000000000000", "ops", "x"); err != ErrNotFound {
		t.Fatalf("unknown break err = %v, want ErrNotFound", err)
	}
	_ = s.Commit(ctx, run("ZAR", base, broken(-7)))
	open, _ := s.ListBreaks(ctx, true)
	if _, err := s.Investigate(ctx, open[0].BreakID, "ops", ""); err == nil {
		t.Fatal("an empty note must be refused")
	}
}

// Every asset must have a fresh matched run. One clean asset does not open the gate for another.
func TestEveryAssetMustBeClean(t *testing.T) {
	s := openTest(t)
	_ = s.Commit(context.Background(), run("ZAR", base, matched()))
	if g := gate(t, s, base, "ZAR", "USD"); g.Open {
		t.Fatal("USD has no run; the gate must stay closed")
	}
	_ = s.Commit(context.Background(), run("USD", base, matched()))
	if g := gate(t, s, base.Add(time.Minute), "ZAR", "USD"); !g.Open {
		t.Fatalf("both assets clean must open the gate: %+v", g)
	}
}
