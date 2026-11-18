package callerauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

type fakeLedger struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
}

func (fakeLedger) GetBalances(context.Context, *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	return connect.NewResponse(&ledgerv1.GetBalancesResponse{}), nil
}

func TestAllowlistDeniesByDefault(t *testing.T) {
	allow := Allowlist{"svc-payments": {"GetBalances": true}}
	path, h := ledgerv1connect.NewLedgerServiceHandler(fakeLedger{}, connect.WithInterceptors(allow.Interceptor(DefaultHeader)))
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, srv.URL)

	call := func(caller string) error {
		r := connect.NewRequest(&ledgerv1.GetBalancesRequest{EntityCode: "BANK"})
		if caller != "" {
			r.Header().Set(DefaultHeader, caller)
		}
		_, err := client.GetBalances(context.Background(), r)
		return err
	}
	if got := connect.CodeOf(call("")); got != connect.CodeUnauthenticated {
		t.Errorf("no caller: %v", got)
	}
	if got := connect.CodeOf(call("svc-unknown")); got != connect.CodePermissionDenied {
		t.Errorf("unknown caller: %v", got)
	}
	if err := call("svc-payments"); err != nil {
		t.Errorf("granted call: %v", err)
	}
}
