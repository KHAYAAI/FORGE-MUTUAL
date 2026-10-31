package systemaccounts

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

// accounts replays by key, the way the ledger gateway does.
type accounts struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	mu   sync.Mutex
	seq  int
	keys map[string]string
}

func (a *accounts) CreateAccounts(_ context.Context, r *connect.Request[ledgerv1.CreateAccountsRequest]) (*connect.Response[ledgerv1.CreateAccountsResponse], error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if id, ok := a.keys[r.Msg.IdempotencyKey]; ok {
		return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_REPLAYED, AccountIds: []string{id}}), nil
	}
	a.seq++
	id := fmt.Sprintf("%032x", a.seq)
	a.keys[r.Msg.IdempotencyKey] = id
	return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_OK, AccountIds: []string{id}}), nil
}

func client(t *testing.T) ledgerv1connect.LedgerServiceClient {
	t.Helper()
	path, h := ledgerv1connect.NewLedgerServiceHandler(&accounts{keys: map[string]string{}})
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL)
}

// The same key from two callers must give the same account, and a different key a different one.
func TestEnsureIsStableByKey(t *testing.T) {
	c := client(t)
	ctx := context.Background()
	a, err := Ensure(ctx, c, "BANK", "ZAR", 3003, "system:pool:ZAR:v1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Ensure(ctx, c, "BANK", "ZAR", 3003, "system:pool:ZAR:v1")
	if err != nil || a != b {
		t.Fatalf("same key gave %s then %s (%v)", a, b, err)
	}
	s, err := Ensure(ctx, c, "BANK", "ZAR", 2005, "system:suspense:ZAR:v1")
	if err != nil || s == a {
		t.Fatalf("a different key must give a different account: %s, %v", s, err)
	}
}

func TestEnsureRefusesMissingFields(t *testing.T) {
	if _, err := Ensure(context.Background(), client(t), "BANK", "ZAR", 0, "k"); err == nil {
		t.Fatal("a zero code must be refused")
	}
}
