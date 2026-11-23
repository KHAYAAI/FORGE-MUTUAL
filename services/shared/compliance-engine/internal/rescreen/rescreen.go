// Package rescreen screens every customer again whenever the sanctions list changes, and once a day. A match is
// handled as targeted financial sanctions require (FICA s.26A to s.26C): the party is frozen at once, so nothing more
// can leave its accounts, and a CRITICAL case opens for a compliance officer to confirm or clear. Unfreezing after a
// false positive is a deliberate operations step, never automatic.
package rescreen

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/cases"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
)

//go:embed schema.sql
var schema string

// Open applies the schema.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	return err
}

// Parties is what the rescreen needs from party-service.
type Parties interface {
	ListParties(context.Context, *connect.Request[partyv1.ListPartiesRequest]) (*connect.Response[partyv1.ListPartiesResponse], error)
	SetStatus(context.Context, *connect.Request[partyv1.SetStatusRequest]) (*connect.Response[partyv1.SetStatusResponse], error)
}

// Rule is the case rule for a sanctions match.
const Rule = "SANCTIONS_MATCH"

// Screened are the statuses re-screened: every party that is, or is about to be, a customer.
var Screened = []partyv1.PartyStatus{partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING, partyv1.PartyStatus_PARTY_STATUS_ACTIVE, partyv1.PartyStatus_PARTY_STATUS_RESTRICTED}

// Rescreener runs the rescreen.
type Rescreener struct {
	Pool     *pgxpool.Pool
	Parties  Parties
	Screener screening.Screener
	Now      func() time.Time
}

// Result is one run's outcome.
type Result struct {
	Screened, Matched, Frozen int
	Skipped                   bool // this list version was already screened today
}

// Run screens every party against the list identified by version (its checksum). A run that finished for the same
// version on the same day is not repeated. A run that stopped part way is repeated from the start: opening a case and
// freezing are both idempotent.
func (r *Rescreener) Run(ctx context.Context, version string) (Result, error) {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	key := version + "@" + now().UTC().Format("2006-01-02")
	var done bool
	if err := r.Pool.QueryRow(ctx, `select exists (select 1 from rescreen_run where run_key = $1 and finished_at is not null)`, key).Scan(&done); err != nil {
		return Result{}, err
	}
	if done {
		return Result{Skipped: true}, nil
	}
	if _, err := r.Pool.Exec(ctx, `insert into rescreen_run (run_key, list_version, started_at) values ($1, $2, $3)
		on conflict (run_key) do update set started_at = excluded.started_at`, key, version, now().UTC()); err != nil {
		return Result{}, err
	}
	var res Result
	after := ""
	for {
		page, err := r.Parties.ListParties(ctx, connect.NewRequest(&partyv1.ListPartiesRequest{Statuses: Screened, AfterPartyId: after, Limit: 500}))
		if err != nil {
			return res, fmt.Errorf("rescreen: list parties: %w", err)
		}
		names := make([]string, 0, len(page.Msg.Parties))
		for _, p := range page.Msg.Parties {
			names = append(names, p.LegalName)
		}
		if len(names) > 0 {
			hits, err := r.Screener.Screen(ctx, names)
			if err != nil {
				// A stale or unavailable list stops the run; it is retried, never treated as clear.
				return res, fmt.Errorf("rescreen: screen: %w", err)
			}
			hit := map[string]bool{}
			for _, h := range hits.Hits {
				hit[h] = true
			}
			for _, p := range page.Msg.Parties {
				res.Screened++
				if !hit[p.LegalName] {
					continue
				}
				res.Matched++
				frozen, err := r.handle(ctx, p, version)
				if err != nil {
					return res, err
				}
				if frozen {
					res.Frozen++
				}
			}
		}
		if page.Msg.NextAfterPartyId == "" {
			break
		}
		after = page.Msg.NextAfterPartyId
	}
	_, err := r.Pool.Exec(ctx, `update rescreen_run set finished_at = $2, screened = $3, matched = $4 where run_key = $1`,
		key, now().UTC(), res.Screened, res.Matched)
	return res, err
}

// handle opens the case, then freezes the party. The case comes first, so a freeze never happens without a record of
// why; both are safe to repeat.
func (r *Rescreener) handle(ctx context.Context, p *partyv1.Party, version string) (bool, error) {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	if _, err := cases.New(r.Pool).OpenFromFindings(ctx, []monitor.Finding{{
		Rule: Rule, PartyID: p.PartyId, Severity: "CRITICAL",
		Detail:   "The party's name matched the sanctions list on re-screening. The party is frozen. Confirm the match and report it (s.28A), or clear it as a false positive and ask operations to unfreeze.",
		Evidence: []string{"list:" + version},
	}}, "sanctions-rescreen", now()); err != nil {
		return false, fmt.Errorf("rescreen: open case: %w", err)
	}
	if p.Status == partyv1.PartyStatus_PARTY_STATUS_FROZEN {
		return false, nil
	}
	// KYC_PENDING cannot move to FROZEN in the party lifecycle; such an applicant is declined by onboarding's own
	// screening before activation, and the case records the match.
	if p.Status == partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING {
		return false, nil
	}
	_, err := r.Parties.SetStatus(ctx, connect.NewRequest(&partyv1.SetStatusRequest{
		PartyId: p.PartyId, ToStatus: partyv1.PartyStatus_PARTY_STATUS_FROZEN, Reason: "sanctions screening match", Actor: "sanctions-rescreen",
	}))
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) && ce.Code() == connect.CodeFailedPrecondition {
			return false, nil // already moved on, for example frozen by someone else
		}
		return false, fmt.Errorf("rescreen: freeze: %w", err)
	}
	return true, nil
}
