package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	tb "github.com/tigerbeetle/tigerbeetle-go"
	tbtypes "github.com/tigerbeetle/tigerbeetle-go/pkg/types"
)

// TigerBeetle is a Store backed by a TigerBeetle cluster through the official Go client.
// Amounts are uint64 minor units, so every balance is read back as a value that fits in
// 64 bits; a higher half that is not zero is reported as an error rather than truncated.
//
// Verified against TigerBeetle 0.16.60 with client v0.16.60 (SPEC.md section 8): the gateway suite, a differential
// test against MemStore, and a three-replica cluster that lost its primary, lost a second replica, and lost quorum.
// The client must never be newer than the cluster: upgrade the cluster first.
type TigerBeetle struct {
	client tb.Client
	// maxWait is how long one call waits for the cluster. slots caps how many calls may be waiting at once.
	maxWait time.Duration
	slots   chan struct{}
}

// ErrUnavailable is returned when the cluster does not answer in time. It does not mean the request was not applied.
// The client never gives up: without a quorum it keeps the request and delivers it when the cluster is back. So the
// outcome is unknown, and the caller must ask again with the same IDs. A transfer that was applied after all then
// answers "exists", and one that was not is applied once. The gateway's idempotency records do exactly this.
var ErrUnavailable = errors.New("tigerbeetle: the cluster did not answer in time; the outcome is unknown, retry with the same ids")

const (
	// defaultMaxWait is well above a view change (about five seconds when a primary dies, measured) and well below
	// the gateway's 30-second write timeout.
	defaultMaxWait = 12 * time.Second
	// defaultMaxWaiting bounds the calls (and their goroutines) parked while the cluster is away. Beyond it, calls
	// are refused at once rather than queued.
	defaultMaxWaiting = 512
)

// NewTigerBeetle connects to a cluster. addresses are host:port pairs. It returns before the cluster has answered:
// use Ping to find out whether it does.
func NewTigerBeetle(clusterID uint64, addresses []string) (*TigerBeetle, error) {
	c, err := tb.NewClient(tbtypes.ToUint128(clusterID), addresses)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle: connect: %w", err)
	}
	return &TigerBeetle{client: c, maxWait: defaultMaxWait, slots: make(chan struct{}, defaultMaxWaiting)}, nil
}

// Close releases the client. Calls still waiting for the cluster end with an error.
func (t *TigerBeetle) Close() { t.client.Close() }

// Ping reports whether the cluster answers. A lookup goes through consensus like any other request, so an answer
// means a quorum of replicas is up.
func (t *TigerBeetle) Ping(ctx context.Context) error {
	_, err := await(ctx, t, "ping", func() ([]tbtypes.Account, error) {
		return t.client.LookupAccounts([]tbtypes.Uint128{tbtypes.ToUint128(1)})
	})
	return err
}

// await runs one client call and stops waiting for it when ctx ends or maxWait passes, whichever is first. The call
// itself cannot be cancelled: see ErrUnavailable for what that means for the caller.
func await[T any](ctx context.Context, t *TigerBeetle, what string, call func() (T, error)) (T, error) {
	var zero T
	select {
	case t.slots <- struct{}{}:
	default:
		return zero, fmt.Errorf("%w (%s: %d calls are already waiting)", ErrUnavailable, what, cap(t.slots))
	}
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		defer func() { <-t.slots }()
		v, err := call()
		done <- result{v, err}
	}()
	timer := time.NewTimer(t.maxWait)
	defer timer.Stop()
	select {
	case r := <-done:
		if r.err != nil {
			return zero, fmt.Errorf("tigerbeetle: %s: %w", what, r.err)
		}
		return r.v, nil
	case <-ctx.Done():
		return zero, fmt.Errorf("%w (%s: %v)", ErrUnavailable, what, ctx.Err())
	case <-timer.C:
		return zero, fmt.Errorf("%w (%s: no answer in %s)", ErrUnavailable, what, t.maxWait)
	}
}

func toU128(id ID) tbtypes.Uint128 { return tbtypes.BytesToUint128(id) }

func fromU128(v tbtypes.Uint128) ID { return ID(v.Bytes()) }

func u64(v tbtypes.Uint128) (uint64, error) {
	b := v.Bytes()
	for _, x := range b[8:] {
		if x != 0 {
			return 0, errors.New("tigerbeetle: balance exceeds 64 bits")
		}
	}
	var out uint64
	for i := 7; i >= 0; i-- {
		out = out<<8 | uint64(b[i])
	}
	return out, nil
}

func toU128Amount(v uint64) tbtypes.Uint128 { return tbtypes.ToUint128(v) }

// CreateAccounts implements Store.
func (t *TigerBeetle) CreateAccounts(ctx context.Context, accounts []Account) ([]Outcome, error) {
	batch := make([]tbtypes.Account, len(accounts))
	for i, a := range accounts {
		flags := tbtypes.AccountFlags{Linked: a.Linked, DebitsMustNotExceedCredits: a.DebitsMustNotExceedCredits}
		batch[i] = tbtypes.Account{
			ID:          toU128(a.ID),
			UserData128: tbtypes.BytesToUint128(a.UserData128),
			UserData64:  a.UserData64,
			UserData32:  a.UserData32,
			Ledger:      a.Ledger,
			Code:        a.Code,
			Flags:       flags.ToUint16(),
		}
	}
	res, err := await(ctx, t, "create accounts", func() ([]tbtypes.AccountEventResult, error) { return t.client.CreateAccounts(batch) })
	if err != nil {
		return nil, err
	}
	out := make([]Outcome, len(accounts))
	for i := range out {
		out[i] = Outcome{Index: i, Reason: ReasonOK}
	}
	for _, r := range res {
		out[r.Index] = Outcome{Index: int(r.Index), Reason: mapAccountResult(r.Result)}
	}
	return out, nil
}

// CreateTransfers implements Store.
func (t *TigerBeetle) CreateTransfers(ctx context.Context, transfers []Transfer) ([]Outcome, error) {
	batch := make([]tbtypes.Transfer, len(transfers))
	for i, tr := range transfers {
		flags := tbtypes.TransferFlags{
			Linked:              tr.Linked,
			Pending:             tr.Pending,
			PostPendingTransfer: tr.PostPending,
			VoidPendingTransfer: tr.VoidPending,
		}
		batch[i] = tbtypes.Transfer{
			ID:              toU128(tr.ID),
			DebitAccountID:  toU128(tr.DebitAccountID),
			CreditAccountID: toU128(tr.CreditAccountID),
			Amount:          toU128Amount(tr.Amount),
			PendingID:       toU128(tr.PendingID),
			UserData128:     tbtypes.BytesToUint128(tr.UserData128),
			UserData64:      tr.UserData64,
			UserData32:      tr.UserData32,
			Timeout:         tr.Timeout,
			Ledger:          tr.Ledger,
			Code:            tr.Code,
			Flags:           flags.ToUint16(),
		}
	}
	res, err := await(ctx, t, "create transfers", func() ([]tbtypes.TransferEventResult, error) { return t.client.CreateTransfers(batch) })
	if err != nil {
		return nil, err
	}
	out := make([]Outcome, len(transfers))
	for i := range out {
		out[i] = Outcome{Index: i, Reason: ReasonOK}
	}
	for _, r := range res {
		out[r.Index] = Outcome{Index: int(r.Index), Reason: mapTransferResult(r.Result)}
	}
	return out, nil
}

// LookupAccounts implements Store.
func (t *TigerBeetle) LookupAccounts(ctx context.Context, ids []ID) ([]Account, error) {
	keys := make([]tbtypes.Uint128, len(ids))
	for i, id := range ids {
		keys[i] = toU128(id)
	}
	accs, err := await(ctx, t, "lookup accounts", func() ([]tbtypes.Account, error) { return t.client.LookupAccounts(keys) })
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(accs))
	for _, a := range accs {
		converted, err := fromTBAccount(a)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

// LookupTransfers implements Store.
func (t *TigerBeetle) LookupTransfers(ctx context.Context, ids []ID) ([]Transfer, error) {
	keys := make([]tbtypes.Uint128, len(ids))
	for i, id := range ids {
		keys[i] = toU128(id)
	}
	trs, err := await(ctx, t, "lookup transfers", func() ([]tbtypes.Transfer, error) { return t.client.LookupTransfers(keys) })
	if err != nil {
		return nil, err
	}
	out := make([]Transfer, 0, len(trs))
	for _, tr := range trs {
		amt, err := u64(tr.Amount)
		if err != nil {
			return nil, err
		}
		f := tr.TransferFlags()
		out = append(out, Transfer{
			ID:              fromU128(tr.ID),
			DebitAccountID:  fromU128(tr.DebitAccountID),
			CreditAccountID: fromU128(tr.CreditAccountID),
			Amount:          amt,
			PendingID:       fromU128(tr.PendingID),
			UserData128:     tr.UserData128.Bytes(),
			UserData64:      tr.UserData64,
			UserData32:      tr.UserData32,
			Timeout:         tr.Timeout,
			Ledger:          tr.Ledger,
			Code:            tr.Code,
			Linked:          f.Linked,
			Pending:         f.Pending,
			PostPending:     f.PostPendingTransfer,
			VoidPending:     f.VoidPendingTransfer,
			Timestamp:       tr.Timestamp,
		})
	}
	return out, nil
}

func fromTBAccount(a tbtypes.Account) (Account, error) {
	var err error
	out := Account{
		ID:                         fromU128(a.ID),
		Ledger:                     a.Ledger,
		Code:                       a.Code,
		DebitsMustNotExceedCredits: a.AccountFlags().DebitsMustNotExceedCredits,
		UserData128:                a.UserData128.Bytes(),
		UserData64:                 a.UserData64,
		UserData32:                 a.UserData32,
		Timestamp:                  a.Timestamp,
	}
	fields := []struct {
		src tbtypes.Uint128
		dst *uint64
	}{
		{a.DebitsPending, &out.DebitsPending},
		{a.DebitsPosted, &out.DebitsPosted},
		{a.CreditsPending, &out.CreditsPending},
		{a.CreditsPosted, &out.CreditsPosted},
	}
	for _, f := range fields {
		if *f.dst, err = u64(f.src); err != nil {
			return Account{}, err
		}
	}
	return out, nil
}

// mapAccountResult maps TigerBeetle account results to Reason.
func mapAccountResult(r tbtypes.CreateAccountResult) Reason {
	switch r {
	case tbtypes.AccountOK:
		return ReasonOK
	case tbtypes.AccountExists:
		return ReasonExists
	case tbtypes.AccountExistsWithDifferentFlags, tbtypes.AccountExistsWithDifferentLedger,
		tbtypes.AccountExistsWithDifferentCode, tbtypes.AccountExistsWithDifferentUserData128,
		tbtypes.AccountExistsWithDifferentUserData64, tbtypes.AccountExistsWithDifferentUserData32:
		return ReasonExistsWithDifferentFields
	case tbtypes.AccountLinkedEventFailed:
		return ReasonLinkedEventFailed
	default:
		return ReasonInvalidAccount
	}
}

// mapTransferResult maps TigerBeetle transfer results to Reason.
func mapTransferResult(r tbtypes.CreateTransferResult) Reason {
	switch r {
	case tbtypes.TransferOK:
		return ReasonOK
	case tbtypes.TransferExists:
		return ReasonExists
	case tbtypes.TransferExistsWithDifferentFlags, tbtypes.TransferExistsWithDifferentPendingID,
		tbtypes.TransferExistsWithDifferentTimeout, tbtypes.TransferExistsWithDifferentDebitAccountID,
		tbtypes.TransferExistsWithDifferentCreditAccountID, tbtypes.TransferExistsWithDifferentAmount,
		tbtypes.TransferExistsWithDifferentLedger, tbtypes.TransferExistsWithDifferentCode,
		tbtypes.TransferExistsWithDifferentUserData128, tbtypes.TransferExistsWithDifferentUserData64,
		tbtypes.TransferExistsWithDifferentUserData32:
		return ReasonExistsWithDifferentFields
	case tbtypes.TransferIDAlreadyFailed:
		return ReasonIDAlreadyFailed
	case tbtypes.TransferLinkedEventFailed:
		return ReasonLinkedEventFailed
	case tbtypes.TransferDebitAccountNotFound, tbtypes.TransferCreditAccountNotFound:
		return ReasonAccountNotFound
	case tbtypes.TransferAccountsMustBeDifferent:
		return ReasonSameAccount
	case tbtypes.TransferAccountsMustHaveTheSameLedger, tbtypes.TransferTransferMustHaveTheSameLedgerAsAccounts,
		tbtypes.TransferPendingTransferHasDifferentLedger:
		return ReasonLedgerMismatch
	case tbtypes.TransferExceedsCredits:
		return ReasonInsufficientFunds
	case tbtypes.TransferOverflowsDebitsPending, tbtypes.TransferOverflowsCreditsPending,
		tbtypes.TransferOverflowsDebitsPosted, tbtypes.TransferOverflowsCreditsPosted,
		tbtypes.TransferOverflowsDebits, tbtypes.TransferOverflowsCredits, tbtypes.TransferOverflowsTimeout:
		return ReasonOverflow
	case tbtypes.TransferPendingTransferNotFound:
		return ReasonPendingNotFound
	case tbtypes.TransferPendingTransferNotPending:
		return ReasonPendingNotPending
	case tbtypes.TransferPendingTransferAlreadyPosted:
		return ReasonPendingAlreadyPosted
	case tbtypes.TransferPendingTransferAlreadyVoided:
		return ReasonPendingAlreadyVoided
	case tbtypes.TransferPendingTransferExpired:
		return ReasonPendingExpired
	case tbtypes.TransferPendingTransferHasDifferentDebitAccountID, tbtypes.TransferPendingTransferHasDifferentCreditAccountID,
		tbtypes.TransferPendingTransferHasDifferentCode, tbtypes.TransferPendingTransferHasDifferentAmount,
		tbtypes.TransferExceedsPendingTransferAmount:
		return ReasonPendingMismatch
	default:
		return ReasonInvalidTransfer
	}
}
