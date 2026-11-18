// Package ledgerfake is an in-memory ledger for tests. It implements the parts of the gateway the FX saga uses, with the
// behaviour that matters for the saga: pending transfers, posting, voiding, and idempotent replay by key. It can be told
// to fail an operation once, to exercise recovery.
package ledgerfake

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

type acct struct {
	id      string
	asset   string
	code    uint32
	debits  int64
	credits int64
	pendDeb int64 // pending debits held against this account
}

type pending struct {
	id     string
	debit  string
	credit string
	asset  string
	amount int64
	key    string
}

// Fake is the ledger.
type Fake struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	mu       sync.Mutex
	seq      int
	accounts map[string]*acct
	byKey    map[string]string   // account key to id
	txnKeys  map[string][]string // transfer key to transfer ids
	pendings map[string]*pending
	posted   map[string]bool
	// Fail is consulted before an operation: when it returns an error, the operation is not performed.
	Fail func(op, key string) error
}

// New returns an empty ledger.
func New() *Fake {
	return &Fake{accounts: map[string]*acct{}, byKey: map[string]string{}, txnKeys: map[string][]string{},
		pendings: map[string]*pending{}, posted: map[string]bool{}}
}

// Balance returns an account's posted balance: credits minus debits.
func (f *Fake) Balance(id string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.accounts[id]
	if a == nil {
		return 0
	}
	return a.credits - a.debits
}

// PendingDebit returns the pending debits held against an account.
func (f *Fake) PendingDebit(id string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a := f.accounts[id]; a != nil {
		return a.pendDeb
	}
	return 0
}

// Accounts returns the account id created under a key.
func (f *Fake) Account(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byKey[key]
}

// Seed credits an account directly, as a deposit would, for tests.
func (f *Fake) Seed(id string, amount int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accounts[id].credits += amount
}

// failErr keeps a Connect code the test set, so an injected refusal stays a refusal; anything else is unavailable.
func failErr(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce
	}
	return connect.NewError(connect.CodeUnavailable, err)
}

func (f *Fake) fail(op, key string) error {
	if f.Fail != nil {
		return f.Fail(op, key)
	}
	return nil
}

func (f *Fake) CreateAccounts(_ context.Context, r *connect.Request[ledgerv1.CreateAccountsRequest]) (*connect.Response[ledgerv1.CreateAccountsResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("CreateAccounts", r.Msg.IdempotencyKey); err != nil {
		return nil, failErr(err)
	}
	if id, ok := f.byKey[r.Msg.IdempotencyKey]; ok {
		return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_REPLAYED, AccountIds: []string{id}}), nil
	}
	f.seq++
	id := fmt.Sprintf("acc-%04d", f.seq)
	spec := r.Msg.Accounts[0]
	f.accounts[id] = &acct{id: id, asset: spec.Asset, code: spec.Code}
	f.byKey[r.Msg.IdempotencyKey] = id
	return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_OK, AccountIds: []string{id}}), nil
}

// CreateTransfers applies legs as one chain. A pending chain holds value until it is posted or voided.
func (f *Fake) CreateTransfers(_ context.Context, r *connect.Request[ledgerv1.CreateTransfersRequest]) (*connect.Response[ledgerv1.CreateTransfersResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := r.Msg.IdempotencyKey
	if err := f.fail("CreateTransfers", key); err != nil {
		return nil, failErr(err)
	}
	if ids, ok := f.txnKeys[key]; ok {
		return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REPLAYED, TransferIds: ids}), nil
	}
	// Check the whole chain before applying any of it: a linked chain is all or nothing.
	need := map[string]int64{}
	for _, leg := range r.Msg.Legs {
		amt, _ := strconv.ParseInt(leg.Amount.AmountMinor, 10, 64)
		if amt <= 0 {
			return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REJECTED}), nil
		}
		need[leg.DebitAccountId] += amt
	}
	for id, n := range need {
		a := f.accounts[id]
		if a == nil || (a.code == 1001 || a.code == 2003) && a.credits-a.debits-a.pendDeb < n {
			return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REJECTED,
				RejectionCodes: []string{"insufficient_available"}}), nil
		}
	}
	var ids []string
	for i, leg := range r.Msg.Legs {
		amt, _ := strconv.ParseInt(leg.Amount.AmountMinor, 10, 64)
		id := fmt.Sprintf("%s-%d", key, i)
		ids = append(ids, id)
		if r.Msg.Pending {
			f.pendings[id] = &pending{id: id, debit: leg.DebitAccountId, credit: leg.CreditAccountId, asset: leg.Amount.Asset, amount: amt, key: key}
			f.accounts[leg.DebitAccountId].pendDeb += amt
			continue
		}
		f.accounts[leg.DebitAccountId].debits += amt
		f.accounts[leg.CreditAccountId].credits += amt
	}
	f.txnKeys[key] = ids
	return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_OK, TransferIds: ids}), nil
}

// PostPending posts pending transfers. Posting is idempotent by key.
func (f *Fake) PostPending(_ context.Context, r *connect.Request[ledgerv1.PostPendingRequest]) (*connect.Response[ledgerv1.PostPendingResponse], error) {
	return f.settle(r.Msg.IdempotencyKey, r.Msg.PendingTransferIds, true)
}

// VoidPending releases pending transfers. Voiding is idempotent by key.
func (f *Fake) VoidPending(_ context.Context, r *connect.Request[ledgerv1.VoidPendingRequest]) (*connect.Response[ledgerv1.VoidPendingResponse], error) {
	res, err := f.settle(r.Msg.IdempotencyKey, r.Msg.PendingTransferIds, false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ledgerv1.VoidPendingResponse{Status: res.Msg.Status}), nil
}

func (f *Fake) settle(key string, ids []string, post bool) (*connect.Response[ledgerv1.PostPendingResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	op := "VoidPending"
	if post {
		op = "PostPending"
	}
	if err := f.fail(op, key); err != nil {
		return nil, failErr(err)
	}
	if f.posted[key] {
		return connect.NewResponse(&ledgerv1.PostPendingResponse{Status: ledgerv1.Status_STATUS_REPLAYED}), nil
	}
	// Check every id before applying any: a request either settles all of its transfers or none.
	for _, id := range ids {
		if _, ok := f.pendings[id]; !ok {
			return connect.NewResponse(&ledgerv1.PostPendingResponse{Status: ledgerv1.Status_STATUS_REJECTED,
				RejectionCodes: []string{"no_pending_transfer"}}), nil
		}
	}
	for _, id := range ids {
		p := f.pendings[id]
		f.accounts[p.debit].pendDeb -= p.amount
		if post {
			f.accounts[p.debit].debits += p.amount
			f.accounts[p.credit].credits += p.amount
		}
		delete(f.pendings, id)
	}
	f.posted[key] = true
	return connect.NewResponse(&ledgerv1.PostPendingResponse{Status: ledgerv1.Status_STATUS_OK}), nil
}
