package ledger

import (
	"context"
	"errors"
	"fmt"

	tb "github.com/tigerbeetle/tigerbeetle-go"
	tbtypes "github.com/tigerbeetle/tigerbeetle-go/pkg/types"
)

// TigerBeetle is a Store backed by a TigerBeetle cluster through the official Go client.
// Amounts are uint64 minor units, so every balance is read back as a value that fits in
// 64 bits; a higher half that is not zero is reported as an error rather than truncated.
//
// Status: compiled against tigerbeetle-go v0.16.60. Not yet run against a live cluster in
// this environment (VERIFY-TB; see SPEC.md section 8).
type TigerBeetle struct {
	client tb.Client
}

// NewTigerBeetle connects to a cluster. addresses are host:port pairs.
func NewTigerBeetle(clusterID uint64, addresses []string) (*TigerBeetle, error) {
	c, err := tb.NewClient(tbtypes.ToUint128(clusterID), addresses)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle: connect: %w", err)
	}
	return &TigerBeetle{client: c}, nil
}

// Close releases the client.
func (t *TigerBeetle) Close() { t.client.Close() }

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
func (t *TigerBeetle) CreateAccounts(_ context.Context, accounts []Account) ([]Outcome, error) {
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
	res, err := t.client.CreateAccounts(batch)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle: create accounts: %w", err)
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
func (t *TigerBeetle) CreateTransfers(_ context.Context, transfers []Transfer) ([]Outcome, error) {
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
	res, err := t.client.CreateTransfers(batch)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle: create transfers: %w", err)
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
func (t *TigerBeetle) LookupAccounts(_ context.Context, ids []ID) ([]Account, error) {
	keys := make([]tbtypes.Uint128, len(ids))
	for i, id := range ids {
		keys[i] = toU128(id)
	}
	accs, err := t.client.LookupAccounts(keys)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle: lookup accounts: %w", err)
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
func (t *TigerBeetle) LookupTransfers(_ context.Context, ids []ID) ([]Transfer, error) {
	keys := make([]tbtypes.Uint128, len(ids))
	for i, id := range ids {
		keys[i] = toU128(id)
	}
	trs, err := t.client.LookupTransfers(keys)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle: lookup transfers: %w", err)
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
		tbtypes.AccountExistsWithDifferentCode:
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
		tbtypes.TransferExistsWithDifferentLedger, tbtypes.TransferExistsWithDifferentCode:
		return ReasonExistsWithDifferentFields
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
