package cards

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	cardsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cards/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/services/bank/card-service/internal/ledgerfake"
)

// These tests need PostgreSQL: CARDS_TEST_DSN names a throwaway database whose tables are dropped.

type owners map[string]map[string]string

func (o owners) Accounts(_ context.Context, p string) (map[string]string, error) { return o[p], nil }

type parties map[string]bool

func (p parties) Active(_ context.Context, id string) (bool, error) { return p[id], nil }

type rig struct {
	t      *testing.T
	s      *Service
	ledger *ledgerfake.Fake
	sim    *Simulator
	acct   string // the customer's account
	card   string
	token  string
	now    time.Time
	pty    parties
}

const party = "party-1"

func setup(t *testing.T) *rig {
	t.Helper()
	dsn := os.Getenv("CARDS_TEST_DSN")
	if dsn == "" {
		t.Skip("CARDS_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `drop table if exists card_refund, card_auth, card cascade`); err != nil {
		t.Fatal(err)
	}
	if err := Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	fake := ledgerfake.New()
	mux := http.NewServeMux()
	path, h := ledgerv1ConnectHandler(fake)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := ledgerClient(srv)

	r := &rig{t: t, ledger: fake, sim: &Simulator{Secret: []byte("0123456789abcdef0123")}, now: time.Date(2027, 3, 1, 10, 0, 0, 0, time.UTC), pty: parties{party: true}}
	// The customer's current account, funded with R1 000.00.
	res, err := client.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{IdempotencyKey: "acct-1", EntityCode: "BANK",
		Accounts: []*ledgerv1.AccountSpec{{OwnerPartyId: party, Asset: "ZAR", Code: 1001, DebitsMustNotExceedCredits: true}}}))
	if err != nil {
		t.Fatal(err)
	}
	r.acct = res.Msg.AccountIds[0]
	fake.Seed(r.acct, 100_000)
	r.s = &Service{Pool: pool, Ledger: client, Owners: owners{party: {r.acct: "ZAR"}}, Parties: r.pty, Processor: r.sim, Licence: licence.Gate{}, Entity: "BANK",
		Log: slogDiscard(), Now: func() time.Time { return r.now },
		DefaultPerTxn: 50_000, DefaultDaily: 80_000, MaxPerTxn: 200_000, MaxDaily: 500_000, MaxCardsPerParty: 3, BlockedMCC: map[string]bool{"7995": true}}
	if err := r.s.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	issued, err := r.s.IssueCard(ctx, connect.NewRequest(&cardsv1.IssueCardRequest{IdempotencyKey: "k1", PartyId: party, AccountId: r.acct, Label: "Subscriptions"}))
	if err != nil {
		t.Fatal(err)
	}
	r.card = issued.Msg.Card.CardId
	if err := pool.QueryRow(ctx, `select token from card where card_id = $1`, r.card).Scan(&r.token); err != nil {
		t.Fatal(err)
	}
	return r
}

var seq int

func (r *rig) auth(amount int64, opts ...func(*Event)) (Decision, string) {
	r.t.Helper()
	seq++
	ev := Event{Type: EventAuthorisation, ID: fmt.Sprintf("auth-%d", seq), Token: r.token, AmountMinor: amount, Currency: "ZAR", Merchant: "Acme", MCC: "5734", Country: "ZA"}
	for _, o := range opts {
		o(&ev)
	}
	d, err := r.s.Authorise(context.Background(), ev)
	if err != nil {
		r.t.Fatalf("authorise: %v", err)
	}
	return d, ev.ID
}

func (r *rig) status(auth string) string {
	var s string
	if err := r.s.Pool.QueryRow(context.Background(), `select status from card_auth where auth_id = $1`, auth).Scan(&s); err != nil {
		r.t.Fatal(err)
	}
	return s
}

func (r *rig) exception(auth string) string {
	var s string
	_ = r.s.Pool.QueryRow(context.Background(), `select exception from card_auth where auth_id = $1`, auth).Scan(&s)
	return s
}

func TestIssuingNeedsAnOwnedRandAccountAndIsIdempotent(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	call := func(key, account string) error {
		_, err := r.s.IssueCard(ctx, connect.NewRequest(&cardsv1.IssueCardRequest{IdempotencyKey: key, PartyId: party, AccountId: account, Label: "x"}))
		return err
	}
	if connect.CodeOf(call("k2", "somebody-elses")) != connect.CodeFailedPrecondition {
		t.Fatal("a card needs an account the party owns")
	}
	again, err := r.s.IssueCard(ctx, connect.NewRequest(&cardsv1.IssueCardRequest{IdempotencyKey: "k1", PartyId: party, AccountId: r.acct, Label: "Subscriptions"}))
	if err != nil || again.Msg.Card.CardId != r.card {
		t.Fatalf("a retry must return the same card: %v %v", err, again)
	}
	if connect.CodeOf(call("k1", "other")) != connect.CodeAlreadyExists {
		t.Fatal("same key, different request")
	}
	if err := call("k3", r.acct); err != nil {
		t.Fatal(err)
	}
	if err := call("k4", r.acct); err != nil {
		t.Fatal(err)
	}
	if connect.CodeOf(call("k5", r.acct)) != connect.CodeFailedPrecondition {
		t.Fatal("the open-card limit must hold")
	}
	r.pty[party] = false
	if connect.CodeOf(call("k6", r.acct)) != connect.CodeFailedPrecondition {
		t.Fatal("a frozen party cannot be issued a card")
	}
}

func TestApprovedPurchaseHoldsThenPostsAndNeverHoldsTwice(t *testing.T) {
	r := setup(t)
	d, id := r.auth(30_000)
	if !d.Approved || d.AuthCode == "" {
		t.Fatalf("decision %+v", d)
	}
	if r.ledger.PendingDebit(r.acct) != 30_000 || r.ledger.Balance(r.acct) != 100_000 {
		t.Fatalf("a hold, not a debit: pending %d balance %d", r.ledger.PendingDebit(r.acct), r.ledger.Balance(r.acct))
	}
	again, err := r.s.Authorise(context.Background(), Event{Type: EventAuthorisation, ID: id, Token: r.token, AmountMinor: 30_000, Currency: "ZAR", MCC: "5734"})
	if err != nil || !again.Approved || again.AuthCode != d.AuthCode || r.ledger.PendingDebit(r.acct) != 30_000 {
		t.Fatalf("a replay returns the same answer and holds nothing more: %+v %v pending %d", again, err, r.ledger.PendingDebit(r.acct))
	}
	if err := r.s.Clear(context.Background(), Event{Type: EventClearing, ID: "clr-1", AuthID: id, AmountMinor: 30_000}); err != nil {
		t.Fatal(err)
	}
	if r.status(id) != "CAPTURED" || r.ledger.PendingDebit(r.acct) != 0 || r.ledger.Balance(r.acct) != 70_000 || r.ledger.Balance(r.s.Settlement) != 30_000 {
		t.Fatalf("posted: status %s pending %d balance %d settlement %d", r.status(id), r.ledger.PendingDebit(r.acct), r.ledger.Balance(r.acct), r.ledger.Balance(r.s.Settlement))
	}
	if err := r.s.Clear(context.Background(), Event{Type: EventClearing, ID: "clr-1", AuthID: id, AmountMinor: 30_000}); err != nil || r.ledger.Balance(r.acct) != 70_000 {
		t.Fatalf("the same clearing twice must not debit twice: %v balance %d", err, r.ledger.Balance(r.acct))
	}
}

func TestEveryRuleDeclinesAndADeclineNeverHoldsMoney(t *testing.T) {
	r := setup(t)
	cases := []struct {
		name string
		do   func() (Decision, string)
		want string
	}{
		{"foreign currency", func() (Decision, string) { return r.auth(1000, func(e *Event) { e.Currency = "USD" }) }, ReasonCurrency},
		{"gambling", func() (Decision, string) { return r.auth(1000, func(e *Event) { e.MCC = "7995" }) }, ReasonMerchant},
		{"over the per-transaction limit", func() (Decision, string) { return r.auth(50_001) }, ReasonPerTxn},
		{"more than the account holds", func() (Decision, string) { return r.auth(50_000, func(e *Event) {}) }, ""}, // within limits and funds: approved below
	}
	for _, c := range cases[:3] {
		d, id := c.do()
		if d.Approved || d.Reason != c.want || r.status(id) != "DECLINED" || r.ledger.PendingDebit(r.acct) != 0 {
			t.Fatalf("%s: %+v status %s pending %d", c.name, d, r.status(id), r.ledger.PendingDebit(r.acct))
		}
	}
	// R500 held, then R300 more fits the daily limit of R800 but the account has R1 000 and R500 is held: R600 does not.
	if d, _ := r.auth(50_000); !d.Approved {
		t.Fatal("R500 is within everything")
	}
	if d, id := r.auth(30_000); !d.Approved || r.status(id) != "HELD" {
		t.Fatalf("R300 fits: %+v", d)
	}
	if d, _ := r.auth(10_000); d.Approved || d.Reason != ReasonDaily {
		t.Fatalf("R100 more would pass R800 a day: %+v", d)
	}
	if _, err := r.s.Pool.Exec(context.Background(), `update card set daily_limit = 500000, per_txn_limit = 200000 where card_id = $1`, r.card); err != nil {
		t.Fatal(err)
	}
	if d, _ := r.auth(30_000); d.Approved || d.Reason != ReasonFunds {
		t.Fatalf("R1 000 less R800 held cannot cover R300: %+v", d)
	}
	if r.ledger.PendingDebit(r.acct) != 80_000 {
		t.Fatalf("only the two approved holds remain: %d", r.ledger.PendingDebit(r.acct))
	}
	// A frozen card, a frozen party, an unknown card.
	_, _ = r.s.Pool.Exec(context.Background(), `update card set status='FROZEN' where card_id = $1`, r.card)
	if d, _ := r.auth(1000); d.Approved || d.Reason != ReasonNotActive {
		t.Fatalf("frozen card: %+v", d)
	}
	_, _ = r.s.Pool.Exec(context.Background(), `update card set status='ACTIVE' where card_id = $1`, r.card)
	r.pty[party] = false
	if d, _ := r.auth(1000); d.Approved || d.Reason != ReasonAccountStatus {
		t.Fatalf("frozen party: %+v", d)
	}
	if d, err := r.s.Authorise(context.Background(), Event{Type: EventAuthorisation, ID: "x", Token: "tok_nope", AmountMinor: 1, Currency: "ZAR"}); err != nil || d.Approved || d.Reason != ReasonUnknownCard {
		t.Fatalf("unknown card: %+v %v", d, err)
	}
}

func TestPurchasesAtTheSameMomentCannotPassTheDailyLimit(t *testing.T) {
	r := setup(t)
	_, _ = r.s.Pool.Exec(context.Background(), `update card set daily_limit = 100000, per_txn_limit = 100000 where card_id = $1`, r.card)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := r.s.Authorise(context.Background(), Event{Type: EventAuthorisation, ID: fmt.Sprintf("race-%d", i), Token: r.token, AmountMinor: 30_000, Currency: "ZAR", MCC: "5734"})
			if err == nil && d.Approved {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 3 || r.ledger.PendingDebit(r.acct) != 90_000 {
		t.Fatalf("%d approved, %d held; R300 at a time under R1 000 a day is 3", ok, r.ledger.PendingDebit(r.acct))
	}
}

func TestReversalPartialClearingAndOverCapture(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	_, a := r.auth(40_000)
	if err := r.s.Reverse(ctx, Event{Type: EventReversal, AuthID: a}); err != nil || r.status(a) != "REVERSED" || r.ledger.PendingDebit(r.acct) != 0 {
		t.Fatalf("reversal releases the hold: %v %s %d", err, r.status(a), r.ledger.PendingDebit(r.acct))
	}
	if err := r.s.Reverse(ctx, Event{Type: EventReversal, AuthID: a}); err != nil {
		t.Fatal(err)
	}
	_, b := r.auth(40_000)
	if err := r.s.Clear(ctx, Event{Type: EventClearing, ID: "c2", AuthID: b, AmountMinor: 25_000}); err != nil {
		t.Fatal(err)
	}
	if r.status(b) != "CAPTURED" || r.ledger.Balance(r.acct) != 75_000 || r.ledger.PendingDebit(r.acct) != 0 {
		t.Fatalf("a smaller clearing posts the smaller amount and frees the rest: %s %d %d", r.status(b), r.ledger.Balance(r.acct), r.ledger.PendingDebit(r.acct))
	}
	_, c := r.auth(10_000)
	if err := r.s.Clear(ctx, Event{Type: EventClearing, ID: "c3", AuthID: c, AmountMinor: 15_000}); err != nil {
		t.Fatal(err)
	}
	if r.status(c) != "EXCEPTION" || r.exception(c) != "OVER_CAPTURE" || r.ledger.Balance(r.acct) != 75_000 {
		t.Fatalf("a clearing above the hold posts nothing and goes to an operator: %s %s %d", r.status(c), r.exception(c), r.ledger.Balance(r.acct))
	}
	_, d := r.auth(1_000, func(e *Event) { e.Currency = "USD" })
	if err := r.s.Clear(ctx, Event{Type: EventClearing, ID: "c4", AuthID: d, AmountMinor: 1_000}); err != nil || r.status(d) != "EXCEPTION" {
		t.Fatalf("clearing for a declined authorisation: %v %s", err, r.status(d))
	}
	if err := r.s.Clear(ctx, Event{Type: EventClearing, ID: "c5", AuthID: "no-such", AmountMinor: 1}); err == nil {
		t.Fatal("an unknown authorisation must be reported")
	}
}

func TestRefundsAreBoundedAndIdempotent(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	_, a := r.auth(40_000)
	_ = r.s.Clear(ctx, Event{Type: EventClearing, ID: "c1", AuthID: a, AmountMinor: 40_000})
	refund := func(id string, amt int64) error {
		return r.s.Refund(ctx, Event{Type: EventRefund, ID: id, AuthID: a, AmountMinor: amt})
	}
	if err := refund("r1", 15_000); err != nil || r.ledger.Balance(r.acct) != 75_000 {
		t.Fatalf("partial refund: %v %d", err, r.ledger.Balance(r.acct))
	}
	if err := refund("r1", 15_000); err != nil || r.ledger.Balance(r.acct) != 75_000 {
		t.Fatalf("the same refund again must not pay twice: %v %d", err, r.ledger.Balance(r.acct))
	}
	if err := refund("r2", 25_000); err != nil || r.status(a) != "REFUNDED" || r.ledger.Balance(r.acct) != 100_000 {
		t.Fatalf("the rest: %v %s %d", err, r.status(a), r.ledger.Balance(r.acct))
	}
	if err := refund("r3", 1); err != nil || r.exception(a) != "REFUND_FOR_REFUNDED" && r.exception(a) != "REFUND_EXCEEDS_PURCHASE" {
		t.Fatalf("a refund beyond the purchase goes to an operator: %v %q", err, r.exception(a))
	}
	if r.ledger.Balance(r.acct) != 100_000 {
		t.Fatal("and pays nothing")
	}
}

func TestWhenTheLedgerIsAwayTheAnswerIsNoAndRecoveryLeavesNothingHeld(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	var fail bool
	r.ledger.Fail = func(op, key string) error {
		if fail && op == "CreateTransfers" && strings.HasPrefix(key, "card-auth:") {
			return errors.New("ledger away")
		}
		return nil
	}
	fail = true
	d, id := r.auth(20_000)
	if d.Approved || d.Reason != ReasonUnavailable || r.status(id) != "PENDING" {
		t.Fatalf("no answer from the ledger means no: %+v %s", d, r.status(id))
	}
	fail = false
	if n, err := r.s.Recover(ctx); err != nil || n != 0 {
		t.Fatalf("too soon to recover: %d %v", n, err)
	}
	r.now = r.now.Add(3 * time.Minute)
	n, err := r.s.Recover(ctx)
	if err != nil || n != 1 || r.status(id) != "DECLINED" || r.ledger.PendingDebit(r.acct) != 0 {
		t.Fatalf("recovery closes it and releases anything held: %d %v %s pending %d", n, err, r.status(id), r.ledger.PendingDebit(r.acct))
	}
}

func TestExpiredHoldsAreMarkedAndStillPostWhenFundsAreThere(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	_, id := r.auth(20_000)
	r.now = r.now.Add(8 * 24 * time.Hour)
	if _, err := r.s.Recover(ctx); err != nil || r.status(id) != "EXPIRED" {
		t.Fatalf("the hold is past its time: %v %s", err, r.status(id))
	}
	// The ledger released it at its own timeout; simulate that.
	if _, err := r.ledger.VoidPending(ctx, connect.NewRequest(&ledgerv1.VoidPendingRequest{IdempotencyKey: "expire", PendingTransferIds: []string{"card-auth:" + id + "-0"}})); err != nil {
		t.Fatal(err)
	}
	if err := r.s.Clear(ctx, Event{Type: EventClearing, ID: "late", AuthID: id, AmountMinor: 20_000}); err != nil || r.status(id) != "CAPTURED" || r.ledger.Balance(r.acct) != 80_000 {
		t.Fatalf("a late clearing is paid from the account: %v %s %d", err, r.status(id), r.ledger.Balance(r.acct))
	}
}

func TestACardBelongsToItsPartyAndStatusRulesHold(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	other := connect.NewRequest(&cardsv1.GetCardRequest{PartyId: "party-2", CardId: r.card})
	if _, err := r.s.GetCard(ctx, other); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("someone else's card must look absent: %v", err)
	}
	set := func(to string) error {
		_, err := r.s.SetCardStatus(ctx, connect.NewRequest(&cardsv1.SetCardStatusRequest{PartyId: party, CardId: r.card, Status: to}))
		return err
	}
	if err := set("FROZEN"); err != nil {
		t.Fatal(err)
	}
	if d, _ := r.auth(100); d.Approved {
		t.Fatal("a frozen card approves nothing")
	}
	if err := set("ACTIVE"); err != nil {
		t.Fatal(err)
	}
	if err := set("CLOSED"); err != nil || !r.sim.Closed(r.token) {
		t.Fatalf("closing also closes it at the processor: %v", err)
	}
	if connect.CodeOf(set("ACTIVE")) != connect.CodeFailedPrecondition {
		t.Fatal("a closed card stays closed")
	}
	lim := func(perTxn, daily string) error {
		_, err := r.s.SetLimits(ctx, connect.NewRequest(&cardsv1.SetLimitsRequest{PartyId: party, CardId: r.card, PerTransactionLimitMinor: perTxn, DailyLimitMinor: daily}))
		return err
	}
	if connect.CodeOf(lim("1000", "2000")) != connect.CodeFailedPrecondition {
		t.Fatal("a closed card takes no new limits")
	}
}

func TestLimitsAreBoundedByTheBanksCeilings(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	lim := func(perTxn, daily string) error {
		_, err := r.s.SetLimits(ctx, connect.NewRequest(&cardsv1.SetLimitsRequest{PartyId: party, CardId: r.card, PerTransactionLimitMinor: perTxn, DailyLimitMinor: daily}))
		return err
	}
	if err := lim("10000", "20000"); err != nil {
		t.Fatal(err)
	}
	if connect.CodeOf(lim("20000", "10000")) != connect.CodeInvalidArgument {
		t.Fatal("daily below per-transaction")
	}
	if connect.CodeOf(lim("200001", "300000")) != connect.CodeFailedPrecondition || connect.CodeOf(lim("100", "500001")) != connect.CodeFailedPrecondition {
		t.Fatal("ceilings")
	}
	if d, _ := r.auth(10_001); d.Approved || d.Reason != ReasonPerTxn {
		t.Fatalf("the new limit applies: %+v", d)
	}
}

func TestProductionRefusesWithoutApprovalAndTheWebhookNeedsASignature(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	prod, err := licence.FromEnv(func(k string) string {
		if k == "FORGE_ENV" {
			return "production"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	r.s.Licence = prod
	if _, err := r.s.IssueCard(ctx, connect.NewRequest(&cardsv1.IssueCardRequest{IdempotencyKey: "p", PartyId: party, AccountId: r.acct, Label: "x"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("not approved: %v", err)
	}
	if d, _ := r.auth(100); d.Approved || d.Reason != ReasonNotAvailable {
		t.Fatalf("an unapproved product declines: %+v", d)
	}
	r.s.Licence = licence.Gate{}

	body := []byte(fmt.Sprintf(`{"type":"AUTHORISATION","id":"wh-1","card_token":%q,"amount_minor":5000,"currency":"ZAR","merchant_name":"Acme","mcc":"5734","country":"ZA"}`, r.token))
	post := func(sig string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/webhooks/processor", bytes.NewReader(body))
		req.Header.Set("X-Forge-Signature", sig)
		w := httptest.NewRecorder()
		r.s.Webhook(w, req)
		return w
	}
	if w := post("00"); w.Code != 401 {
		t.Fatalf("an unsigned delivery: %d", w.Code)
	}
	w := post(r.sim.Sign(body))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"approved":true`) {
		t.Fatalf("a signed delivery: %d %s", w.Code, w.Body.String())
	}
}
