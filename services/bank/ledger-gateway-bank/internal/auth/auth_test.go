package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/memrepo"
)

func TestAllowlistDeniesByDefault(t *testing.T) {
	svc := gateway.NewService(gateway.DefaultConfig(), ledger.NewMemStore(nil), memrepo.New())
	allow := Allowlist{"svc-payments": {"CreateTransfers": true, "GetBalances": true}}
	path, h := ledgerv1connect.NewLedgerServiceHandler(svc, connect.WithInterceptors(allow.Interceptor()))
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, srv.URL)

	balances := func(caller string) error {
		r := connect.NewRequest(&ledgerv1.GetBalancesRequest{EntityCode: "BANK"})
		if caller != "" {
			r.Header().Set(gateway.CallerHeader, caller)
		}
		_, err := client.GetBalances(context.Background(), r)
		return err
	}
	createAccounts := func(caller string) error {
		r := connect.NewRequest(&ledgerv1.CreateAccountsRequest{})
		r.Header().Set(gateway.CallerHeader, caller)
		_, err := client.CreateAccounts(context.Background(), r)
		return err
	}

	if got := connect.CodeOf(balances("")); got != connect.CodeUnauthenticated {
		t.Errorf("no caller: code %v, want Unauthenticated", got)
	}
	if got := connect.CodeOf(balances("svc-unknown")); got != connect.CodePermissionDenied {
		t.Errorf("unknown caller: code %v, want PermissionDenied", got)
	}
	if got := connect.CodeOf(createAccounts("svc-payments")); got != connect.CodePermissionDenied {
		t.Errorf("method not granted: code %v, want PermissionDenied", got)
	}
	if err := balances("svc-payments"); err != nil {
		t.Errorf("granted call failed: %v", err)
	}
}
