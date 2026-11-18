// Package membership keeps the member register of a mutual bank: who the members are, the shares they hold, the
// meetings and resolutions they vote on, and the surplus distributed to them (ARCHITECTURE.md 5.4).
//
// It is built because Forge plans to be a mutual bank (ADR-0020), and stays unused if Forge becomes a bank: Enabled
// reads the legal form's profile, so nothing here branches on the licence route. The voting rule comes from the same
// profile, as data. It is not wired into a running service until the licence decision is made.
package membership

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
)

//go:embed schema.sql
var Schema string

var (
	ErrNotMutual      = errors.New("membership: the legal form has no member shares")
	ErrNotMember      = errors.New("membership: not an active member")
	ErrNotEligible    = errors.New("membership: admitted after the meeting's record date")
	ErrAlreadyVoted   = errors.New("membership: this member has already voted on this resolution")
	ErrClosed         = errors.New("membership: the resolution is closed")
	ErrTooFewShares   = errors.New("membership: redemption is more than the member holds")
	ErrKeyReused      = errors.New("membership: idempotency key reused for a different request")
	ErrBadRequest     = errors.New("membership: invalid request")
	ErrCapitalAtRisk  = errors.New("membership: redemption refused by the capital check")
	ErrNoSuchMeeting  = errors.New("membership: no such meeting or resolution")
	ErrMinimumMembers = errors.New("membership: a mutual bank needs at least the founding number of members")
)

// Register is the member register.
type Register struct {
	Pool    *pgxpool.Pool
	Profile banklayer.Profile
	// CapitalCheck is asked before any redemption; it refuses one that would take capital below the minimum or the
	// risk appetite. Required: member shares are capital.
	CapitalCheck func(ctx context.Context, redeemMinor int64) error
}

// Enabled reports whether the legal form has members at all.
func (r Register) Enabled() bool { return r.Profile.MemberShares }

func (r Register) guard() error {
	if !r.Enabled() {
		return ErrNotMutual
	}
	return nil
}

func unique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// Admit makes a party a member. Admitting the same party again returns the existing member.
func (r Register) Admit(ctx context.Context, memberID, partyID, class string, at time.Time) (string, error) {
	if err := r.guard(); err != nil {
		return "", err
	}
	if memberID == "" || partyID == "" {
		return "", ErrBadRequest
	}
	var id string
	err := r.Pool.QueryRow(ctx, `
		insert into member (member_id, party_id, member_class, status, admitted_at) values ($1,$2,$3,'ACTIVE',$4)
		on conflict (party_id) do update set party_id = excluded.party_id
		returning member_id`, memberID, partyID, class, at).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	return id, nil
}

// Count is the number of active members.
func (r Register) Count(ctx context.Context) (int, error) {
	var n int
	err := r.Pool.QueryRow(ctx, `select count(*) from member where status = 'ACTIVE'`).Scan(&n)
	return n, err
}

// ReadyToApply checks the founding number of members (seven for a mutual bank).
func (r Register) ReadyToApply(ctx context.Context) error {
	n, err := r.Count(ctx)
	if err != nil {
		return err
	}
	if n < r.Profile.MinFounders {
		return fmt.Errorf("%w: %d of %d", ErrMinimumMembers, n, r.Profile.MinFounders)
	}
	return nil
}

func active(ctx context.Context, q pgx.Tx, memberID string) (time.Time, error) {
	var admitted time.Time
	err := q.QueryRow(ctx, `select admitted_at from member where member_id = $1 and status = 'ACTIVE' for share`, memberID).Scan(&admitted)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrNotMember
	}
	return admitted, err
}

// Holding is a member's shares in one class.
func Holding(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, memberID, class string) (units, paidUp int64, err error) {
	err = q.QueryRow(ctx, `select coalesce(sum(units),0), coalesce(sum(paid_up_minor),0) from member_share where member_id = $1 and share_class = $2`,
		memberID, class).Scan(&units, &paidUp)
	return
}

// MoveShares issues (units > 0) or redeems (units < 0) shares. The key makes it safe to retry.
func (r Register) MoveShares(ctx context.Context, key, memberID, class string, units, paidUpMinor int64, at time.Time) error {
	if err := r.guard(); err != nil {
		return err
	}
	if key == "" || units == 0 || (units > 0) != (paidUpMinor > 0) {
		return ErrBadRequest
	}
	return pgx.BeginFunc(ctx, r.Pool, func(tx pgx.Tx) error {
		var u, p int64
		var m, c string
		err := tx.QueryRow(ctx, `select member_id, share_class, units, paid_up_minor from member_share where movement_key = $1`, key).Scan(&m, &c, &u, &p)
		if err == nil {
			if m == memberID && c == class && u == units && p == paidUpMinor {
				return nil
			}
			return ErrKeyReused
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := active(ctx, tx, memberID); err != nil {
			return err
		}
		if units < 0 {
			// Redemptions of one member are serialised so two cannot both pass the holding check.
			if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('member-shares:' || $1))`, memberID); err != nil {
				return err
			}
			held, _, err := Holding(ctx, tx, memberID, class)
			if err != nil {
				return err
			}
			if held+units < 0 {
				return ErrTooFewShares
			}
			if r.CapitalCheck == nil {
				return fmt.Errorf("%w: no capital check configured", ErrCapitalAtRisk)
			}
			if err := r.CapitalCheck(ctx, -paidUpMinor); err != nil {
				return fmt.Errorf("%w: %v", ErrCapitalAtRisk, err)
			}
		}
		_, err = tx.Exec(ctx, `insert into member_share (movement_key, member_id, share_class, units, paid_up_minor, at) values ($1,$2,$3,$4,$5,$6)`,
			key, memberID, class, units, paidUpMinor, at)
		return err
	})
}

// CallMeeting records a general meeting. Members admitted after the notice is sent may not vote at it.
func (r Register) CallMeeting(ctx context.Context, meetingID, kind string, noticeSentAt time.Time, quorum int) error {
	if err := r.guard(); err != nil {
		return err
	}
	_, err := r.Pool.Exec(ctx, `insert into meeting (meeting_id, kind, notice_sent_at, quorum) values ($1,$2,$3,$4) on conflict do nothing`,
		meetingID, kind, noticeSentAt, quorum)
	return err
}

// Propose adds a resolution. ORDINARY passes on more than half the weight cast for or against; SPECIAL on at least
// three quarters (PROPOSED; the constitution decides). The voting rule is the legal form's.
func (r Register) Propose(ctx context.Context, resolutionID, meetingID, textRef, kind string) error {
	if err := r.guard(); err != nil {
		return err
	}
	_, err := r.Pool.Exec(ctx, `insert into resolution (resolution_id, meeting_id, text_ref, kind, voting_rule) values ($1,$2,$3,$4,$5) on conflict do nothing`,
		resolutionID, meetingID, textRef, kind, r.Profile.VotingRule)
	return err
}

// Vote casts one member's vote. One member, one vote: a second vote by any channel is refused.
func (r Register) Vote(ctx context.Context, resolutionID, memberID, choice, channel string, at time.Time) error {
	if err := r.guard(); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, r.Pool, func(tx pgx.Tx) error {
		var notice time.Time
		var rule string
		var result *string
		err := tx.QueryRow(ctx, `select m.notice_sent_at, r.voting_rule, r.result from resolution r join meeting m using (meeting_id) where r.resolution_id = $1 for share of r`,
			resolutionID).Scan(&notice, &rule, &result)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoSuchMeeting
		}
		if err != nil {
			return err
		}
		if result != nil {
			return ErrClosed
		}
		admitted, err := active(ctx, tx, memberID)
		if err != nil {
			return err
		}
		if admitted.After(notice) {
			return ErrNotEligible
		}
		weight := int64(1)
		if rule == "BY_SHARES" {
			if err := tx.QueryRow(ctx, `select coalesce(sum(units),0) from member_share where member_id = $1`, memberID).Scan(&weight); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `insert into member_vote (resolution_id, member_id, weight, choice, channel, cast_at) values ($1,$2,$3,$4,$5,$6)`,
			resolutionID, memberID, weight, choice, channel, at)
		if unique(err) {
			return ErrAlreadyVoted
		}
		return err
	})
}

// Result is a closed resolution.
type Result struct {
	Outcome               string
	For, Against, Abstain int64
	Voters                int
}

// Close counts the votes and records the result. Closing again returns the recorded result.
func (r Register) Close(ctx context.Context, resolutionID string) (Result, error) {
	if err := r.guard(); err != nil {
		return Result{}, err
	}
	var res Result
	err := pgx.BeginFunc(ctx, r.Pool, func(tx pgx.Tx) error {
		var kind string
		var quorum int
		var outcome *string
		err := tx.QueryRow(ctx, `select r.kind, m.quorum, r.result from resolution r join meeting m using (meeting_id) where r.resolution_id = $1 for update of r`,
			resolutionID).Scan(&kind, &quorum, &outcome)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoSuchMeeting
		}
		if err != nil {
			return err
		}
		if outcome != nil {
			return tx.QueryRow(ctx, `select result, for_weight, against_weight, abstain_weight, voters from resolution where resolution_id = $1`, resolutionID).
				Scan(&res.Outcome, &res.For, &res.Against, &res.Abstain, &res.Voters)
		}
		if err := tx.QueryRow(ctx, `select coalesce(sum(weight) filter (where choice='FOR'),0), coalesce(sum(weight) filter (where choice='AGAINST'),0),
			coalesce(sum(weight) filter (where choice='ABSTAIN'),0), count(*) from member_vote where resolution_id = $1`, resolutionID).
			Scan(&res.For, &res.Against, &res.Abstain, &res.Voters); err != nil {
			return err
		}
		res.Outcome = Decide(kind, quorum, res)
		_, err = tx.Exec(ctx, `update resolution set result=$2, for_weight=$3, against_weight=$4, abstain_weight=$5, voters=$6 where resolution_id=$1`,
			resolutionID, res.Outcome, res.For, res.Against, res.Abstain, res.Voters)
		return err
	})
	return res, err
}

// Decide applies the quorum and the majority. Abstentions count towards the quorum but not the majority.
func Decide(kind string, quorum int, r Result) string {
	if r.Voters < quorum {
		return "INQUORATE"
	}
	cast := r.For + r.Against
	if cast == 0 {
		return "FAILED"
	}
	switch kind {
	case "SPECIAL":
		if r.For*4 >= cast*3 {
			return "PASSED"
		}
	default:
		if r.For*2 > cast {
			return "PASSED"
		}
	}
	return "FAILED"
}

// Allocate splits a pool between members in proportion to their weights (fees paid, balances, shares), to the cent:
// the parts always add up to the pool, with leftover cents going to the largest remainders, ties by member id.
func Allocate(pool int64, weights map[string]int64) (map[string]int64, error) {
	if pool < 0 {
		return nil, ErrBadRequest
	}
	var total int64
	ids := make([]string, 0, len(weights))
	for id, w := range weights {
		if w < 0 {
			return nil, ErrBadRequest
		}
		total += w
		ids = append(ids, id)
	}
	out := map[string]int64{}
	if total == 0 || pool == 0 {
		for _, id := range ids {
			out[id] = 0
		}
		return out, nil
	}
	sort.Strings(ids)
	type rem struct {
		id string
		r  int64
	}
	rems := make([]rem, 0, len(ids))
	var given int64
	for _, id := range ids {
		q, rm := mulDiv(pool, weights[id], total)
		out[id] = q
		given += q
		rems = append(rems, rem{id, rm})
	}
	sort.SliceStable(rems, func(i, j int) bool { return rems[i].r > rems[j].r })
	for i := int64(0); i < pool-given; i++ {
		out[rems[i].id]++
	}
	return out, nil
}

// mulDiv returns a*b/c and its remainder, exactly, whatever the size of a*b.
func mulDiv(a, b, c int64) (int64, int64) {
	q, r := new(big.Int).QuoRem(new(big.Int).Mul(big.NewInt(a), big.NewInt(b)), big.NewInt(c), new(big.Int))
	return q.Int64(), r.Int64()
}

// Distribute records an approved allocation for a period. A second run of the same period and basis changes nothing.
func (r Register) Distribute(ctx context.Context, period, basis, approvedBy string, amounts map[string]int64) error {
	if err := r.guard(); err != nil {
		return err
	}
	if approvedBy == "" {
		return fmt.Errorf("%w: a distribution needs a named approver (maker-checker)", ErrBadRequest)
	}
	return pgx.BeginFunc(ctx, r.Pool, func(tx pgx.Tx) error {
		for id, amt := range amounts {
			if _, err := tx.Exec(ctx, `insert into member_distribution (period, member_id, basis, amount_minor, approved_by) values ($1,$2,$3,$4,$5)
				on conflict do nothing`, period, id, basis, amt, approvedBy); err != nil {
				return err
			}
		}
		return nil
	})
}
