package ledger

import "math/big"

// Reason explains why an event in a batch was rejected or why it succeeded as a
// replay. Names follow TigerBeetle's result codes where one exists.
type Reason string

const (
	ReasonOK                        Reason = ""
	ReasonExists                    Reason = "exists" // same ID, identical fields: idempotent success
	ReasonExistsWithDifferentFields Reason = "exists_with_different_fields"
	ReasonLinkedEventFailed         Reason = "linked_event_failed"
	ReasonAccountNotFound           Reason = "account_not_found"
	ReasonSameAccount               Reason = "same_account"
	ReasonLedgerMismatch            Reason = "ledger_mismatch"
	ReasonInsufficientFunds         Reason = "insufficient_funds"
	ReasonOverflow                  Reason = "overflow"
	ReasonPendingNotFound           Reason = "pending_not_found"
	ReasonPendingAlreadyPosted      Reason = "pending_already_posted"
	ReasonPendingAlreadyVoided      Reason = "pending_already_voided"
	ReasonPendingExpired            Reason = "pending_expired"
	ReasonPendingMismatch           Reason = "pending_mismatch"
	ReasonInvalidAmount             Reason = "invalid_amount"
	ReasonInvalidAccount            Reason = "invalid_account"
	ReasonInvalidTransfer           Reason = "invalid_transfer"
)

// IsSuccess reports whether a reason means the event was applied or already applied.
func (r Reason) IsSuccess() bool { return r == ReasonOK || r == ReasonExists }

// Account is a ledger account. Balances are in minor units of the account's asset.
type Account struct {
	ID                         ID
	Ledger                     uint32
	Code                       uint16
	DebitsMustNotExceedCredits bool
	UserData128                [16]byte
	UserData64                 uint64
	UserData32                 uint32
	Linked                     bool

	DebitsPending  uint64
	DebitsPosted   uint64
	CreditsPending uint64
	CreditsPosted  uint64
	Timestamp      uint64
}

// Available returns credits_posted - debits_posted - debits_pending. It can be
// negative only on accounts without debits_must_not_exceed_credits.
func (a Account) Available() *big.Int {
	v := new(big.Int).SetUint64(a.CreditsPosted)
	v.Sub(v, new(big.Int).SetUint64(a.DebitsPosted))
	v.Sub(v, new(big.Int).SetUint64(a.DebitsPending))
	return v
}

// Transfer moves Amount from DebitAccountID to CreditAccountID on one ledger.
// A post or void references a pending transfer through PendingID.
type Transfer struct {
	ID              ID
	DebitAccountID  ID
	CreditAccountID ID
	Amount          uint64
	PendingID       ID
	UserData128     [16]byte
	UserData64      uint64
	UserData32      uint32
	Timeout         uint32 // seconds; pending only
	Ledger          uint32
	Code            uint16

	Linked      bool
	Pending     bool
	PostPending bool
	VoidPending bool
	Timestamp   uint64
}

// Outcome is the per-event result of a batch, in the order submitted.
type Outcome struct {
	Index  int
	Reason Reason
}
