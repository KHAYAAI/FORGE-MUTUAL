package ledger

import "context"

// Store is the ledger contract. Implementations must apply each batch atomically
// with respect to other batches: a linked chain either applies in full or not at all.
//
// Outcomes are returned one per submitted event, in order. A linked chain in which
// any member's answer is not ReasonOK is not applied at all: that member carries its
// own reason and every other member reports ReasonLinkedEventFailed (TigerBeetle
// behaviour). "Not OK" includes ReasonExists, so a chain that is sent a second time
// answers ReasonExists for its first member and ReasonLinkedEventFailed for the rest.
// A caller that retries must read that as "already applied" and confirm it with
// LookupTransfers.
//
// A transfer refused for a reason that depended on the ledger's state (Reason.Transient)
// has its ID refused for good: every later use answers ReasonIDAlreadyFailed.
//
// MemStore is the model of these rules and TigerBeetle the real thing. The differential
// test (tigerbeetle_diff_test.go) sends both the same random batches and requires the
// same outcomes, balances and stored transfers.
type Store interface {
	CreateAccounts(ctx context.Context, accounts []Account) ([]Outcome, error)
	CreateTransfers(ctx context.Context, transfers []Transfer) ([]Outcome, error)
	// LookupAccounts returns found accounts in input order; missing IDs are omitted.
	LookupAccounts(ctx context.Context, ids []ID) ([]Account, error)
	// LookupTransfers returns found transfers in input order; missing IDs are omitted.
	LookupTransfers(ctx context.Context, ids []ID) ([]Transfer, error)
}
