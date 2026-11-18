package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"

	reconv1 "github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/store"
)

// The gate is read by payments through this surface. The allow-list must refuse callers that are not named, and an
// empty identity must never reach the handler.
func TestGateIsOnlyServedToNamedCallers(t *testing.T) {
	dsn := os.Getenv("RECON_TEST_DSN")
	if dsn == "" {
		t.Skip("RECON_TEST_DSN not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if _, err := st.Pool().Exec(ctx, `truncate recon_investigation, recon_break, recon_run`); err != nil {
		t.Fatal(err)
	}

	svc := &Service{Store: st, Assets: []string{"ZAR"}, MaxAge: time.Hour, Now: time.Now}
	allow := callerauth.Allowlist{
		"svc-payments": {"GetGate": true},
	}
	mux := http.NewServeMux()
	path, h := reconv1connect.NewReconServiceHandler(svc, connect.WithInterceptors(allow.Interceptor(callerauth.DefaultHeader)))
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := reconv1connect.NewReconServiceClient(srv.Client(), srv.URL)

	noIdentity := connect.NewRequest(&reconv1.GetGateRequest{})
	if _, err := client.GetGate(ctx, noIdentity); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("no caller identity: code %v, want Unauthenticated", connect.CodeOf(err))
	}

	notAllowed := connect.NewRequest(&reconv1.ListBreaksRequest{})
	notAllowed.Header().Set(callerauth.DefaultHeader, "svc-payments")
	if _, err := client.ListBreaks(ctx, notAllowed); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("payments may not list breaks: code %v, want PermissionDenied", connect.CodeOf(err))
	}

	allowed := connect.NewRequest(&reconv1.GetGateRequest{})
	allowed.Header().Set(callerauth.DefaultHeader, "svc-payments")
	resp, err := client.GetGate(ctx, allowed)
	if err != nil {
		t.Fatalf("named caller: %v", err)
	}
	if !resp.Msg.BreakActive {
		t.Fatal("with no run recorded the gate must be closed")
	}

	// A matched run opens the gate for the named caller.
	if err := st.Commit(ctx, store.Run{Asset: "ZAR", StartedAt: time.Now(), FinishedAt: time.Now(),
		Outcome: store.OutcomeConclusive, Checks: []recon.Result{{Check: "statement_arithmetic", Matched: true}}}); err != nil {
		t.Fatal(err)
	}
	resp, err = client.GetGate(ctx, allowed)
	if err != nil || resp.Msg.BreakActive {
		t.Fatalf("after a matched run the gate must be open: %+v, %v", resp, err)
	}
}
