package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	ledgerv1 "github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/gen/forge/ledger/v1"
)

const procedure = "/forge.ledger.v1.LedgerService/CreateTransfers"

// The interceptor is exercised through a real Connect handler and client, so the procedure
// name and response types are the ones the gateway uses.
func TestOutcomesAndRejectionCodesAreCounted(t *testing.T) {
	c := New()
	c.Register(prometheus.NewRegistry())

	next := []*ledgerv1.CreateTransfersResponse{
		{Status: ledgerv1.Status_STATUS_OK},
		{Status: ledgerv1.Status_STATUS_REPLAYED},
		{Status: ledgerv1.Status_STATUS_REJECTED, RejectionCodes: []string{"INSUFFICIENT_FUNDS"}},
	}
	i := 0
	handler := connect.NewUnaryHandler(procedure,
		func(context.Context, *connect.Request[ledgerv1.CreateTransfersRequest]) (*connect.Response[ledgerv1.CreateTransfersResponse], error) {
			r := next[i]
			i++
			return connect.NewResponse(r), nil
		},
		connect.WithInterceptors(c.Interceptor()),
	)
	mux := http.NewServeMux()
	mux.Handle(procedure, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := connect.NewClient[ledgerv1.CreateTransfersRequest, ledgerv1.CreateTransfersResponse](http.DefaultClient, srv.URL+procedure)
	for range next {
		if _, err := client.CallUnary(context.Background(), connect.NewRequest(&ledgerv1.CreateTransfersRequest{})); err != nil {
			t.Fatal(err)
		}
	}

	checks := map[string]float64{
		"ok": 1, "replayed": 1, "rejected": 1,
	}
	for outcome, want := range checks {
		if got := testutil.ToFloat64(c.rpcs.WithLabelValues("CreateTransfers", outcome)); got != want {
			t.Errorf("outcome %s = %v, want %v", outcome, got, want)
		}
	}
	if got := testutil.ToFloat64(c.rejections.WithLabelValues("CreateTransfers", "INSUFFICIENT_FUNDS")); got != 1 {
		t.Errorf("rejection code = %v, want 1", got)
	}
}
