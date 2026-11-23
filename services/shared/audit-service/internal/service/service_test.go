package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"

	auditv1 "github.com/khayaai/forge-mutual/pkg/api/forge/audit/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/audit/v1/auditv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/store"
)

func TestWritesAreRecordedUnderTheCallerIdentity(t *testing.T) {
	dsn := os.Getenv("AUDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("AUDIT_TEST_DSN not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	_, _ = st.Pool().Exec(ctx, `alter table audit_entry disable trigger audit_entry_no_update`)
	if _, err := st.Pool().Exec(ctx, `delete from audit_entry`); err != nil {
		t.Fatal(err)
	}
	_, _ = st.Pool().Exec(ctx, `alter table audit_entry enable trigger audit_entry_no_update`)

	allow := callerauth.Allowlist{"svc-policy": {"Append": true, "Query": true, "Head": true}}
	mux := http.NewServeMux()
	path, h := auditv1connect.NewAuditServiceHandler(&Service{Store: st}, connect.WithInterceptors(allow.Interceptor(callerauth.DefaultHeader)))
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := auditv1connect.NewAuditServiceClient(srv.Client(), srv.URL)

	anon := connect.NewRequest(&auditv1.AppendRequest{Action: "a", Entity: "BANK", PayloadJson: "{}"})
	if _, err := client.Append(ctx, anon); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("no identity: %v", err)
	}

	req := connect.NewRequest(&auditv1.AppendRequest{Action: "policy.decided", Entity: "BANK", PayloadJson: `{"allow":true}`})
	req.Header().Set(callerauth.DefaultHeader, "svc-policy")
	res, err := client.Append(ctx, req)
	if err != nil || res.Msg.Seq != 1 {
		t.Fatalf("append: %+v, %v", res, err)
	}

	bad := connect.NewRequest(&auditv1.AppendRequest{Action: "", Entity: "BANK", PayloadJson: "{}"})
	bad.Header().Set(callerauth.DefaultHeader, "svc-policy")
	if _, err := client.Append(ctx, bad); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty action: %v", err)
	}

	q := connect.NewRequest(&auditv1.QueryRequest{Entity: "BANK", Limit: 10})
	q.Header().Set(callerauth.DefaultHeader, "svc-policy")
	list, err := client.Query(ctx, q)
	if err != nil || len(list.Msg.Entries) != 1 {
		t.Fatalf("query: %+v, %v", list, err)
	}
	if e := list.Msg.Entries[0]; e.Actor != "svc-policy" || json.Valid([]byte(e.PayloadJson)) == false {
		t.Fatalf("entry = %+v", e)
	}
}
