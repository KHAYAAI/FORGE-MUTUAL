package ledger

import "context"

// Store is the ledger contract. Implementations must apply each batch atomically
// with respect to other batches: a linked chain either applies in full or not at all.
//
// Outcomes are returned one per submitted event, in order. A linked chain whose
// members are not all successful reports ReasonLinkedEventFailed on every member
// except the one that failed, which carries its own reason (TigerBeetle behaviour).
type Store interface {
	CreateAccounts(ctx context.Context, accounts []Account) ([]Outcome, error)
	CreateTransfers(ctx context.Context, transfers []Transfer) ([]Outcome, error)
	// LookupAccounts returns found accounts in input order; missing IDs are omitted.
	LookupAccounts(ctx context.Context, ids []ID) ([]Account, error)
	// LookupTransfers returns found transfers in input order; missing IDs are omitted.
	LookupTransfers(ctx context.Context, ids []ID) ([]Transfer, error)
}
