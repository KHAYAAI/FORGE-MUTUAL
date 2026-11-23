package lending

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
)

func (w *world) at(t time.Time) { *w.clock = t }

// A loan's life on its statements: paid out, an instalment collected, one missed. Twenty days overdue, a default
// notice is issued once; it shows on the facility until the arrears are paid, then clears.
func TestStatementsAndDefaultNotices(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	f := w.apply(t, "term-loan", 6_000_000, 6)
	w.approve(t, f.FacilityId, "")
	f = w.accept(t, f.FacilityId)

	w.at(time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC))
	if paid, _, _ := w.svc.Collect(ctx, *w.clock); paid != 1 {
		t.Fatal("October was not collected")
	}
	w.ledger.Seed(w.current, -w.ledger.Balance(w.current)) // the money is spent
	w.at(time.Date(2026, 11, 15, 6, 0, 0, 0, time.UTC))
	if _, missed, _ := w.svc.Collect(ctx, *w.clock); missed != 1 {
		t.Fatal("November was not missed")
	}
	w.at(time.Date(2026, 12, 2, 6, 0, 0, 0, time.UTC)) // 17 days: too early for a notice
	_, _, _ = w.svc.Collect(ctx, *w.clock)
	if n, _ := w.svc.IssueNotices(ctx); n != 0 {
		t.Fatal("a notice was issued at 17 days")
	}
	w.at(time.Date(2026, 12, 6, 6, 0, 0, 0, time.UTC)) // 21 days
	_, _, _ = w.svc.Collect(ctx, *w.clock)
	for i := 0; i < 2; i++ {
		n, err := w.svc.IssueNotices(ctx)
		if err != nil || n != map[int]int{0: 1, 1: 0}[i] {
			t.Fatalf("issue %d: %d, %v", i, n, err)
		}
	}
	got, _ := w.svc.facility(ctx, "party-1", f.FacilityId)
	if got.DefaultNotice == nil || got.DefaultNotice.DaysPastDue < NoticeAfterDays || got.DefaultNotice.ArrearsMinor == "0" || got.DefaultNotice.RespondBy != "2026-12-20" {
		t.Fatalf("notice = %+v", got.DefaultNotice)
	}

	statement := func(period string) []string {
		res, err := w.svc.GetStatement(ctx, connect.NewRequest(&lendingv1.GetStatementRequest{PartyId: "party-1", FacilityId: f.FacilityId, Period: period}))
		if err != nil {
			t.Fatal(err)
		}
		var kinds []string
		for _, e := range res.Msg.Entries {
			kinds = append(kinds, e.Kind)
		}
		return kinds
	}
	for period, want := range map[string]string{"2026-09": "DISBURSED", "2026-10": "INSTALMENT_COLLECTED", "2026-11": "INSTALMENT_MISSED", "2026-12": "DEFAULT_NOTICE"} {
		if k := statement(period); len(k) != 1 || k[0] != want {
			t.Fatalf("%s statement = %v, want [%s]", period, k, want)
		}
	}
	if _, err := w.svc.GetStatement(ctx, connect.NewRequest(&lendingv1.GetStatementRequest{PartyId: "someone-else", FacilityId: f.FacilityId, Period: "2026-09"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("another party's statement: %v", err)
	}

	w.ledger.Seed(w.current, 10_000_000)
	w.at(time.Date(2026, 12, 7, 6, 0, 0, 0, time.UTC))
	_, _, _ = w.svc.Collect(ctx, *w.clock)
	_, _ = w.svc.IssueNotices(ctx)
	got, _ = w.svc.facility(ctx, "party-1", f.FacilityId)
	if got.DefaultNotice != nil || got.DaysPastDue != 0 {
		t.Fatalf("after paying the arrears: notice %+v, days past due %d", got.DefaultNotice, got.DaysPastDue)
	}
}
