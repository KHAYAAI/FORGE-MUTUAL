package service

import (
	"context"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	casesv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/cases"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
)

// These tests need PostgreSQL. Set COMPLIANCE_TEST_DSN to a throwaway database; compliance_case is truncated.

var opened = time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

func newService(t *testing.T, now time.Time) (*Service, *cases.Store) {
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
	if _, err := pool.Exec(ctx, `truncate compliance_case`); err != nil {
		t.Fatal(err)
	}
	st := cases.New(pool)
	return &Service{Store: st, Now: func() time.Time { return now }}, st
}

// openOne opens one case through the store, as the monitoring rules do, and returns its id.
func openOne(t *testing.T, st *cases.Store, party string, severity string) string {
	t.Helper()
	ctx := context.Background()
	f := monitor.Finding{Rule: monitor.RuleVelocity, PartyID: party, Severity: severity, Detail: "d", Evidence: []string{"e1"}}
	if n, err := st.OpenFromFindings(ctx, []monitor.Finding{f}, "system", opened); err != nil || n != 1 {
		t.Fatalf("open: %d, %v", n, err)
	}
	cs, err := st.List(ctx, true, 10)
	if err != nil || len(cs) == 0 {
		t.Fatalf("list: %v, %v", cs, err)
	}
	return cs[0].CaseID
}

func code(err error) connect.Code { return connect.CodeOf(err) }

const note = "The velocity was explained by a documented payroll run."

// The person who opened a case cannot close it. A second person can, with a real note, and the case records who closed it.
func TestCloseIsMakerChecker(t *testing.T) {
	s, st := newService(t, opened.Add(time.Hour))
	ctx := context.Background()
	id := openOne(t, st, "11111111-1111-4111-8111-111111111111", "WARNING")

	_, err := s.CloseCase(ctx, connect.NewRequest(&casesv1.CloseCaseRequest{CaseId: id, Actor: "system", Note: note}))
	if code(err) != connect.CodePermissionDenied {
		t.Fatalf("the opener closing the case: want PermissionDenied, got %v", err)
	}

	res, err := s.CloseCase(ctx, connect.NewRequest(&casesv1.CloseCaseRequest{CaseId: id, Actor: "analyst-2", Note: note}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Msg.Case.Status != "CLOSED" || res.Msg.Case.ClosedBy != "analyst-2" || res.Msg.Case.ClosedAt == "" {
		t.Fatalf("closed case = %+v", res.Msg.Case)
	}

	_, err = s.CloseCase(ctx, connect.NewRequest(&casesv1.CloseCaseRequest{CaseId: id, Actor: "analyst-3", Note: note}))
	if code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("closing a closed case: want FailedPrecondition, got %v", err)
	}
}

func TestShortNoteIsRefused(t *testing.T) {
	s, st := newService(t, opened.Add(time.Hour))
	id := openOne(t, st, "11111111-1111-4111-8111-111111111111", "WARNING")
	_, err := s.CloseCase(context.Background(), connect.NewRequest(&casesv1.CloseCaseRequest{CaseId: id, Actor: "analyst-2", Note: "ok"}))
	if code(err) != connect.CodeInvalidArgument {
		t.Fatalf("a short note: want InvalidArgument, got %v", err)
	}
}

// An open case past its SLA is marked overdue when read. One inside its SLA is not.
func TestOverdueIsComputedWhenRead(t *testing.T) {
	ctx := context.Background()
	s, st := newService(t, opened.Add(25*time.Hour))
	crit := openOne(t, st, "11111111-1111-4111-8111-111111111111", "CRITICAL") // SLA 24 hours: overdue at +25h

	res, err := s.ListCases(ctx, connect.NewRequest(&casesv1.ListCasesRequest{OpenOnly: true}))
	if err != nil || len(res.Msg.Cases) != 1 || !res.Msg.Cases[0].Overdue {
		t.Fatalf("list = %+v, %v; want one overdue case", res, err)
	}

	early := &Service{Store: st, Now: func() time.Time { return opened.Add(time.Hour) }}
	got, err := early.GetCase(ctx, connect.NewRequest(&casesv1.GetCaseRequest{CaseId: crit}))
	if err != nil || got.Msg.Case.Overdue {
		t.Fatalf("an hour in, the case is not overdue: %+v, %v", got, err)
	}
}

func TestMissingAndMalformedIDs(t *testing.T) {
	s, _ := newService(t, opened)
	ctx := context.Background()
	if _, err := s.GetCase(ctx, connect.NewRequest(&casesv1.GetCaseRequest{CaseId: "11111111-1111-4111-8111-999999999999"})); code(err) != connect.CodeNotFound {
		t.Fatalf("missing case: want NotFound, got %v", err)
	}
	if _, err := s.GetCase(ctx, connect.NewRequest(&casesv1.GetCaseRequest{CaseId: "not-a-uuid"})); code(err) != connect.CodeInvalidArgument {
		t.Fatalf("malformed id: want InvalidArgument, got %v", err)
	}
	if _, err := s.ListCases(ctx, connect.NewRequest(&casesv1.ListCasesRequest{Limit: 501})); code(err) != connect.CodeInvalidArgument {
		t.Fatalf("limit over 500: want InvalidArgument, got %v", err)
	}
}
