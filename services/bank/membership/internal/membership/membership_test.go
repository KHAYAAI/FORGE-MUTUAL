package membership

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
)

// These tests need PostgreSQL: MEMBERSHIP_TEST_DSN names a throwaway database; tables are dropped.

var t0 = time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC)

func register(t *testing.T, form banklayer.LegalForm) Register {
	t.Helper()
	dsn := os.Getenv("MEMBERSHIP_TEST_DSN")
	if dsn == "" {
		t.Skip("MEMBERSHIP_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `drop table if exists member_distribution, member_vote, resolution, meeting, member_share, member cascade`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	p, _ := banklayer.ProfileFor(form)
	return Register{Pool: pool, Profile: p, CapitalCheck: func(context.Context, int64) error { return nil }}
}

func admit(t *testing.T, r Register, n int, at time.Time) []string {
	t.Helper()
	var ids []string
	for i := range n {
		id, err := r.Admit(context.Background(), fmt.Sprintf("m%d", i), fmt.Sprintf("party-%d", i), "INDIVIDUAL", at)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func TestABankHasNoMembers(t *testing.T) {
	r := register(t, banklayer.LegalFormBank)
	if _, err := r.Admit(context.Background(), "m1", "p1", "INDIVIDUAL", t0); !errors.Is(err, ErrNotMutual) {
		t.Fatalf("a bank must refuse members: %v", err)
	}
}

func TestSevenFoundersBeforeApplying(t *testing.T) {
	r := register(t, banklayer.LegalFormMutual)
	ctx := context.Background()
	admit(t, r, 6, t0)
	if err := r.ReadyToApply(ctx); !errors.Is(err, ErrMinimumMembers) {
		t.Fatalf("six founders: %v", err)
	}
	if again, _ := r.Admit(ctx, "other-id", "party-0", "INDIVIDUAL", t0); again != "m0" {
		t.Fatalf("admitting the same party again must return the same member, got %s", again)
	}
	admit(t, r, 7, t0)
	if err := r.ReadyToApply(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSharesAreIssuedOnceAndCannotBeOverRedeemed(t *testing.T) {
	r := register(t, banklayer.LegalFormMutual)
	ctx := context.Background()
	admit(t, r, 1, t0)
	if err := r.MoveShares(ctx, "k1", "m0", "A", 10, 1_000_00, t0); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveShares(ctx, "k1", "m0", "A", 10, 1_000_00, t0); err != nil {
		t.Fatalf("a retry is not a second issue: %v", err)
	}
	if err := r.MoveShares(ctx, "k1", "m0", "A", 11, 1_100_00, t0); !errors.Is(err, ErrKeyReused) {
		t.Fatalf("reused key: %v", err)
	}
	if u, p, _ := Holding(ctx, r.Pool, "m0", "A"); u != 10 || p != 1_000_00 {
		t.Fatalf("holding %d units, %d paid", u, p)
	}
	if err := r.MoveShares(ctx, "k2", "m0", "A", -11, -1_100_00, t0); !errors.Is(err, ErrTooFewShares) {
		t.Fatalf("over-redemption: %v", err)
	}
	r.CapitalCheck = func(context.Context, int64) error { return errors.New("below minimum") }
	if err := r.MoveShares(ctx, "k3", "m0", "A", -5, -500_00, t0); !errors.Is(err, ErrCapitalAtRisk) {
		t.Fatalf("capital check: %v", err)
	}
	r.CapitalCheck = func(context.Context, int64) error { return nil }

	// Ten simultaneous redemptions of 2 units from 10: exactly five succeed.
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r.MoveShares(ctx, fmt.Sprintf("race-%d", i), "m0", "A", -2, -200_00, t0) == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if u, _, _ := Holding(ctx, r.Pool, "m0", "A"); ok != 5 || u != 0 {
		t.Fatalf("%d redemptions succeeded, %d units left", ok, u)
	}
}

func TestOneMemberOneVote(t *testing.T) {
	r := register(t, banklayer.LegalFormMutual)
	ctx := context.Background()
	ids := admit(t, r, 5, t0)
	// A large shareholder still has one vote.
	if err := r.MoveShares(ctx, "big", ids[0], "A", 1000, 100_000_00, t0); err != nil {
		t.Fatal(err)
	}
	notice := t0.Add(24 * time.Hour)
	late, _ := r.Admit(ctx, "late", "party-late", "BUSINESS", notice.Add(time.Hour))
	if err := r.CallMeeting(ctx, "agm-2027", "AGM", notice, 3); err != nil {
		t.Fatal(err)
	}
	if err := r.Propose(ctx, "res-1", "agm-2027", "docs/agm/2027/res-1.md", "ORDINARY"); err != nil {
		t.Fatal(err)
	}
	vote := func(m, c string) error { return r.Vote(ctx, "res-1", m, c, "app", notice.Add(48*time.Hour)) }
	if err := vote(ids[0], "AGAINST"); err != nil {
		t.Fatal(err)
	}
	if err := vote(ids[0], "FOR"); !errors.Is(err, ErrAlreadyVoted) {
		t.Fatalf("second vote: %v", err)
	}
	if err := vote(late, "FOR"); !errors.Is(err, ErrNotEligible) {
		t.Fatalf("admitted after the record date: %v", err)
	}
	for _, m := range ids[1:3] {
		if err := vote(m, "FOR"); err != nil {
			t.Fatal(err)
		}
	}
	res, err := r.Close(ctx, "res-1")
	if err != nil || res.Outcome != "PASSED" || res.For != 2 || res.Against != 1 {
		t.Fatalf("2 for, 1 against (the big holder) must pass: %+v %v", res, err)
	}
	if err := vote(ids[3], "AGAINST"); !errors.Is(err, ErrClosed) {
		t.Fatalf("vote after close: %v", err)
	}
	if again, _ := r.Close(ctx, "res-1"); again != res {
		t.Fatalf("closing again must return the same result: %+v", again)
	}
}

func TestQuorumAndMajorities(t *testing.T) {
	cases := []struct {
		kind   string
		quorum int
		r      Result
		want   string
	}{
		{"ORDINARY", 3, Result{For: 2, Voters: 2}, "INQUORATE"},
		{"ORDINARY", 2, Result{For: 1, Against: 1, Voters: 2}, "FAILED"},
		{"ORDINARY", 2, Result{For: 2, Against: 1, Abstain: 5, Voters: 8}, "PASSED"},
		{"SPECIAL", 2, Result{For: 2, Against: 1, Voters: 3}, "FAILED"},
		{"SPECIAL", 2, Result{For: 3, Against: 1, Voters: 4}, "PASSED"},
		{"ORDINARY", 1, Result{Abstain: 3, Voters: 3}, "FAILED"},
	}
	for _, c := range cases {
		if got := Decide(c.kind, c.quorum, c.r); got != c.want {
			t.Errorf("%s %+v = %s, want %s", c.kind, c.r, got, c.want)
		}
	}
}

func TestAllocationAddsUpToTheCent(t *testing.T) {
	got, err := Allocate(100, map[string]int64{"a": 1, "b": 1, "c": 1})
	if err != nil || got["a"]+got["b"]+got["c"] != 100 || got["a"] != 34 {
		t.Fatalf("three-way split of 100: %v %v", got, err)
	}
	big, _ := Allocate(9_000_000_000_000, map[string]int64{"x": 9_000_000_000_000, "y": 1})
	if big["x"]+big["y"] != 9_000_000_000_000 {
		t.Fatalf("large values: %v", big)
	}
	if _, err := Allocate(10, map[string]int64{"a": -1}); err == nil {
		t.Fatal("negative weights are refused")
	}
}

func TestDistributionNeedsAnApproverAndRunsOnce(t *testing.T) {
	r := register(t, banklayer.LegalFormMutual)
	ctx := context.Background()
	admit(t, r, 2, t0)
	amounts, _ := Allocate(1_000_00, map[string]int64{"m0": 3, "m1": 1})
	if err := r.Distribute(ctx, "2027", "FEE_REBATE", "", amounts); err == nil {
		t.Fatal("no approver")
	}
	for range 2 {
		if err := r.Distribute(ctx, "2027", "FEE_REBATE", "board-2027-04", amounts); err != nil {
			t.Fatal(err)
		}
	}
	var n, sum int64
	_ = r.Pool.QueryRow(ctx, `select count(*), sum(amount_minor) from member_distribution`).Scan(&n, &sum)
	if n != 2 || sum != 1_000_00 {
		t.Fatalf("%d rows, %d total", n, sum)
	}
}
