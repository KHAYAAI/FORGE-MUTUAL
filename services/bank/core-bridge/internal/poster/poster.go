// Package poster posts queued journals to Fineract. One poster runs at a time (an advisory lock). A retry whose previous
// outcome is unknown first looks for the journal by its reference, so a write that succeeded before a timeout is
// recorded, not repeated.
package poster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/chart"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/fineract"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/store"
)

// lockKey identifies the poster lock ("CBP").
const lockKey = 0x434250

// Poster runs posting passes.
type Poster struct {
	Pool        *pgxpool.Pool
	Fin         *fineract.Client
	OfficeID    int64
	GL          map[string]int64 // Fineract GL id by GLCode
	MaxAttempts int              // default 5
	Batch       int              // default 50
	Log         *slog.Logger
}

// Pass posts one batch. It returns how many were posted and how many failed. If another poster holds the lock, it does
// nothing and reports zero.
func (p *Poster) Pass(ctx context.Context) (posted, failed int, err error) {
	max, batch := p.MaxAttempts, p.Batch
	if max <= 0 {
		max = 5
	}
	if batch <= 0 {
		batch = 50
	}
	conn, err := p.Pool.Acquire(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `select pg_try_advisory_lock($1)`, lockKey).Scan(&locked); err != nil {
		return 0, 0, err
	}
	if !locked {
		return 0, 0, nil
	}
	defer conn.Exec(context.Background(), `select pg_advisory_unlock($1)`, lockKey) //nolint:errcheck // released with the session

	jobs, err := store.Claim(ctx, p.Pool, batch, max)
	if err != nil {
		return 0, 0, err
	}
	for _, j := range jobs {
		debitGL, okD := p.GL[chart.GLCode(j.DebitCode, j.Asset)]
		creditGL, okC := p.GL[chart.GLCode(j.CreditCode, j.Asset)]
		if !okD || !okC {
			failed++
			_ = store.MarkFailed(ctx, p.Pool, j.TransferID, max, true, "no GL account for this code and asset")
			continue
		}
		ref := j.TransferID
		if j.Attempts > 1 {
			// An earlier attempt may have reached Fineract. Look before writing again.
			txn, found, err := p.Fin.FindByReference(ctx, debitGL, ref)
			if err != nil {
				failed++
				_ = store.MarkFailed(ctx, p.Pool, j.TransferID, max, false, "lookup before retry: "+err.Error())
				continue
			}
			if found {
				if err := store.MarkPosted(ctx, p.Pool, j.TransferID, txn); err != nil {
					return posted, failed, err
				}
				p.log("recorded an earlier posting instead of repeating it", j.TransferID)
				posted++
				continue
			}
		}
		txn, err := p.Fin.PostJournal(ctx, fineract.Journal{
			OfficeID: p.OfficeID, Currency: j.Asset, Date: j.OccurredAt, Reference: ref,
			Comments: "ledger transfer " + j.TransferID,
			Debits:   []fineract.Line{{GLAccountID: debitGL, AmountMinor: j.Amount}},
			Credits:  []fineract.Line{{GLAccountID: creditGL, AmountMinor: j.Amount}},
		})
		if err != nil {
			failed++
			definitive := !fineract.Uncertain(err) && isDefinitive(err)
			if mErr := store.MarkFailed(ctx, p.Pool, j.TransferID, max, definitive, err.Error()); mErr != nil {
				return posted, failed, mErr
			}
			continue
		}
		if err := store.MarkPosted(ctx, p.Pool, j.TransferID, txn); err != nil {
			return posted, failed, fmt.Errorf("poster: record posting: %w", err)
		}
		posted++
	}
	return posted, failed, nil
}

func isDefinitive(err error) bool {
	var api *fineract.APIError
	return errors.As(err, &api) && api.Definitive()
}

func (p *Poster) log(msg, id string) {
	if p.Log != nil {
		p.Log.Info(msg, "transfer", id)
	}
}
