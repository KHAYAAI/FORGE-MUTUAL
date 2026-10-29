// Package memrepo is an in-process gateway.Repository for tests and local development.
// It is not durable. The Postgres repository is the reference for production.
package memrepo

import (
	"bytes"
	"context"
	"sync"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// Repo implements gateway.Repository in memory.
type Repo struct {
	mu      sync.Mutex
	records map[string]*gateway.Record
	dir     map[ledger.ID]gateway.DirectoryEntry
	outbox  []outboxRow
}

type outboxRow struct {
	ev        gateway.OutboxEvent
	published bool
}

var _ gateway.Repository = (*Repo)(nil)

// New returns an empty repository.
func New() *Repo {
	return &Repo{records: make(map[string]*gateway.Record), dir: make(map[ledger.ID]gateway.DirectoryEntry)}
}

func recKey(caller, key string) string { return caller + "\x00" + key }

// Begin implements gateway.Repository.
func (r *Repo) Begin(_ context.Context, caller, key string, hash []byte, proposed []ledger.ID) (gateway.Record, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := recKey(caller, key)
	if existing, ok := r.records[k]; ok {
		if !bytes.Equal(existing.Hash, hash) {
			return gateway.Record{}, false, gateway.ErrConflict
		}
		return *existing, false, nil
	}
	rec := &gateway.Record{Status: gateway.StatusPending, Hash: append([]byte(nil), hash...), IDs: append([]ledger.ID(nil), proposed...)}
	r.records[k] = rec
	return *rec, true, nil
}

// Complete implements gateway.Repository.
func (r *Repo) Complete(_ context.Context, caller, key string, result []byte, dir []gateway.DirectoryEntry, events []gateway.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[recKey(caller, key)]
	if !ok {
		return ErrNoRecord
	}
	if rec.Status == gateway.StatusCommitted {
		return nil
	}
	rec.Status = gateway.StatusCommitted
	rec.Result = append([]byte(nil), result...)
	for _, d := range dir {
		if _, exists := r.dir[d.AccountID]; !exists {
			r.dir[d.AccountID] = d
		}
	}
	for _, e := range events {
		r.outbox = append(r.outbox, outboxRow{ev: e})
	}
	return nil
}

// Directory implements gateway.Repository.
func (r *Repo) Directory(_ context.Context, ids []ledger.ID) (map[ledger.ID]gateway.DirectoryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[ledger.ID]gateway.DirectoryEntry)
	for _, id := range ids {
		if d, ok := r.dir[id]; ok {
			out[id] = d
		}
	}
	return out, nil
}

// RelayBatch implements gateway.Repository.
func (r *Repo) RelayBatch(_ context.Context, limit int, publish func([]gateway.OutboxEvent) error) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var batch []int
	var events []gateway.OutboxEvent
	for i := range r.outbox {
		if r.outbox[i].published {
			continue
		}
		batch = append(batch, i)
		events = append(events, r.outbox[i].ev)
		if len(batch) == limit {
			break
		}
	}
	if len(events) == 0 {
		return 0, nil
	}
	if err := publish(events); err != nil {
		return 0, err
	}
	for _, i := range batch {
		r.outbox[i].published = true
	}
	return len(events), nil
}

// ErrNoRecord is returned when completing a record that was never begun.
var ErrNoRecord = errNoRecord{}

type errNoRecord struct{}

func (errNoRecord) Error() string { return "memrepo: no idempotency record" }
