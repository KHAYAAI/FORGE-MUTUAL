// Package gateway implements the ledger gateway's RPC behaviour: validation,
// idempotency, ledger submission and the outbox. Storage sits behind Repository.
package gateway

import (
	"context"
	"errors"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// ErrConflict means an idempotency key was reused with a different request.
var ErrConflict = errors.New("idempotency key reused with a different request")

// RecordStatus is the state of an idempotency record.
type RecordStatus string

const (
	StatusPending   RecordStatus = "PENDING"   // written before the ledger call
	StatusCommitted RecordStatus = "COMMITTED" // result stored; replays return it
)

// Record is a stored idempotent request.
type Record struct {
	Status RecordStatus
	Hash   []byte
	// IDs are the ledger IDs generated for this request. They are stored before the
	// ledger call so a retry after a crash reuses them.
	IDs []ledger.ID
	// Result is the serialised response, returned on replay.
	Result []byte
}

// DirectoryEntry maps a ledger account to its business identity.
type DirectoryEntry struct {
	AccountID    ledger.ID
	Entity       string
	Asset        string
	Ledger       uint32
	Code         uint16
	OwnerPartyID string
	ProductID    string
	Insured      bool
}

// OutboxEvent is a committed event waiting for publication.
type OutboxEvent struct {
	EventID string
	Topic   string
	Key     []byte
	Payload []byte
}

// Repository holds idempotency records, the account directory and the outbox.
type Repository interface {
	// Begin returns the record for (caller, key). If none exists it inserts one as PENDING
	// holding proposed IDs, and the returned record has those IDs. A different request hash
	// for an existing key returns ErrConflict. created reports whether this call inserted it.
	Begin(ctx context.Context, caller, key string, hash []byte, proposed []ledger.ID) (rec Record, created bool, err error)

	// Complete marks the record COMMITTED with its result. In the same transaction it writes
	// directory entries (idempotently) and outbox events. Completing an already committed
	// record is a no-op.
	Complete(ctx context.Context, caller, key string, result []byte, dir []DirectoryEntry, events []OutboxEvent) error

	// Directory returns the entries for the given accounts. Missing accounts are omitted.
	Directory(ctx context.Context, ids []ledger.ID) (map[ledger.ID]DirectoryEntry, error)

	// RelayBatch hands up to limit unpublished events to publish. Events are marked published
	// only if publish returns nil, in one transaction. It returns how many were published.
	RelayBatch(ctx context.Context, limit int, publish func([]OutboxEvent) error) (int, error)
}
