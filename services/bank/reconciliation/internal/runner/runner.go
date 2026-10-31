// Package runner performs one reconciliation run for one asset: it reads the ledger, reads the bank, evaluates the
// checks and commits the result. It fails closed: a run that cannot be stabilised is recorded as inconclusive and
// changes no break, and a run that cannot read its inputs records nothing, so the gate ages out.
package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/ledgerview"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/store"
)

// InFlightSource reads the in-flight terms for an asset from the payments service.
type InFlightSource interface {
	InFlight(ctx context.Context, asset string) (recon.InFlight, error)
}

// Runner runs reconciliation for one or more assets.
type Runner struct {
	Ledger   ledgerview.Reader
	Bank     banklayer.BankLayer
	InFlight InFlightSource
	Store    *store.Store
	Now      func() time.Time
	Attempts int // how many times to try for a stable read; default 3
}

// Outcome is what one run concluded.
type Outcome struct {
	RunID      string
	Asset      string
	Outcome    string
	AllMatched bool
	Checks     []recon.Result
}

// Run reconciles one asset against the bank's statement for today (UTC).
func (r *Runner) Run(ctx context.Context, asset string) (Outcome, error) {
	rail, ok := recon.ByAsset(asset)
	if !ok {
		return Outcome{}, fmt.Errorf("runner: %s is not a monitored asset", asset)
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	attempts := r.Attempts
	if attempts <= 0 {
		attempts = 3
	}
	started := now().UTC()

	if r.InFlight == nil {
		return Outcome{}, errors.New("runner: no in-flight source is configured; refusing to run without it")
	}

	var (
		snap   ledgerview.Snapshot
		stmt   banklayer.Statement
		flight recon.InFlight
		stable bool
	)
	for i := 0; i < attempts && !stable; i++ {
		before, err := r.Ledger.Snapshot(ctx, rail.Ledger)
		if err != nil {
			return Outcome{}, err
		}
		s, err := r.statement(ctx, rail, now().UTC())
		if err != nil {
			return Outcome{}, err
		}
		// Read in-flight between the statement and the second snapshot: a change in either the books or the in-flight
		// money during the read shows up as a digest mismatch, so the run retries rather than mixing two moments.
		f, err := r.InFlight.InFlight(ctx, rail.Asset)
		if err != nil {
			return Outcome{}, err
		}
		after, err := r.Ledger.Snapshot(ctx, rail.Ledger)
		if err != nil {
			return Outcome{}, err
		}
		if before.Digest == after.Digest {
			snap, stmt, flight, stable = after, s, f, true
		}
	}

	finished := now().UTC()
	run := store.Run{Asset: asset, StartedAt: started, FinishedAt: finished}
	if !stable {
		run.Outcome = store.OutcomeInconclusive
		if err := r.Store.Commit(ctx, run); err != nil {
			return Outcome{}, err
		}
		return Outcome{Asset: asset, Outcome: store.OutcomeInconclusive}, nil
	}

	checks := recon.Evaluate(rail, snap.Totals, factsOf(stmt), flight)
	run.Outcome = store.OutcomeConclusive
	run.Checks = checks
	if err := r.Store.Commit(ctx, run); err != nil {
		return Outcome{}, err
	}
	out := Outcome{Asset: asset, Outcome: store.OutcomeConclusive, Checks: checks, AllMatched: recon.AllMatched(checks)}
	return out, nil
}

// statement reads today's statement from the bank that holds the asset: the sponsor for ZAR, the partner for USD.
func (r *Runner) statement(ctx context.Context, rail recon.Rail, day time.Time) (banklayer.Statement, error) {
	if rail.Source == "sponsor" {
		return r.Bank.StatementSource().Statement(ctx, rail.Asset, day)
	}
	for _, p := range r.Bank.ForeignCurrency() {
		if p.Asset() == rail.Asset {
			return p.Statement(ctx, day)
		}
	}
	return banklayer.Statement{}, errors.New("runner: no foreign-currency provider for " + rail.Asset)
}

func factsOf(s banklayer.Statement) recon.Statement {
	out := recon.Statement{Opening: s.OpeningMinor, Closing: s.ClosingMinor}
	for _, m := range s.Movements {
		switch m.Direction {
		case "IN":
			out.InboundSum += m.AmountMinor
		case "OUT":
			out.OutboundSum += m.AmountMinor
		}
	}
	return out
}
