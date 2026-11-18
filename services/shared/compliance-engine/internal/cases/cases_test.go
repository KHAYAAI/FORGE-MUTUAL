package cases

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
)

func openTest(t *testing.T) *Store {
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
	if err := Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate compliance_case`); err != nil {
		t.Fatal(err)
	}
	return New(pool)
}

var now = time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

func finding(party string) monitor.Finding {
	return monitor.Finding{Rule: monitor.RuleVelocity, PartyID: party, Severity: "WARNING", Detail: "d", Evidence: []string{"e1"}}
}

// A repeat finding for a party already under review must not open a second case.
func TestOneOpenCasePerPartyAndRule(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	if n, err := s.OpenFromFindings(ctx, []monitor.Finding{finding("p1"), finding("p1")}, "system", now); err != nil || n != 1 {
		t.Fatalf("opened %d, %v; want 1", n, err)
	}
	if n, _ := s.OpenFromFindings(ctx, []monitor.Finding{finding("p1")}, "system", now.Add(time.Hour)); n != 0 {
		t.Fatalf("a repeat opened %d cases", n)
	}
}

// The SLA runs from when the case opened, by severity.
func TestSLAComesFromSeverity(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	crit := finding("p2")
	crit.Severity = "CRITICAL"
	_, _ = s.OpenFromFindings(ctx, []monitor.Finding{crit, finding("p3")}, "system", now)
	overdue, err := s.Overdue(ctx, now.Add(25*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(overdue) != 1 || overdue[0].PartyID != "p2" {
		t.Fatalf("after 25h only the critical case is overdue: %+v", overdue)
	}
}

// The person who opened a case cannot close it. A different person can, with a real note.
func TestMakerCheckerOnClose(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	_, _ = s.OpenFromFindings(ctx, []monitor.Finding{finding("p4")}, "analyst-1", now)
	var id string
	cs, _ := s.Overdue(ctx, now.Add(100*time.Hour))
	id = cs[0].CaseID

	if err := s.Close(ctx, id, "analyst-1", "a long enough note here for the record", now); !errors.Is(err, ErrSameClerk) {
		t.Fatalf("same person closing: %v", err)
	}
	if err := s.Close(ctx, id, "officer-2", "short", now); !errors.Is(err, ErrNoClosingNote) {
		t.Fatalf("short note: %v", err)
	}
	if err := s.Close(ctx, id, "officer-2", "reviewed the payments; explained by a supplier batch", now); err != nil {
		t.Fatalf("close: %v", err)
	}
	c, err := s.Get(ctx, id)
	if err != nil || c.Status != "CLOSED" || c.ClosedBy != "officer-2" {
		t.Fatalf("after close: %+v, %v", c, err)
	}
	if err := s.Close(ctx, id, "officer-3", "closing again should not be possible", now); !errors.Is(err, ErrNotOpen) {
		t.Fatalf("closing twice: %v", err)
	}
}
