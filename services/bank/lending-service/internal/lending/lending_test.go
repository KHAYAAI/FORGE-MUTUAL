package lending

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/services/bank/lending-service/internal/ledgerfake"
)

type owners map[string]map[string]string

func (o owners) Accounts(_ context.Context, p string) (map[string]string, error) { return o[p], nil }

// activity is the account's movements, as consolidated-view reports them.
type activity struct {
	mu      sync.Mutex
	entries []*consolidatedv1.ActivityEntry
}

func (a *activity) add(direction string, amount int64, code uint32, at time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, &consolidatedv1.ActivityEntry{Seq: uint64(len(a.entries) + 1), Direction: direction,
		AmountMinor: strconv.FormatInt(amount, 10), Code: code, At: at.Format(time.RFC3339)})
}

func (a *activity) GetActivity(_ context.Context, r *connect.Request[consolidatedv1.GetActivityRequest]) (*connect.Response[consolidatedv1.GetActivityResponse], error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := &consolidatedv1.GetActivityResponse{NextAfterSeq: r.Msg.AfterSeq}
	for _, e := range a.entries {
		if e.Seq > r.Msg.AfterSeq && len(out.Entries) < int(r.Msg.Limit) {
			out.Entries = append(out.Entries, e)
			out.NextAfterSeq = e.Seq
		}
	}
	return connect.NewResponse(out), nil
}

type world struct {
	svc     *Service
	ledger  *ledgerfake.Ledger
	act     *activity
	current string
	clock   *time.Time
}

func setup(t *testing.T) *world {
	t.Helper()
	dsn := os.Getenv("LENDING_TEST_DSN")
	if dsn == "" {
		t.Skip("LENDING_TEST_DSN not set")
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
	if _, err := pool.Exec(ctx, `truncate op, od_charge, od_accrual, instalment, facility_event, default_notice, facility`); err != nil {
		t.Fatal(err)
	}
	fake, client := ledgerfake.New(t)
	current := fake.Open(true)
	clock := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	act := &activity{}
	// R300 000 received in the last 90 days: R100 000 a month. An internal transfer in does not count.
	for _, d := range []int{20, 50, 80} {
		act.add("IN", 10_000_000, 10, clock.AddDate(0, 0, -d))
	}
	act.add("IN", 99_000_000, 12, clock.AddDate(0, 0, -5))
	svc := &Service{Pool: pool, Ledger: client, Owners: owners{"party-1": {current: "ZAR"}}, Activity: act,
		Products: DefaultProducts(), Entity: "BANK", Now: func() time.Time { return clock }}
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	return &world{svc: svc, ledger: fake, act: act, current: current, clock: &clock}
}

func (w *world) apply(t *testing.T, product string, amount int64, term uint32) *lendingv1.Facility {
	t.Helper()
	res, err := w.svc.ApplyForCredit(context.Background(), connect.NewRequest(&lendingv1.ApplyForCreditRequest{
		IdempotencyKey: product + fmt.Sprint(amount), PartyId: "party-1", ProductId: product, AmountMinor: fmt.Sprint(amount),
		TermMonths: term, CurrentAccountId: w.current, Purpose: "stock for the season", Actor: "Thandi"}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Facility
}

func (w *world) approve(t *testing.T, id string, amount string) {
	t.Helper()
	if _, err := w.svc.DecideCreditApplication(context.Background(), connect.NewRequest(&lendingv1.DecideCreditApplicationRequest{
		FacilityId: id, Approve: true, ApprovedMinor: amount, Note: "within policy; strong inflows", Officer: "Sipho (credit)"})); err != nil {
		t.Fatal(err)
	}
}

func (w *world) accept(t *testing.T, id string) *lendingv1.Facility {
	t.Helper()
	res, err := w.svc.AcceptOffer(context.Background(), connect.NewRequest(&lendingv1.AcceptOfferRequest{PartyId: "party-1", FacilityId: id, Actor: "Thandi", AcceptsTerms: true}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Facility
}

// An application is assessed on real inflows only; the applicant cannot decide it; the customer never sees the note.
func TestAnApplicationIsAssessedAndDecidedBySomeoneElse(t *testing.T) {
	w := setup(t)
	f := w.apply(t, "term-loan", 50_000_000, 12)
	if f.Status != "ASSESSED" || f.Assessment.Inflow_90DMinor != "30000000" || f.Assessment.SuggestedMaxMinor != "30000000" || len(f.Assessment.Notes) != 1 {
		t.Fatalf("assessment = %+v (status %s)", f.Assessment, f.Status)
	}
	if again := w.apply(t, "term-loan", 50_000_000, 12); again.FacilityId != f.FacilityId {
		t.Fatal("the same application twice made two")
	}
	_, err := w.svc.DecideCreditApplication(context.Background(), connect.NewRequest(&lendingv1.DecideCreditApplicationRequest{
		FacilityId: f.FacilityId, Approve: true, Note: "fine", Officer: "thandi"}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("the applicant deciding: %v", err)
	}
	if _, err := w.svc.DecideCreditApplication(context.Background(), connect.NewRequest(&lendingv1.DecideCreditApplicationRequest{
		FacilityId: f.FacilityId, Approve: true, ApprovedMinor: "60000000", Note: "x", Officer: "Sipho"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("approving more than asked: %v", err)
	}
	w.approve(t, f.FacilityId, "30000000")
	list, _ := w.svc.ListFacilities(context.Background(), connect.NewRequest(&lendingv1.ListFacilitiesRequest{PartyId: "party-1"}))
	if list.Msg.Facilities[0].DecisionNote != "" || list.Msg.Facilities[0].ApprovedMinor != "30000000" {
		t.Fatalf("customer view: %+v", list.Msg.Facilities[0])
	}
}

// A term loan is paid out once on acceptance and collected monthly. An instalment the account cannot cover goes into
// arrears and is collected when the money arrives; early settlement waives the interest not yet due.
func TestATermLoanIsCollectedFallsIntoArrearsAndSettles(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	f := w.apply(t, "term-loan", 12_000_000, 12)
	w.approve(t, f.FacilityId, "")
	if quote := f.TotalRepayableMinor; quote == "" || quote == "0" {
		t.Fatal("no quotation before acceptance")
	}
	f = w.accept(t, f.FacilityId)
	w.accept(t, f.FacilityId) // again: nothing more is paid
	if w.ledger.Balance(w.current) != 12_000_000 || len(f.Schedule) != 12 || f.Schedule[0].DueOn != "2026-10-15" {
		t.Fatalf("payout %d, schedule %d from %v", w.ledger.Balance(w.current), len(f.Schedule), f.Schedule[0].DueOn)
	}
	first, _ := strconv.ParseInt(f.Schedule[0].PrincipalMinor, 10, 64)
	firstInt, _ := strconv.ParseInt(f.Schedule[0].InterestMinor, 10, 64)

	paid, arrears, err := w.svc.Collect(ctx, time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC))
	if err != nil || paid != 1 || arrears != 0 {
		t.Fatalf("first collection: %d paid, %d arrears, %v", paid, arrears, err)
	}
	if w.ledger.Balance(w.svc.Income) != firstInt || w.ledger.Balance(w.current) != 12_000_000-first-firstInt {
		t.Fatal("the instalment did not split into principal and interest")
	}
	// The customer spends everything; November's instalment cannot be collected.
	w.ledger.Seed(w.current, -w.ledger.Balance(w.current))
	if _, arrears, _ = w.svc.Collect(ctx, time.Date(2026, 11, 20, 6, 0, 0, 0, time.UTC)); arrears != 1 {
		t.Fatalf("arrears = %d", arrears)
	}
	got, _ := w.svc.facility(ctx, "party-1", f.FacilityId)
	if got.DaysPastDue != 5 || got.Schedule[1].Status != "ARREARS" {
		t.Fatalf("days past due %d, status %s", got.DaysPastDue, got.Schedule[1].Status)
	}
	// Money arrives; the arrears are collected; then the customer settles early.
	w.ledger.Seed(w.current, 20_000_000)
	if paid, _, _ = w.svc.Collect(ctx, time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)); paid != 1 {
		t.Fatal("arrears were not collected once the money was there")
	}
	got, _ = w.svc.facility(ctx, "party-1", f.FacilityId)
	if got.DaysPastDue != 0 {
		t.Fatal("days past due not cleared")
	}
	owed := got.OutstandingMinor
	if _, err := w.svc.Repay(ctx, connect.NewRequest(&lendingv1.RepayRequest{IdempotencyKey: "part", PartyId: "party-1", FacilityId: f.FacilityId, AmountMinor: "1000", Actor: "Thandi"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a part settlement: %v", err)
	}
	res, err := w.svc.Repay(ctx, connect.NewRequest(&lendingv1.RepayRequest{IdempotencyKey: "settle", PartyId: "party-1", FacilityId: f.FacilityId, AmountMinor: owed, Actor: "Thandi"}))
	if err != nil || res.Msg.Facility.Status != "SETTLED" || res.Msg.Facility.OutstandingMinor != "0" {
		t.Fatalf("settlement: %+v, %v", res.Msg.GetFacility(), err)
	}
	if w.ledger.Sum() != 0 {
		t.Fatal("the books do not balance")
	}
}

// An overdraft is drawn up to its limit, never past it even with concurrent draws; interest accrues daily on what is
// drawn and is charged to the facility monthly.
func TestAnOverdraftIsDrawnWithinItsLimitAndChargedInterest(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	f := w.apply(t, "overdraft", 5_000_000, 0)
	w.approve(t, f.FacilityId, "")
	f = w.accept(t, f.FacilityId)
	if w.ledger.Balance(w.current) != 0 || f.AvailableMinor != "5000000" {
		t.Fatal("an overdraft pays nothing out until it is drawn")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := w.svc.Draw(ctx, connect.NewRequest(&lendingv1.DrawRequest{IdempotencyKey: fmt.Sprint("d", i), PartyId: "party-1", FacilityId: f.FacilityId, AmountMinor: "2000000", Actor: "Thandi"}))
			if err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if ok != 2 || w.ledger.Balance(w.current) != 4_000_000 {
		t.Fatalf("%d draws of R20 000 on a R50 000 limit; current %d", ok, w.ledger.Balance(w.current))
	}
	for d := 1; d <= 30; d++ {
		if err := w.svc.AccrueOverdrafts(ctx, time.Date(2026, 9, 15+d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 0)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := w.svc.ChargeOverdrafts(ctx, "2026-09"); err != nil {
			t.Fatal(err)
		}
	}
	// R40 000 at 16.75% for the 15 September days: R275.34.
	if got := w.ledger.Balance(w.svc.Income); got != 27_534 && got != 27_533 {
		t.Fatalf("September's overdraft interest = %d", got)
	}
}

// A revenue-based advance is repaid with its share of each payment received after it was paid out, split between
// fee and principal, until both are paid.
func TestRevenueBasedFinancingIsRepaidFromPaymentsReceived(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	f := w.apply(t, "revenue-based", 10_000_000, 0)
	w.approve(t, f.FacilityId, "")
	if f.TotalRepayableMinor != "10800000" {
		t.Fatalf("quotation = %s, want the advance plus the 8%% fee", f.TotalRepayableMinor)
	}
	f = w.accept(t, f.FacilityId)
	// Earlier payments are not swept. Then R1 000 000 in arrives over time; 10% of it repays the advance.
	for i := 0; i < 12; i++ {
		w.act.add("IN", 10_000_000, 10, w.clock.AddDate(0, 0, i+1))
		w.ledger.Seed(w.current, 10_000_000)
		if _, err := w.svc.Sweep(ctx); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := w.svc.facility(ctx, "party-1", f.FacilityId)
	if got.Status != "SETTLED" || got.OutstandingMinor != "0" || w.ledger.Balance(w.svc.Income) != 800_000 {
		t.Fatalf("status %s, outstanding %s, fee income %d", got.Status, got.OutstandingMinor, w.ledger.Balance(w.svc.Income))
	}
	if w.ledger.Sum() != 0 {
		t.Fatal("the books do not balance")
	}
}

// Production refuses every credit product until a decision approves it.
func TestProductionRefusesUnapprovedCredit(t *testing.T) {
	w := setup(t)
	w.svc.Licence, _ = licence.FromEnv(func(k string) string { return map[string]string{"FORGE_ENV": "production"}[k] })
	_, err := w.svc.ApplyForCredit(context.Background(), connect.NewRequest(&lendingv1.ApplyForCreditRequest{
		IdempotencyKey: "x", PartyId: "party-1", ProductId: "overdraft", AmountMinor: "1000000", CurrentAccountId: w.current, Actor: "Thandi"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("credit in production without approval: %v", err)
	}
}
