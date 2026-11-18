package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	reconv1 "github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
)

type gateStub struct {
	reconv1connect.UnimplementedReconServiceHandler
	breakActive bool
	fail        bool
}

func (g gateStub) GetGate(context.Context, *connect.Request[reconv1.GetGateRequest]) (*connect.Response[reconv1.GetGateResponse], error) {
	if g.fail {
		return nil, connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	}
	return connect.NewResponse(&reconv1.GetGateResponse{BreakActive: g.breakActive}), nil
}

func gateClient(t *testing.T, g gateStub) ConnectRecon {
	t.Helper()
	path, h := reconv1connect.NewReconServiceHandler(g)
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return ConnectRecon{Client: reconv1connect.NewReconServiceClient(srv.Client(), srv.URL)}
}

// An open break must read as breaking. A closed gate must read as clear.
func TestConnectReconReadsTheGate(t *testing.T) {
	if b, err := gateClient(t, gateStub{breakActive: true}).Breaking(context.Background()); err != nil || !b {
		t.Fatalf("open break: %v, %v", b, err)
	}
	if b, err := gateClient(t, gateStub{breakActive: false}).Breaking(context.Background()); err != nil || b {
		t.Fatalf("closed gate: %v, %v", b, err)
	}
}

// If the gate cannot be read, the answer is an error, never "no break". The activity then fails and retries.
func TestUnreadableGateIsAnErrorNotClear(t *testing.T) {
	b, err := gateClient(t, gateStub{fail: true}).Breaking(context.Background())
	if err == nil {
		t.Fatalf("an unreadable gate must be an error; got breaking=%v with no error", b)
	}
}
