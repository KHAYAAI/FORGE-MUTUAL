package payments

import "context"

// ReconStatus reports whether a sponsor reconciliation break is open (ARCHITECTURE.md 13). Payments above the
// policy threshold are held while one is.
type ReconStatus interface {
	Breaking(ctx context.Context) (bool, error)
}

// StaticRecon is a fixed reconciliation status. It stands in for the reconciliation service (P1-08), which
// is not built yet. The service refuses to start without an explicit value: a missing status must never
// read as "no break".
type StaticRecon struct {
	Break bool
}

// Breaking implements ReconStatus.
func (s StaticRecon) Breaking(context.Context) (bool, error) { return s.Break, nil }
