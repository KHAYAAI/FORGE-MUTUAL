// Package integration runs the payment workflow against the real party, policy, ledger and rails services.
// It builds their binaries, starts each with its own Postgres database, and runs the real payment activities
// against them in Temporal's in-process test environment.
//
// Set PAYMENTS_TEST_PG to a Postgres base DSN, for example postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable.
// The test creates throwaway databases and drops them afterwards.
package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/testsuite"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	policyv1connect "github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	railsv1connect "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1/railsv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/payments"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/store"
)

type service struct {
	cmd *exec.Cmd
	url string
}

type stack struct {
	t           *testing.T
	base        string
	dbs         []string
	services    []service
	urls        map[string]string
	paymentsDSN string
}

func randHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func randKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func createDB(t *testing.T, base, name string) string {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect to base database: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "create database "+name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func dropDB(base, name string) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	_, _ = conn.Exec(ctx, "drop database if exists "+name+" with (force)")
}

func build(t *testing.T, dir, importPath, name string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", out, importPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, stderr.String())
	}
	return out
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func (s *stack) start(bin, listenEnv string, env []string) service {
	addr := freeAddr(s.t)
	cmd := exec.Command(bin)
	cmd.Env = append(append(os.Environ(), env...), listenEnv+"="+addr)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		s.t.Fatal(err)
	}
	svc := service{cmd: cmd, url: "http://" + addr}
	s.services = append(s.services, svc)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(svc.url + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return svc
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.t.Fatalf("%s did not become healthy\n%s", listenEnv, out.String())
	return svc
}

func (s *stack) cleanup() {
	for _, svc := range s.services {
		if svc.cmd.Process != nil {
			_ = svc.cmd.Process.Kill()
			_, _ = svc.cmd.Process.Wait()
		}
	}
	for _, db := range s.dbs {
		dropDB(s.base, db)
	}
}

// startStack builds and starts the services the payment calls, and returns the payments database DSN.
func startStack(t *testing.T, base string) *stack {
	t.Helper()
	dir := t.TempDir()
	s := &stack{t: t, base: base, urls: map[string]string{}}
	t.Cleanup(s.cleanup)
	suffix := randHex(t, 4)
	dsn := map[string]string{}
	for _, name := range []string{"party", "policy", "ledger", "payments"} {
		db := "pay_" + name + "_" + suffix
		dsn[name] = createDB(t, base, db)
		s.dbs = append(s.dbs, db)
	}
	s.paymentsDSN = dsn["payments"]

	partyBin := build(t, dir, "github.com/khayaai/forge-mutual/services/platform/party-service/cmd/party-service", "party")
	policyBin := build(t, dir, "github.com/khayaai/forge-mutual/services/shared/policy-service/cmd/policy-service", "policy")
	ledgerBin := build(t, dir, "github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/cmd/ledger-gateway-bank", "ledger")
	railsBin := build(t, dir, "github.com/khayaai/forge-mutual/services/bank/rails-gateway/cmd/rails-gateway", "rails")

	partyCallers := writeFile(t, dir, "party.json", `{"svc-payments":["GetParty"],"svc-test":["CreateParty","SetStatus","SetTier","GetParty"]}`)
	policyCallers := writeFile(t, dir, "policy.json", `{"svc-payments":["Evaluate"]}`)
	ledgerCallers := writeFile(t, dir, "ledger.json", `{"svc-payments":["CreateAccounts","CreateTransfers","PostPending","VoidPending","GetBalances"],"svc-test":["CreateAccounts","CreateTransfers","GetBalances"]}`)
	railsCallers := writeFile(t, dir, "rails.json", `{"svc-payments":["Initiate","Status"]}`)
	repo, _ := filepath.Abs("../../../../../policy")

	party := s.start(partyBin, "PARTY_LISTEN", []string{
		"DATABASE_URL=" + dsn["party"], "PARTY_CALLERS_FILE=" + partyCallers, "PARTY_PUBLISHER=log",
		"PARTY_SEAL_KEY=" + randKey(t), "PARTY_INDEX_KEY=" + randKey(t),
	})
	policy := s.start(policyBin, "POLICY_LISTEN", []string{
		"DATABASE_URL=" + dsn["policy"], "POLICY_CALLERS_FILE=" + policyCallers, "POLICY_PUBLISHER=log",
		"POLICY_BUNDLE_DIR=" + repo, "POLICY_DEV_UNSIGNED=true",
	})
	ledger := s.start(ledgerBin, "LEDGER_LISTEN", []string{
		"DATABASE_URL=" + dsn["ledger"], "LEDGER_CALLERS_FILE=" + ledgerCallers, "LEDGER_STORE=memory", "LEDGER_PUBLISHER=log",
	})
	rails := s.start(railsBin, "RAILS_LISTEN", []string{
		"RAILS_CALLERS_FILE=" + railsCallers, "RAILS_SIMULATION=true",
	})
	s.urls["party"] = party.url
	s.urls["policy"] = policy.url
	s.urls["ledger"] = ledger.url
	s.urls["rails"] = rails.url
	return s
}

// callerClient stamps every request with a caller identity.
func callerClient(name string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(callerauth.DefaultHeader, name)
			return next(ctx, req)
		}
	}))
}

// Shared test rig: one stack, one operating account, and a helper that creates funded customers.
type rig struct {
	t         *testing.T
	s         *stack
	party     partyv1connect.PartyServiceClient
	ledger    ledgerv1connect.LedgerServiceClient
	test      ledgerv1connect.LedgerServiceClient
	testParty partyv1connect.PartyServiceClient
	operating string
	settle    string
	st        *store.Store
	acts      *payments.Activities
}

func newRig(t *testing.T) *rig {
	t.Helper()
	base := os.Getenv("PAYMENTS_TEST_PG")
	if base == "" {
		t.Skip("PAYMENTS_TEST_PG not set")
	}
	s := startStack(t, base)
	ctx := context.Background()
	r := &rig{t: t, s: s}
	r.testParty = partyv1connect.NewPartyServiceClient(http.DefaultClient, s.urls["party"], callerClient("svc-test"))
	r.party = partyv1connect.NewPartyServiceClient(http.DefaultClient, s.urls["party"], callerClient("svc-payments"))
	r.test = ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, s.urls["ledger"], callerClient("svc-test"))
	r.ledger = ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, s.urls["ledger"], callerClient("svc-payments"))

	st, err := store.Open(ctx, s.paymentsDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	r.st = st

	settle, err := payments.EnsureSettlementAccount(ctx, r.ledger, "BANK")
	if err != nil {
		t.Fatalf("settlement account: %v", err)
	}
	r.settle = settle
	again, err := payments.EnsureSettlementAccount(ctx, r.ledger, "BANK")
	if err != nil || again != settle {
		t.Fatalf("the settlement account must be the same on every start: %s vs %s (%v)", settle, again, err)
	}

	op, err := r.test.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: "op-" + randHex(t, 4), BusinessTxnId: newUUID(t), EntityCode: "BANK",
		Accounts: []*ledgerv1.AccountSpec{{Asset: "ZAR", Code: 2001}}, Actor: "test",
	}))
	if err != nil {
		t.Fatal(err)
	}
	r.operating = op.Msg.AccountIds[0]

	screen, err := screening.NewFixture([]string{"Blocked Person"}, time.Now(), time.Hour, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	r.acts = &payments.Activities{
		Store:      st,
		Party:      r.party,
		Policy:     policyv1connect.NewPolicyServiceClient(http.DefaultClient, s.urls["policy"], callerClient("svc-payments")),
		Ledger:     r.ledger,
		Rails:      railsv1connect.NewRailServiceClient(http.DefaultClient, s.urls["rails"], callerClient("svc-payments")),
		Screener:   screen,
		Recon:      payments.StaticRecon{Break: false},
		Settlement: settle,
		Entity:     "BANK",
	}
	return r
}

func newUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// customer creates an active, standard-tier party with a ledger account funded with the given amount.
func (r *rig) customer(fund int64) (partyID, accountID string) {
	t := r.t
	t.Helper()
	ctx := context.Background()
	p, err := r.testParty.CreateParty(ctx, connect.NewRequest(&partyv1.CreatePartyRequest{
		IdempotencyKey: "cust-" + randHex(t, 6), Type: partyv1.PartyType_PARTY_TYPE_PERSON,
		LegalName: "Thandi Dube", NationalId: "8" + randHex(t, 6), Country: "ZA", Actor: "test",
	}))
	if err != nil {
		t.Fatal(err)
	}
	partyID = p.Msg.Party.PartyId
	for _, to := range []partyv1.PartyStatus{partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING, partyv1.PartyStatus_PARTY_STATUS_ACTIVE} {
		if _, err := r.testParty.SetStatus(ctx, connect.NewRequest(&partyv1.SetStatusRequest{PartyId: partyID, ToStatus: to, Actor: "test"})); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.testParty.SetTier(ctx, connect.NewRequest(&partyv1.SetTierRequest{PartyId: partyID, Tier: "standard", Actor: "test"})); err != nil {
		t.Fatal(err)
	}
	acc, err := r.test.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: "acc-" + randHex(t, 6), BusinessTxnId: newUUID(t), EntityCode: "BANK",
		Accounts: []*ledgerv1.AccountSpec{{OwnerPartyId: partyID, Asset: "ZAR", Code: 1001, DebitsMustNotExceedCredits: true}},
		Actor:    "test",
	}))
	if err != nil {
		t.Fatal(err)
	}
	accountID = acc.Msg.AccountIds[0]
	if fund > 0 {
		res, err := r.test.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
			IdempotencyKey: "fund-" + randHex(t, 6), BusinessTxnId: newUUID(t), EntityCode: "BANK",
			Legs: []*ledgerv1.TransferLeg{{DebitAccountId: r.operating, CreditAccountId: accountID,
				Amount: &ledgerv1.Money{Asset: "ZAR", AmountMinor: fmt.Sprint(fund)}, Code: 10}},
			Actor: "test",
		}))
		if err != nil || res.Msg.Status != ledgerv1.Status_STATUS_OK {
			t.Fatalf("fund customer: %v %v", res, err)
		}
	}
	return partyID, accountID
}

// balance returns the available amount of an account.
func (r *rig) balance(accountID string) string {
	r.t.Helper()
	res, err := r.test.GetBalances(context.Background(), connect.NewRequest(&ledgerv1.GetBalancesRequest{
		EntityCode: "BANK", AccountIds: []string{accountID}}))
	if err != nil || len(res.Msg.Balances) != 1 {
		r.t.Fatalf("balance: %v %v", res, err)
	}
	return res.Msg.Balances[0].AvailableMinor
}

func (r *rig) pending(accountID string) string {
	r.t.Helper()
	res, err := r.test.GetBalances(context.Background(), connect.NewRequest(&ledgerv1.GetBalancesRequest{
		EntityCode: "BANK", AccountIds: []string{accountID}}))
	if err != nil || len(res.Msg.Balances) != 1 {
		r.t.Fatalf("balance: %v %v", res, err)
	}
	return res.Msg.Balances[0].DebitsPending
}

// payment builds a valid payment input for the customer.
func (r *rig) payment(partyID, accountID, amount string) payments.Input {
	return payments.Input{
		PaymentID: newUUID(r.t), RequestHash: "integration", PartyID: partyID, FromAccountID: accountID,
		AmountMinor: amount, Asset: "ZAR", BeneficiaryName: "Sipho Ndlovu", AccountNumber: "1234567890",
		BankCode: "250655", Reference: "invoice 42", Actor: "initiator",
	}
}

// run executes the payment workflow with the real activities.
func (r *rig) run(in payments.Input, signals ...func(env *testsuite.TestWorkflowEnvironment)) payments.Status {
	r.t.Helper()
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(payments.DomesticPayment)
	env.RegisterActivity(r.acts)
	for _, s := range signals {
		s(env)
	}
	env.ExecuteWorkflow(payments.DomesticPayment, in)
	if err := env.GetWorkflowError(); err != nil {
		r.t.Fatalf("workflow error: %v", err)
	}
	var st payments.Status
	if err := env.GetWorkflowResult(&st); err != nil {
		r.t.Fatal(err)
	}
	return st
}

func (r *rig) row(paymentID string) store.Row {
	r.t.Helper()
	row, err := store.Get(context.Background(), r.st.Pool(), paymentID)
	if err != nil {
		r.t.Fatalf("payment row: %v", err)
	}
	return row
}

func (r *rig) eventCount(topic string) int {
	r.t.Helper()
	var n int
	if err := r.st.Pool().QueryRow(context.Background(), `select count(*) from outbox where topic = $1`, topic).Scan(&n); err != nil {
		r.t.Fatal(err)
	}
	return n
}

func TestPaymentsAgainstRealServices(t *testing.T) {
	r := newRig(t)

	t.Run("a settled payment moves the money", func(t *testing.T) {
		party, acc := r.customer(100000)
		in := r.payment(party, acc, "15000")
		st := r.run(in)
		if st.State != payments.StateCompleted {
			t.Fatalf("status = %+v", st)
		}
		if got := r.balance(acc); got != "85000" {
			t.Fatalf("customer available = %s, want 85000", got)
		}
		if got := r.pending(acc); got != "0" {
			t.Fatalf("hold still open: %s", got)
		}
		if row := r.row(in.PaymentID); row.State != payments.StateCompleted {
			t.Fatalf("payment row state = %s", row.State)
		}
		if n := r.eventCount(payments.TopicSettled); n < 1 {
			t.Fatal("a settled payment must publish its event")
		}
	})

	t.Run("a rail rejection gives the money back", func(t *testing.T) {
		party, acc := r.customer(100000)
		in := r.payment(party, acc, "15000")
		in.Scenario = "reject_after:0"
		st := r.run(in)
		if st.State != payments.StateFailed || st.Reason != "rail_rejected" {
			t.Fatalf("status = %+v", st)
		}
		if got := r.balance(acc); got != "100000" {
			t.Fatalf("customer available = %s after a rejection, want 100000", got)
		}
		if got := r.pending(acc); got != "0" {
			t.Fatalf("the hold must be released, pending = %s", got)
		}
	})

	t.Run("insufficient funds declines before the rail", func(t *testing.T) {
		party, acc := r.customer(1000)
		in := r.payment(party, acc, "15000")
		st := r.run(in)
		if st.State != payments.StateDeclined || st.Reason != "insufficient_funds" {
			t.Fatalf("status = %+v", st)
		}
		if got := r.balance(acc); got != "1000" {
			t.Fatalf("a declined payment must not move money: %s", got)
		}
	})

	t.Run("a blocked beneficiary is declined before any hold", func(t *testing.T) {
		party, acc := r.customer(100000)
		in := r.payment(party, acc, "15000")
		in.BeneficiaryName = "Blocked Person"
		st := r.run(in)
		if st.State != payments.StateDeclined || st.Reason != "beneficiary_screening_match" {
			t.Fatalf("status = %+v", st)
		}
		if got := r.pending(acc); got != "0" {
			t.Fatalf("no hold may be placed for a screening match: %s", got)
		}
	})

	t.Run("a large payment waits for a checker", func(t *testing.T) {
		party, acc := r.customer(50000000)
		in := r.payment(party, acc, "10000001") // above the standard tier's step-up threshold
		approve := func(env *testsuite.TestWorkflowEnvironment) {
			env.RegisterDelayedCallback(func() {
				env.SignalWorkflow(payments.SignalApproval, payments.Approval{Approve: true, Approver: "checker"})
			}, time.Minute)
		}
		st := r.run(in, approve)
		if st.State != payments.StateCompleted {
			t.Fatalf("status = %+v", st)
		}
		if got := r.balance(acc); got != "39999999" {
			t.Fatalf("customer available = %s, want 39999999", got)
		}
	})
}
