package ledgerread

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

type book struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	codes   map[string]uint32
	debits  map[string]int64
	credits map[string]int64
}

func (b *book) ListAccounts(_ context.Context, r *connect.Request[ledgerv1.ListAccountsRequest]) (*connect.Response[ledgerv1.ListAccountsResponse], error) {
	var ids []string
	for id := range b.codes {
		if id > r.Msg.AfterAccountId {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := &ledgerv1.ListAccountsResponse{}
	for _, id := range ids {
		out.Accounts = append(out.Accounts, &ledgerv1.DirectoryAccount{AccountId: id, Ledger: 710, Code: b.codes[id]})
	}
	return connect.NewResponse(out), nil
}

func (b *book) GetBalances(_ context.Context, r *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	out := &ledgerv1.GetBalancesResponse{}
	for _, id := range r.Msg.AccountIds {
		out.Balances = append(out.Balances, &ledgerv1.AccountBalance{
			AccountId: id, DebitsPosted: strconv.FormatInt(b.debits[id], 10), CreditsPosted: strconv.FormatInt(b.credits[id], 10),
		})
	}
	return connect.NewResponse(out), nil
}

// The reader must classify balances by code and sign: customers credit-normal, pool debit-normal, suspense
// credit-normal. It must also find the largest customer account.
func TestSnapshotClassifiesByCodeAndSign(t *testing.T) {
	b := &book{
		codes:   map[string]uint32{},
		debits:  map[string]int64{},
		credits: map[string]int64{},
	}
	id := func(n int) string { return fmt.Sprintf("%032x", n) }
	b.codes[id(1)], b.codes[id(2)], b.codes[id(3)], b.codes[id(4)] = 3003, 1001, 1001, 2005
	b.debits[id(1)] = 10000 // pool debited by 10000 inbound
	b.credits[id(2)] = 7000 // customer one: 7000
	b.credits[id(3)] = 2000 // customer two: 2000
	b.debits[id(3)] = 500   // customer two has paid out 500, so 1500 remains
	b.credits[id(4)] = 300  // suspense

	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(b)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	r := Reader{Client: ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL), Entity: "BANK"}

	snap, err := r.Snapshot(context.Background(), Asset{Code: "ZAR", Ledger: 710, PoolCode: 3003})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Deposits != 8500 {
		t.Fatalf("deposits = %d, want 8500", snap.Deposits)
	}
	if snap.LargestDeposit != 7000 || snap.LargestAccount != id(2) {
		t.Fatalf("largest = %d at %s", snap.LargestDeposit, snap.LargestAccount)
	}
	if snap.Pool != 10000 {
		t.Fatalf("pool = %d, want 10000", snap.Pool)
	}
	if snap.Suspense != 300 {
		t.Fatalf("suspense = %d, want 300", snap.Suspense)
	}
}
