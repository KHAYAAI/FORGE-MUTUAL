// Package gatewaytest holds the behaviour suite that every gateway.Repository and
// ledger.Store pairing must pass. It is shared by the in-memory and Postgres tests.
package gatewaytest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// Env is a fresh repository and ledger for one subtest.
type Env struct {
	Repo  gateway.Repository
	Store ledger.Store
}

// Factory returns a fresh Env. It is called once per subtest.
type Factory func(t *testing.T) Env

const (
	caller    = "svc-payments"
	entity    = "BANK"
	txnPrefix = "7d7b1c2e-5f0a-4c3b-9a1e-"
)

// Run executes the suite against the factory.
func Run(t *testing.T, newEnv Factory) {
	t.Run("deposit_replay_conflict", func(t *testing.T) { depositReplayConflict(t, newEnv) })
	t.Run("insufficient_funds_is_recorded", func(t *testing.T) { insufficientFundsRecorded(t, newEnv) })
	t.Run("pending_post_and_void", func(t *testing.T) { pendingPostAndVoid(t, newEnv) })
	t.Run("linked_chain_is_atomic", func(t *testing.T) { linkedChainAtomic(t, newEnv) })
	t.Run("crash_after_ledger_commit_is_recovered", func(t *testing.T) { crashRecovered(t, newEnv) })
	t.Run("validation", func(t *testing.T) { validation(t, newEnv) })
	t.Run("outbox_publishes_each_committed_leg_once", func(t *testing.T) { outboxOnce(t, newEnv) })
}

// fixture creates an operating account and a customer account and returns their IDs.
type fixture struct {
	svc      *gateway.Service
	operate  string
	customer string
	env      Env
	seq      int
}

func setup(t *testing.T, newEnv Factory) *fixture {
	t.Helper()
	env := newEnv(t)
	svc := gateway.NewService(gateway.DefaultConfig(), env.Store, env.Repo)
	f := &fixture{svc: svc, env: env}
	resp := f.createAccounts(t, "setup", []*ledgerv1.AccountSpec{
		{Asset: "ZAR", Code: 2001, Insured: false},
		{Asset: "ZAR", Code: 1001, DebitsMustNotExceedCredits: true, Insured: true, OwnerPartyId: txnPrefix + "000000000001"},
	})
	if resp.Status != ledgerv1.Status_STATUS_OK {
		t.Fatalf("setup accounts: %v %v", resp.Status, resp.RejectionCodes)
	}
	f.operate, f.customer = resp.AccountIds[0], resp.AccountIds[1]
	return f
}

func (f *fixture) nextTxn() string {
	f.seq++
	return fmt.Sprintf("%s%012d", txnPrefix, f.seq)
}

func (f *fixture) nextKey(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%d", prefix, f.seq)
}

func withCaller[T any](r *connect.Request[T]) *connect.Request[T] {
	r.Header().Set(gateway.CallerHeader, caller)
	return r
}

func (f *fixture) createAccounts(t *testing.T, key string, specs []*ledgerv1.AccountSpec) *ledgerv1.CreateAccountsResponse {
	t.Helper()
	r := &ledgerv1.CreateAccountsRequest{
		IdempotencyKey: key + "-" + fmt.Sprint(f.seq),
		BusinessTxnId:  f.nextTxn(),
		EntityCode:     entity,
		Accounts:       specs,
		Actor:          "test",
	}
	f.seq++
	res, err := f.svc.CreateAccounts(context.Background(), withCaller(connect.NewRequest(r)))
	if err != nil {
		t.Fatalf("CreateAccounts: %v", err)
	}
	return res.Msg
}

func (f *fixture) transferReq(key string, legs ...*ledgerv1.TransferLeg) *ledgerv1.CreateTransfersRequest {
	return &ledgerv1.CreateTransfersRequest{
		IdempotencyKey: key,
		BusinessTxnId:  f.nextTxn(),
		EntityCode:     entity,
		Legs:           legs,
		Actor:          "test",
	}
}

func leg(debit, credit, asset, amount string, code uint32) *ledgerv1.TransferLeg {
	return &ledgerv1.TransferLeg{
		DebitAccountId:  debit,
		CreditAccountId: credit,
		Amount:          &ledgerv1.Money{Asset: asset, AmountMinor: amount},
		Code:            code,
	}
}

func (f *fixture) send(t *testing.T, r *ledgerv1.CreateTransfersRequest) (*ledgerv1.CreateTransfersResponse, error) {
	t.Helper()
	res, err := f.svc.CreateTransfers(context.Background(), withCaller(connect.NewRequest(r)))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (f *fixture) available(t *testing.T, account string) string {
	t.Helper()
	res, err := f.svc.GetBalances(context.Background(), withCaller(connect.NewRequest(&ledgerv1.GetBalancesRequest{
		EntityCode: entity,
		AccountIds: []string{account},
	})))
	if err != nil {
		t.Fatalf("GetBalances: %v", err)
	}
	if len(res.Msg.Balances) != 1 {
		t.Fatalf("balances: got %d", len(res.Msg.Balances))
	}
	return res.Msg.Balances[0].AvailableMinor
}

func depositReplayConflict(t *testing.T, newEnv Factory) {
	f := setup(t, newEnv)
	key := f.nextKey("deposit")
	r := f.transferReq(key, leg(f.operate, f.customer, "ZAR", "100000", 10))

	first, err := f.send(t, r)
	if err != nil || first.Status != ledgerv1.Status_STATUS_OK || len(first.TransferIds) != 1 {
		t.Fatalf("deposit: err=%v resp=%v", err, first)
	}
	if got := f.available(t, f.customer); got != "100000" {
		t.Fatalf("after deposit available=%s", got)
	}

	replay, err := f.send(t, r)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replay.Status != ledgerv1.Status_STATUS_REPLAYED {
		t.Fatalf("replay status = %v", replay.Status)
	}
	if replay.TransferIds[0] != first.TransferIds[0] {
		t.Fatal("replay returned different transfer id")
	}
	if got := f.available(t, f.customer); got != "100000" {
		t.Fatalf("replay moved money: available=%s", got)
	}

	conflict := f.transferReq(key, leg(f.operate, f.customer, "ZAR", "100001", 10))
	conflict.BusinessTxnId = r.BusinessTxnId
	_, err = f.send(t, conflict)
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("conflicting payload: want AlreadyExists, got %v", err)
	}
}

func insufficientFundsRecorded(t *testing.T, newEnv Factory) {
	f := setup(t, newEnv)
	key := f.nextKey("overdraw")
	r := f.transferReq(key, leg(f.customer, f.operate, "ZAR", "200000", 11))
	res, err := f.send(t, r)
	if err != nil {
		t.Fatalf("overdraw: %v", err)
	}
	if res.Status != ledgerv1.Status_STATUS_REJECTED || res.RejectionCodes[0] != gateway.CodeInsufficientFunds {
		t.Fatalf("overdraw: got %v %v", res.Status, res.RejectionCodes)
	}
	replay, err := f.send(t, r)
	if err != nil {
		t.Fatalf("replay rejected: %v", err)
	}
	if replay.Status != ledgerv1.Status_STATUS_REPLAYED || replay.RejectionCodes[0] != gateway.CodeInsufficientFunds {
		t.Fatalf("replay of rejection: %v %v", replay.Status, replay.RejectionCodes)
	}
}

func pendingPostAndVoid(t *testing.T, newEnv Factory) {
	f := setup(t, newEnv)
	if out, _ := f.send(t, f.transferReq(f.nextKey("fund"), leg(f.operate, f.customer, "ZAR", "100000", 10))); out.Status != ledgerv1.Status_STATUS_OK {
		t.Fatalf("fund: %v", out)
	}

	hold := f.transferReq(f.nextKey("hold"), leg(f.customer, f.operate, "ZAR", "30000", 11))
	hold.Pending = true
	hold.PendingTimeoutSeconds = 600
	held, err := f.send(t, hold)
	if err != nil || held.Status != ledgerv1.Status_STATUS_OK {
		t.Fatalf("hold: %v %v", err, held)
	}
	if got := f.available(t, f.customer); got != "70000" {
		t.Fatalf("hold should reduce available to 70000, got %s", got)
	}

	post, err := f.svc.PostPending(context.Background(), withCaller(connect.NewRequest(&ledgerv1.PostPendingRequest{
		IdempotencyKey:     f.nextKey("post"),
		BusinessTxnId:      f.nextTxn(),
		EntityCode:         entity,
		PendingTransferIds: held.TransferIds,
	})))
	if err != nil || post.Msg.Status != ledgerv1.Status_STATUS_OK {
		t.Fatalf("post: %v %v", err, post)
	}
	if got := f.available(t, f.customer); got != "70000" {
		t.Fatalf("after post available=%s", got)
	}

	hold2 := f.transferReq(f.nextKey("hold2"), leg(f.customer, f.operate, "ZAR", "10000", 11))
	hold2.Pending = true
	hold2.PendingTimeoutSeconds = 600
	held2, _ := f.send(t, hold2)
	void, err := f.svc.VoidPending(context.Background(), withCaller(connect.NewRequest(&ledgerv1.VoidPendingRequest{
		IdempotencyKey:     f.nextKey("void"),
		BusinessTxnId:      f.nextTxn(),
		EntityCode:         entity,
		PendingTransferIds: held2.TransferIds,
	})))
	if err != nil || void.Msg.Status != ledgerv1.Status_STATUS_OK {
		t.Fatalf("void: %v %v", err, void)
	}
	if got := f.available(t, f.customer); got != "70000" {
		t.Fatalf("after void available=%s", got)
	}

	again, _ := f.svc.PostPending(context.Background(), withCaller(connect.NewRequest(&ledgerv1.PostPendingRequest{
		IdempotencyKey:     f.nextKey("post-again"),
		BusinessTxnId:      f.nextTxn(),
		EntityCode:         entity,
		PendingTransferIds: held.TransferIds,
	})))
	if again.Msg.Status != ledgerv1.Status_STATUS_REJECTED {
		t.Fatalf("second post of same hold must be rejected, got %v", again.Msg.Status)
	}
}

func linkedChainAtomic(t *testing.T, newEnv Factory) {
	f := setup(t, newEnv)
	if out, _ := f.send(t, f.transferReq(f.nextKey("fund"), leg(f.operate, f.customer, "ZAR", "5000", 10))); out.Status != ledgerv1.Status_STATUS_OK {
		t.Fatalf("fund: %v", out)
	}
	r := f.transferReq(f.nextKey("chain"),
		leg(f.operate, f.customer, "ZAR", "1000", 10),
		leg(f.customer, f.operate, "ZAR", "999999999", 11), // insufficient funds
	)
	res, err := f.send(t, r)
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	if res.Status != ledgerv1.Status_STATUS_REJECTED || res.RejectionCodes[0] != gateway.CodeInsufficientFunds {
		t.Fatalf("chain: %v %v", res.Status, res.RejectionCodes)
	}
	if got := f.available(t, f.customer); got != "5000" {
		t.Fatalf("partial chain applied: available=%s", got)
	}
}

// flakyStore applies a batch to the real store and then reports an error, as if the
// response was lost after TigerBeetle committed it. Used to test crash recovery.
type flakyStore struct {
	ledger.Store
	failNext bool
}

func (s *flakyStore) CreateTransfers(ctx context.Context, batch []ledger.Transfer) ([]ledger.Outcome, error) {
	out, err := s.Store.CreateTransfers(ctx, batch)
	if s.failNext {
		s.failNext = false
		return nil, errors.New("connection reset after commit")
	}
	return out, err
}

func crashRecovered(t *testing.T, newEnv Factory) {
	f := setup(t, newEnv)
	flaky := &flakyStore{Store: f.env.Store, failNext: true}
	svc := gateway.NewService(gateway.DefaultConfig(), flaky, f.env.Repo)
	key := f.nextKey("crash")
	r := f.transferReq(key, leg(f.operate, f.customer, "ZAR", "42000", 10))

	_, err := svc.CreateTransfers(context.Background(), withCaller(connect.NewRequest(r)))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("first attempt: want Unavailable, got %v", err)
	}
	if got := f.available(t, f.customer); got != "42000" {
		t.Fatalf("ledger should have the committed deposit, got %s", got)
	}

	res, err := svc.CreateTransfers(context.Background(), withCaller(connect.NewRequest(r)))
	if err != nil || res.Msg.Status != ledgerv1.Status_STATUS_OK {
		t.Fatalf("retry: err=%v resp=%v", err, res)
	}
	if got := f.available(t, f.customer); got != "42000" {
		t.Fatalf("retry moved money again: available=%s", got)
	}
	again, err := svc.CreateTransfers(context.Background(), withCaller(connect.NewRequest(r)))
	if err != nil || again.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		t.Fatalf("third call should replay: err=%v status=%v", err, again.Msg.Status)
	}
}

func validation(t *testing.T, newEnv Factory) {
	f := setup(t, newEnv)
	bad := f.transferReq(f.nextKey("v"), leg(f.operate, f.customer, "ZAR", "100", 10))
	bad.EntityCode = "MFI:acme"
	if _, err := f.send(t, bad); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("entity mismatch: want InvalidArgument, got %v", err)
	}

	noKey := f.transferReq("", leg(f.operate, f.customer, "ZAR", "100", 10))
	if _, err := f.send(t, noKey); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("missing key: want InvalidArgument, got %v", err)
	}

	eur := f.transferReq(f.nextKey("eur"), leg(f.operate, f.customer, "EUR", "100", 10))
	if res, _ := f.send(t, eur); res.Status != ledgerv1.Status_STATUS_REJECTED || res.RejectionCodes[0] != gateway.CodeAssetNotAllowed {
		t.Fatalf("EUR: %v %v", res.Status, res.RejectionCodes)
	}

	badCode := f.transferReq(f.nextKey("code"), leg(f.operate, f.customer, "ZAR", "100", 40))
	if res, _ := f.send(t, badCode); res.RejectionCodes[0] != gateway.CodeCodeNotAllowed {
		t.Fatalf("card code 40 must be disabled: %v", res.RejectionCodes)
	}

	zero := f.transferReq(f.nextKey("zero"), leg(f.operate, f.customer, "ZAR", "0", 10))
	if res, _ := f.send(t, zero); res.RejectionCodes[0] != gateway.CodeInvalidAmount {
		t.Fatalf("zero amount: %v", res.RejectionCodes)
	}

	neg := f.transferReq(f.nextKey("neg"), leg(f.operate, f.customer, "ZAR", "-5", 10))
	if _, err := f.send(t, neg); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("negative amount: want InvalidArgument, got %v", err)
	}

	big := f.transferReq(f.nextKey("big"), leg(f.operate, f.customer, "ZAR", "1000000001", 10))
	if res, _ := f.send(t, big); res.RejectionCodes[0] != gateway.CodeAmountLimitExceeded {
		t.Fatalf("limit: %v", res.RejectionCodes)
	}

	self := f.transferReq(f.nextKey("self"), leg(f.customer, f.customer, "ZAR", "1", 10))
	if res, _ := f.send(t, self); res.RejectionCodes[0] != gateway.CodeSameAccount {
		t.Fatalf("same account: %v", res.RejectionCodes)
	}

	unknown := f.transferReq(f.nextKey("unknown"), leg(f.operate, "00000000000000000000000000000099", "ZAR", "1", 10))
	if res, _ := f.send(t, unknown); res.RejectionCodes[0] != gateway.CodeAccountNotFound {
		t.Fatalf("unknown account: %v", res.RejectionCodes)
	}

	pendingNoTimeout := f.transferReq(f.nextKey("pnt"), leg(f.customer, f.operate, "ZAR", "1", 11))
	pendingNoTimeout.Pending = true
	if res, _ := f.send(t, pendingNoTimeout); res.RejectionCodes[0] != gateway.CodeTimeoutRequired {
		t.Fatalf("pending without timeout: %v", res.RejectionCodes)
	}

	tooLong := f.transferReq(f.nextKey("ptl"), leg(f.customer, f.operate, "ZAR", "1", 11))
	tooLong.Pending = true
	tooLong.PendingTimeoutSeconds = 8 * 24 * 60 * 60
	if res, _ := f.send(t, tooLong); res.RejectionCodes[0] != gateway.CodeTimeoutLimit {
		t.Fatalf("timeout over limit: %v", res.RejectionCodes)
	}

	badTxn := f.transferReq(f.nextKey("txn"), leg(f.operate, f.customer, "ZAR", "1", 10))
	badTxn.BusinessTxnId = "not-a-uuid"
	if _, err := f.send(t, badTxn); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad business_txn_id: want InvalidArgument, got %v", err)
	}
}

func outboxOnce(t *testing.T, newEnv Factory) {
	f := setup(t, newEnv)
	legs := 0
	for i := 0; i < 3; i++ {
		res, err := f.send(t, f.transferReq(f.nextKey("ob"), leg(f.operate, f.customer, "ZAR", "100", 10)))
		if err != nil || res.Status != ledgerv1.Status_STATUS_OK {
			t.Fatalf("deposit %d: %v %v", i, err, res)
		}
		legs++
	}
	// A replay must not add events.
	r := f.transferReq(f.nextKey("ob-replay"), leg(f.operate, f.customer, "ZAR", "100", 10))
	f.send(t, r)
	f.send(t, r)
	legs++

	var got []gateway.OutboxEvent
	total := 0
	for {
		n, err := f.env.Repo.RelayBatch(context.Background(), 2, func(evs []gateway.OutboxEvent) error {
			got = append(got, evs...)
			return nil
		})
		if err != nil {
			t.Fatalf("relay: %v", err)
		}
		if n == 0 {
			break
		}
		total += n
	}
	if total != legs {
		t.Fatalf("published %d events, want %d", total, legs)
	}
	seen := map[string]bool{}
	for _, e := range got {
		if e.Topic != gateway.TopicTransferCommitted {
			t.Fatalf("topic = %s", e.Topic)
		}
		var env eventsv1.EventEnvelope
		if err := proto.Unmarshal(e.Payload, &env); err != nil {
			t.Fatalf("envelope: %v", err)
		}
		if seen[env.EventId] {
			t.Fatalf("duplicate event %s", env.EventId)
		}
		seen[env.EventId] = true
		var tc eventsv1.TransferCommitted
		if err := proto.Unmarshal(env.Payload, &tc); err != nil {
			t.Fatalf("payload: %v", err)
		}
		if tc.Asset != "ZAR" || tc.AmountMinor != "100" {
			t.Fatalf("payload = %+v", &tc)
		}
	}
}
