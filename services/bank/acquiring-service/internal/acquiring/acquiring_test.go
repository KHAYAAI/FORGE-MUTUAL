package acquiring

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	acquiringv1 "github.com/khayaai/forge-mutual/pkg/api/forge/acquiring/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/services/bank/acquiring-service/internal/ledgerfake"
)

type owners map[string]map[string]string

func (o owners) Accounts(_ context.Context, p string) (map[string]string, error) { return o[p], nil }

type world struct {
	svc      *Service
	ledger   *ledgerfake.Ledger
	sim      *Simulator
	merchant string
	clock    *time.Time
}

func setup(t *testing.T) *world {
	t.Helper()
	dsn := os.Getenv("ACQUIRING_TEST_DSN")
	if dsn == "" {
		t.Skip("ACQUIRING_TEST_DSN not set")
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
	if _, err := pool.Exec(ctx, `truncate refund, settlement, payment_link`); err != nil {
		t.Fatal(err)
	}
	fake, client := ledgerfake.New(t)
	merchant := fake.Open(true)
	clock := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	sim := &Simulator{PayBaseURL: "http://127.0.0.1:3000", Secret: []byte("acquirer-test-secret-0123456789")}
	svc := &Service{Pool: pool, Ledger: client, Owners: owners{"shop": {merchant: "ZAR"}}, Acquirer: sim, Entity: "BANK",
		MDRBps: 275, Now: func() time.Time { return clock }}
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	return &world{svc: svc, ledger: fake, sim: sim, merchant: merchant, clock: &clock}
}

func (w *world) link(t *testing.T, key string, amount string) *acquiringv1.PaymentLink {
	t.Helper()
	res, err := w.svc.CreatePaymentLink(context.Background(), connect.NewRequest(&acquiringv1.CreatePaymentLinkRequest{
		IdempotencyKey: key, PartyId: "shop", SettlementAccountId: w.merchant, AmountMinor: amount, Description: "Design sprint",
		Reference: "INV-77", MerchantName: "Forge Pilot Studio", Actor: "Thandi"}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Link
}

func (w *world) deliver(t *testing.T, r Result, sign bool) int {
	t.Helper()
	body, _ := json.Marshal(r)
	req := httptest.NewRequest("POST", "/webhooks/acquirer", bytes.NewReader(body))
	if sign {
		req.Header.Set("X-Forge-Signature", w.sim.Sign(body))
	} else {
		req.Header.Set("X-Forge-Signature", "00ff")
	}
	rec := httptest.NewRecorder()
	w.svc.Webhook(rec, req)
	return rec.Code
}

// A link is created once per key; the payer sees only what they pay and to whom, and is sent to the acquirer.
func TestALinkIsCreatedOnceAndCheckedOutAtTheAcquirer(t *testing.T) {
	w := setup(t)
	l := w.link(t, "k1", "250000")
	if again := w.link(t, "k1", "250000"); again.LinkId != l.LinkId || again.Token != l.Token {
		t.Fatal("the same key made a second link")
	}
	if len(l.Token) < 30 {
		t.Fatalf("the token %q is too short to be unguessable", l.Token)
	}
	co, err := w.svc.GetCheckout(context.Background(), connect.NewRequest(&acquiringv1.GetCheckoutRequest{Token: l.Token}))
	if err != nil || co.Msg.MerchantName != "Forge Pilot Studio" || co.Msg.AmountMinor != "250000" || co.Msg.Status != "OPEN" {
		t.Fatalf("checkout: %+v, %v", co.Msg, err)
	}
	st, err := w.svc.StartCheckout(context.Background(), connect.NewRequest(&acquiringv1.StartCheckoutRequest{Token: l.Token}))
	if err != nil || !st.Msg.Simulated || st.Msg.CheckoutUrl != "http://127.0.0.1:3000/pay/"+l.Token+"/simulator" {
		t.Fatalf("start: %+v, %v", st.Msg, err)
	}
	if _, err := w.svc.GetCheckout(context.Background(), connect.NewRequest(&acquiringv1.GetCheckoutRequest{Token: "guess"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("a guessed token: %v", err)
	}
	if _, err := w.svc.CreatePaymentLink(context.Background(), connect.NewRequest(&acquiringv1.CreatePaymentLinkRequest{
		IdempotencyKey: "k2", PartyId: "shop", SettlementAccountId: "someone-elses", AmountMinor: "100", Description: "x", MerchantName: "x", Actor: "x"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("settling to another's account: %v", err)
	}
}

// Only a signed approval for the right amount pays a link, and only once; a forged one is refused.
func TestOnlyASignedApprovalPaysALink(t *testing.T) {
	w := setup(t)
	l := w.link(t, "k1", "250000")
	if code := w.deliver(t, Result{LinkID: l.LinkId, Status: "APPROVED", AmountMinor: 250000}, false); code != http.StatusUnauthorized {
		t.Fatalf("a forged approval: %d", code)
	}
	if code := w.deliver(t, Result{LinkID: l.LinkId, Status: "APPROVED", AmountMinor: 1}, true); code != http.StatusUnprocessableEntity {
		t.Fatalf("an approval for the wrong amount: %d", code)
	}
	for i := 0; i < 2; i++ {
		if code := w.deliver(t, Result{LinkID: l.LinkId, Status: "APPROVED", AmountMinor: 250000, Reference: "acq-1"}, true); code != http.StatusAccepted {
			t.Fatalf("approval %d: %d", i, code)
		}
	}
	co, _ := w.svc.GetCheckout(context.Background(), connect.NewRequest(&acquiringv1.GetCheckoutRequest{Token: l.Token}))
	if co.Msg.Status != "PAID" {
		t.Fatalf("status = %s", co.Msg.Status)
	}
	if _, err := w.svc.StartCheckout(context.Background(), connect.NewRequest(&acquiringv1.StartCheckoutRequest{Token: l.Token})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("paying a paid link again: %v", err)
	}
}

// The day's takings settle the next day, less the merchant discount, once; then a refund is limited to what is left.
func TestTakingsSettleNextDayAndRefundsAreBounded(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	a, b := w.link(t, "a", "100000"), w.link(t, "b", "300000")
	for _, l := range []*acquiringv1.PaymentLink{a, b} {
		if _, err := w.svc.SimulatePayment(ctx, connect.NewRequest(&acquiringv1.SimulatePaymentRequest{Token: l.Token, Approve: true})); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.svc.RefundPayment(ctx, connect.NewRequest(&acquiringv1.RefundPaymentRequest{IdempotencyKey: "r0", PartyId: "shop", LinkId: a.LinkId, AmountMinor: "100", Actor: "Thandi"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a refund before settlement: %v", err)
	}
	if n, _ := w.svc.Settle(ctx); n != 0 {
		t.Fatal("today's takings settled today")
	}
	*w.clock = w.clock.AddDate(0, 0, 1)
	for i := 0; i < 2; i++ {
		if _, err := w.svc.Settle(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// R4 000.00 gross, 2.75% discount R110.00, net R3 890.00.
	if got := w.ledger.Balance(w.merchant); got != 389_000 {
		t.Fatalf("merchant received %d, want 389000", got)
	}
	if w.ledger.Balance(w.svc.Fees) != 11_000 || w.ledger.Balance(w.svc.Settlement) != -400_000 {
		t.Fatal("the fee or the acquirer settlement is wrong")
	}
	st, _ := w.svc.ListSettlements(ctx, connect.NewRequest(&acquiringv1.ListSettlementsRequest{PartyId: "shop"}))
	if len(st.Msg.Settlements) != 1 || st.Msg.Settlements[0].Payments != 2 || st.Msg.Settlements[0].NetMinor != "389000" {
		t.Fatalf("settlements = %+v", st.Msg.Settlements)
	}

	refund := func(key, amount string) error {
		_, err := w.svc.RefundPayment(ctx, connect.NewRequest(&acquiringv1.RefundPaymentRequest{IdempotencyKey: key, PartyId: "shop", LinkId: b.LinkId, AmountMinor: amount, Actor: "Thandi"}))
		return err
	}
	for i := 0; i < 2; i++ {
		if err := refund("r1", "200000"); err != nil {
			t.Fatal(err)
		}
	}
	if err := refund("r2", "100001"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("refunding more than is left: %v", err)
	}
	if w.ledger.Balance(w.merchant) != 189_000 || len(w.sim.Refunds) != 1 {
		t.Fatalf("after one refund sent twice: merchant %d, acquirer refunds %d", w.ledger.Balance(w.merchant), len(w.sim.Refunds))
	}
	if w.ledger.Sum() != 0 {
		t.Fatal("the books do not balance")
	}
}

// Production refuses acquiring until it is approved, and never runs the simulator's checkout.
func TestProductionRefusesAcquiringAndTheSimulator(t *testing.T) {
	w := setup(t)
	l := w.link(t, "k", "1000")
	w.svc.Licence, _ = licence.FromEnv(func(k string) string { return map[string]string{"FORGE_ENV": "production"}[k] })
	if _, err := w.svc.StartCheckout(context.Background(), connect.NewRequest(&acquiringv1.StartCheckoutRequest{Token: l.Token})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("checkout in production without approval: %v", err)
	}
}
