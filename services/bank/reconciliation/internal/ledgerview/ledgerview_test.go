package ledgerview

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

// fakeLedger serves the two read RPCs from memory, paging accounts in the given size.
type fakeLedger struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	accounts map[string]*ledgerv1.DirectoryAccount
	balances map[string]*ledgerv1.AccountBalance
	pageSize int
	hideBal  string
}

func (f *fakeLedger) ListAccounts(_ context.Context, req *connect.Request[ledgerv1.ListAccountsRequest]) (*connect.Response[ledgerv1.ListAccountsResponse], error) {
	var ids []string
	for id, a := range f.accounts {
		if a.Ledger == req.Msg.Ledger && id > req.Msg.AfterAccountId {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := &ledgerv1.ListAccountsResponse{}
	n := min(len(ids), f.pageSize)
	for _, id := range ids[:n] {
		out.Accounts = append(out.Accounts, f.accounts[id])
	}
	if len(ids) > n {
		out.NextAfterAccountId = ids[n-1]
	}
	return connect.NewResponse(out), nil
}

func (f *fakeLedger) GetBalances(_ context.Context, req *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	out := &ledgerv1.GetBalancesResponse{}
	for _, id := range req.Msg.AccountIds {
		if id == f.hideBal {
			continue
		}
		if b, ok := f.balances[id]; ok {
			out.Balances = append(out.Balances, b)
		}
	}
	return connect.NewResponse(out), nil
}

func newFake(pageSize int) *fakeLedger {
	f := &fakeLedger{accounts: map[string]*ledgerv1.DirectoryAccount{}, balances: map[string]*ledgerv1.AccountBalance{}, pageSize: pageSize}
	add := func(id string, code uint32, dp, cp string) {
		f.accounts[id] = &ledgerv1.DirectoryAccount{AccountId: id, Ledger: 710, Code: code, Asset: "ZAR", EntityCode: "BANK"}
		f.balances[id] = &ledgerv1.AccountBalance{AccountId: id, DebitsPosted: dp, CreditsPosted: cp, DebitsPending: "0", CreditsPending: "0"}
	}
	for i := 0; i < 5; i++ {
		add(fmt.Sprintf("%032x", i+1), 1001, "0", "10")
	}
	add(fmt.Sprintf("%032x", 99), 3003, "50", "0")
	return f
}

func client(t *testing.T, f *fakeLedger) ledgerv1connect.LedgerServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(f)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL)
}

// Paging must cover every account, not just the first page.
func TestSnapshotPagesEveryAccount(t *testing.T) {
	f := newFake(2)
	snap, err := (Reader{Client: client(t, f), Entity: "BANK"}).Snapshot(context.Background(), 710)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Accounts != 6 {
		t.Fatalf("read %d accounts, want 6", snap.Accounts)
	}
	if got := snap.Totals.DebitsMinusCredits[1001]; got != -50 {
		t.Fatalf("customer total = %d, want -50", got)
	}
	if got := snap.Totals.DebitsMinusCredits[3003]; got != 50 {
		t.Fatalf("pool = %d, want 50", got)
	}
}

// The digest must change when any balance changes, and stay the same when nothing does.
func TestDigestTracksBooksExactly(t *testing.T) {
	f := newFake(10)
	r := Reader{Client: client(t, f), Entity: "BANK"}
	a, err := r.Snapshot(context.Background(), 710)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := r.Snapshot(context.Background(), 710)
	if a.Digest != b.Digest {
		t.Fatal("unchanged books must have equal digests")
	}
	f.balances[fmt.Sprintf("%032x", 1)].CreditsPosted = "11"
	c, _ := r.Snapshot(context.Background(), 710)
	if a.Digest == c.Digest {
		t.Fatal("a changed balance must change the digest")
	}
}

// A balance missing for a listed account means the read is inconsistent. It must fail, not undercount.
func TestMissingBalanceFails(t *testing.T) {
	f := newFake(10)
	f.hideBal = fmt.Sprintf("%032x", 3)
	if _, err := (Reader{Client: client(t, f), Entity: "BANK"}).Snapshot(context.Background(), 710); err == nil {
		t.Fatal("a missing balance must fail the snapshot")
	}
}
