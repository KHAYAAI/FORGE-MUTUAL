package rescreen

import (
	"context"
	"errors"
	"os"
	"sort"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/cases"
)

type fakeParties struct {
	parties map[string]*partyv1.Party
	sets    int
}

func (f *fakeParties) ListParties(_ context.Context, r *connect.Request[partyv1.ListPartiesRequest]) (*connect.Response[partyv1.ListPartiesResponse], error) {
	ids := make([]string, 0, len(f.parties))
	for id := range f.parties {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	want := map[partyv1.PartyStatus]bool{}
	for _, s := range r.Msg.Statuses {
		want[s] = true
	}
	out := &partyv1.ListPartiesResponse{}
	for _, id := range ids {
		if id <= r.Msg.AfterPartyId || !want[f.parties[id].Status] {
			continue
		}
		out.Parties = append(out.Parties, f.parties[id])
		if len(out.Parties) == int(r.Msg.Limit) {
			out.NextAfterPartyId = id
			break
		}
	}
	return connect.NewResponse(out), nil
}

func (f *fakeParties) SetStatus(_ context.Context, r *connect.Request[partyv1.SetStatusRequest]) (*connect.Response[partyv1.SetStatusResponse], error) {
	f.sets++
	f.parties[r.Msg.PartyId].Status = r.Msg.ToStatus
	return connect.NewResponse(&partyv1.SetStatusResponse{}), nil
}

func setup(t *testing.T) (*pgxpool.Pool, *fakeParties) {
	t.Helper()
	dsn := os.Getenv("COMPLIANCE_TEST_DSN")
	if dsn == "" {
		t.Skip("COMPLIANCE_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := cases.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate rescreen_run, compliance_case`); err != nil {
		t.Fatal(err)
	}
	p := &fakeParties{parties: map[string]*partyv1.Party{}}
	for i, n := range []string{"Clean Co", "Listed Person", "Another Clean", "Fourth Party"} {
		id := "0000000" + string(rune('1'+i))
		p.parties[id] = &partyv1.Party{PartyId: id, LegalName: n, Status: partyv1.PartyStatus_PARTY_STATUS_ACTIVE}
	}
	return pool, p
}

// A match among many customers is frozen at once, with a CRITICAL case; a second run the same day does nothing more.
func TestAMatchIsFrozenWithACaseOnce(t *testing.T) {
	pool, p := setup(t)
	now := time.Now()
	fx, _ := screening.NewFixture([]string{"Listed Person"}, now, 48*time.Hour, time.Now)
	r := &Rescreener{Pool: pool, Parties: p, Screener: fx, Now: func() time.Time { return now }}
	res, err := r.Run(context.Background(), "sha-1")
	if err != nil || res.Screened != 4 || res.Matched != 1 || res.Frozen != 1 {
		t.Fatalf("run: %+v, %v", res, err)
	}
	if p.parties["00000002"].Status != partyv1.PartyStatus_PARTY_STATUS_FROZEN || p.parties["00000001"].Status != partyv1.PartyStatus_PARTY_STATUS_ACTIVE {
		t.Fatal("the wrong party was frozen")
	}
	var n int
	_ = pool.QueryRow(context.Background(), `select count(*) from compliance_case where rule = $1 and severity = 'CRITICAL' and party_id = '00000002'`, Rule).Scan(&n)
	if n != 1 {
		t.Fatalf("cases = %d", n)
	}
	again, err := r.Run(context.Background(), "sha-1")
	if err != nil || !again.Skipped || p.sets != 1 {
		t.Fatalf("second run: %+v, %v, freezes %d", again, err, p.sets)
	}
	// A new list version is screened again; the frozen party is not frozen twice and gets no second open case.
	if _, err := r.Run(context.Background(), "sha-2"); err != nil || p.sets != 1 {
		t.Fatalf("new version: %v, freezes %d", err, p.sets)
	}
	_ = pool.QueryRow(context.Background(), `select count(*) from compliance_case where party_id = '00000002'`).Scan(&n)
	if n != 1 {
		t.Fatalf("cases after a new version = %d", n)
	}
}

// A stale list stops the run with an error: nobody is treated as clear on an old list.
func TestAStaleListStopsTheRun(t *testing.T) {
	pool, p := setup(t)
	old := time.Now().Add(-72 * time.Hour)
	fx, _ := screening.NewFixture([]string{"Listed Person"}, old, 24*time.Hour, time.Now)
	r := &Rescreener{Pool: pool, Parties: p, Screener: fx}
	if _, err := r.Run(context.Background(), "sha-old"); !errors.Is(err, screening.ErrStale) {
		t.Fatalf("stale list: %v", err)
	}
	var finished bool
	_ = pool.QueryRow(context.Background(), `select finished_at is not null from rescreen_run where list_version = 'sha-old'`).Scan(&finished)
	if finished || p.sets != 0 {
		t.Fatal("a failed run was recorded as finished, or froze someone")
	}
}
