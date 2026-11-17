package deposits

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	depositsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/deposits/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/services/bank/deposits-service/internal/ledgerfake"
)

type owners map[string]map[string]string

func (o owners) Accounts(_ context.Context, party string) (map[string]string, error) {
	return o[party], nil
}

type world struct {
	svc     *Service
	ledger  *ledgerfake.Ledger
	current string
	clock   *time.Time
}

func setup(t *testing.T) *world {
	t.Helper()
	dsn := os.Getenv("DEPOSITS_TEST_DSN")
	if dsn == "" {
		t.Skip("DEPOSITS_TEST_DSN not set")
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
	if _, err := pool.Exec(ctx, `truncate move, notice, interest_payment, accrual, deposit_account`); err != nil {
		t.Fatal(err)
	}
	fake, client := ledgerfake.New(t)
	current := fake.Open(true)
	fake.Seed(current, 10_000_000) // R100 000.00
	clock := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	svc := &Service{Pool: pool, Ledger: client, Products: DefaultProducts(), Entity: "BANK",
		Owners: owners{"party-1": {current: "ZAR"}, "party-2": {}}, Now: func() time.Time { return clock }}
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	return &world{svc: svc, ledger: fake, current: current, clock: &clock}
}

func (w *world) open(t *testing.T, product string) *depositsv1.DepositAccount {
	t.Helper()
	res, err := w.svc.OpenDepositAccount(context.Background(), connect.NewRequest(&depositsv1.OpenDepositAccountRequest{PartyId: "party-1", ProductId: product, Actor: "Thandi"}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Account
}

func (w *world) deposit(t *testing.T, key, to string, amount int64) error {
	t.Helper()
	_, err := w.svc.Deposit(context.Background(), connect.NewRequest(&depositsv1.DepositRequest{
		IdempotencyKey: key, PartyId: "party-1", FromAccountId: w.current, ToAccountId: to, AmountMinor: fmt.Sprint(amount), Actor: "Thandi"}))
	return err
}

func (w *world) withdraw(t *testing.T, key, from string, amount int64) (*depositsv1.WithdrawResponse, error) {
	t.Helper()
	res, err := w.svc.Withdraw(context.Background(), connect.NewRequest(&depositsv1.WithdrawRequest{
		IdempotencyKey: key, PartyId: "party-1", FromAccountId: from, ToAccountId: w.current, AmountMinor: fmt.Sprint(amount), Actor: "Thandi"}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// A savings account takes deposits and pays withdrawals at once; a repeat of either is replayed, not repeated.
func TestSavingsMovesMoneyBothWaysOnce(t *testing.T) {
	w := setup(t)
	acc := w.open(t, "savings")
	if again := w.open(t, "savings"); again.AccountId != acc.AccountId {
		t.Fatal("opening savings twice made two accounts")
	}
	for i := 0; i < 2; i++ {
		if err := w.deposit(t, "dep-1", acc.AccountId, 5_000_000); err != nil {
			t.Fatal(err)
		}
	}
	if w.ledger.Balance(acc.AccountId) != 5_000_000 {
		t.Fatalf("savings = %d after a deposit sent twice", w.ledger.Balance(acc.AccountId))
	}
	if err := w.deposit(t, "dep-1", acc.AccountId, 1); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("the same key for another amount: %v", err)
	}
	res, err := w.withdraw(t, "wd-1", acc.AccountId, 1_000_000)
	if err != nil || res.State != "MOVED" || w.ledger.Balance(acc.AccountId) != 4_000_000 {
		t.Fatalf("withdraw: %+v, %v, savings %d", res, err, w.ledger.Balance(acc.AccountId))
	}
	if _, err := w.withdraw(t, "wd-2", acc.AccountId, 4_000_001); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("more than the balance: %v", err)
	}
	if w.ledger.Sum() != 0 {
		t.Fatal("the books do not balance")
	}
}

// Another party can neither deposit into, nor withdraw from, nor see a deposit account.
func TestAnotherPartyCannotTouchTheAccount(t *testing.T) {
	w := setup(t)
	acc := w.open(t, "savings")
	_ = w.deposit(t, "dep", acc.AccountId, 100_000)
	_, err := w.svc.Withdraw(context.Background(), connect.NewRequest(&depositsv1.WithdrawRequest{
		IdempotencyKey: "x", PartyId: "party-2", FromAccountId: acc.AccountId, ToAccountId: w.current, AmountMinor: "1", Actor: "Mallory"}))
	if c := connect.CodeOf(err); c != connect.CodePermissionDenied && c != connect.CodeNotFound {
		t.Fatalf("another party withdrawing: %v", err)
	}
	res, _ := w.svc.ListDepositAccounts(context.Background(), connect.NewRequest(&depositsv1.ListDepositAccountsRequest{PartyId: "party-2"}))
	if len(res.Msg.Accounts) != 0 {
		t.Fatal("another party sees the account")
	}
	if _, err := w.svc.OpenDepositAccount(context.Background(), connect.NewRequest(&depositsv1.OpenDepositAccountRequest{PartyId: "party-2", ProductId: "savings", Actor: "x"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a party with no current account opening savings: %v", err)
	}
}

// A notice account gives notice instead of paying; the money is set aside until the due date, then paid once.
// Money under notice cannot be withdrawn again, and a cancelled notice frees it.
func TestNoticeIsGivenSetAsideAndPaidOnTheDueDate(t *testing.T) {
	w := setup(t)
	acc := w.open(t, "notice-32")
	_ = w.deposit(t, "dep", acc.AccountId, 3_000_000)
	res, err := w.withdraw(t, "n-1", acc.AccountId, 2_000_000)
	if err != nil || res.State != "NOTICE_GIVEN" || res.Notice.DueOn != "2026-11-02" {
		t.Fatalf("notice: %+v, %v", res, err)
	}
	if w.ledger.Balance(acc.AccountId) != 3_000_000 {
		t.Fatal("notice moved money before the due date")
	}
	if _, err := w.withdraw(t, "n-2", acc.AccountId, 1_500_000); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("spending money already under notice: %v", err)
	}
	second, err := w.withdraw(t, "n-3", acc.AccountId, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.CancelNotice(context.Background(), connect.NewRequest(&depositsv1.CancelNoticeRequest{PartyId: "party-1", NoticeId: second.Notice.NoticeId})); err != nil {
		t.Fatal(err)
	}

	paid, failed, err := w.svc.PayNotices(context.Background(), time.Date(2026, 11, 1, 23, 0, 0, 0, time.UTC))
	if err != nil || paid != 0 || failed != 0 {
		t.Fatalf("a day early: paid %d failed %d, %v", paid, failed, err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err := w.svc.PayNotices(context.Background(), time.Date(2026, 11, 2, 0, 5, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	if w.ledger.Balance(acc.AccountId) != 1_000_000 || w.ledger.Balance(w.current) != 10_000_000-3_000_000+2_000_000 {
		t.Fatalf("after the due date: notice account %d, current %d", w.ledger.Balance(acc.AccountId), w.ledger.Balance(w.current))
	}
}

// Two withdrawals at once cannot both spend the same money: the account row is locked while each one checks.
func TestConcurrentNoticesCannotOverspend(t *testing.T) {
	w := setup(t)
	acc := w.open(t, "notice-90")
	_ = w.deposit(t, "dep", acc.AccountId, 1_000_000)
	var wg sync.WaitGroup
	ok := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := w.withdraw(t, fmt.Sprintf("race-%d", i), acc.AccountId, 400_000)
			ok <- err == nil
		}(i)
	}
	wg.Wait()
	close(ok)
	n := 0
	for v := range ok {
		if v {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d notices of R4 000 accepted against R10 000; want 2", n)
	}
}

// A month of daily accruals is paid into the account on the 1st, from interest expense, once; the cents that round
// away carry to the next month; accruing a day twice changes nothing.
func TestInterestAccruesDailyAndIsPaidMonthlyOnce(t *testing.T) {
	w := setup(t)
	acc := w.open(t, "savings") // 6.50%
	_ = w.deposit(t, "dep", acc.AccountId, 10_000_000)
	ctx := context.Background()
	for d := 1; d <= 31; d++ {
		day := time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC)
		*w.clock = day.Add(25 * time.Hour)
		for i := 0; i < 2; i++ {
			if _, err := w.svc.Accrue(ctx, day); err != nil {
				t.Fatal(err)
			}
		}
	}
	*w.clock = time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if err := w.svc.RunDaily(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// R100 000 at 6.5% for 31 days is R552.05 (ACT/365), to within a cent below.
	got := w.ledger.Balance(acc.AccountId) - 10_000_000
	if got != 55_205 && got != 55_204 {
		t.Fatalf("October's interest = %d cents, want 55205", got)
	}
	if w.ledger.Balance(w.svc.Expense) != -got || w.ledger.Sum() != 0 {
		t.Fatal("interest did not come from interest expense, or the books do not balance")
	}
	res, _ := w.svc.ListDepositAccounts(ctx, connect.NewRequest(&depositsv1.ListDepositAccountsRequest{PartyId: "party-1"}))
	if res.Msg.Accounts[0].InterestPaidMinor != fmt.Sprint(got) {
		t.Fatalf("interest paid shown = %s", res.Msg.Accounts[0].InterestPaidMinor)
	}
}

// Production refuses the products until a decision approves them; the list says which are available.
func TestProductionOffersOnlyApprovedProducts(t *testing.T) {
	w := setup(t)
	gate, _ := licence.FromEnv(func(k string) string {
		return map[string]string{"FORGE_ENV": "production", "FORGE_APPROVAL_SAVINGS": "DEC-0050"}[k]
	})
	w.svc.Licence = gate
	if _, err := w.svc.OpenDepositAccount(context.Background(), connect.NewRequest(&depositsv1.OpenDepositAccountRequest{PartyId: "party-1", ProductId: "notice-32", Actor: "x"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("an unapproved notice account in production: %v", err)
	}
	w.open(t, "savings")
	res, _ := w.svc.ListProducts(context.Background(), connect.NewRequest(&depositsv1.ListProductsRequest{}))
	for _, p := range res.Msg.Products {
		if p.Available != (p.ProductId == "savings") {
			t.Fatalf("%s available = %v", p.ProductId, p.Available)
		}
	}
}
