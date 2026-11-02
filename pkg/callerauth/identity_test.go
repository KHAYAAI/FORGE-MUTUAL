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

// withTransportIdentity stands in for the TLS layer: it sets the identity a verified certificate
// would give, from a test header that is never used in production.
func withTransportIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Header.Get("X-Test-Transport-Identity"); id != "" {
			r = r.WithContext(WithIdentity(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}

func callerClient(base, header, transport string) error {
	client := ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, base, connect.WithInterceptors(
		connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if header != "" {
					req.Header().Set(DefaultHeader, header)
				}
				if transport != "" {
					req.Header().Set("X-Test-Transport-Identity", transport)
				}
				return next(ctx, req)
			}
		})))
	_, err := client.GetBalances(context.Background(), connect.NewRequest(&ledgerv1.GetBalancesRequest{}))
	return err
}

// The transport identity decides who the caller is. A header cannot promote a caller the transport
// did not identify as allowed, and cannot demote one it did.
func TestTransportIdentityDecidesTheCaller(t *testing.T) {
	allow := Allowlist{"svc-workspace": {"GetBalances": true}}
	path, h := ledgerv1connect.NewLedgerServiceHandler(fakeLedger{}, connect.WithInterceptors(allow.Interceptor(DefaultHeader)))
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(withTransportIdentity(mux))
	t.Cleanup(srv.Close)

	cases := []struct {
		name, header, transport string
		allowed                 bool
	}{
		{"allowed identity, spoofed header", "svc-intruder", "svc-workspace", true},
		{"denied identity, allowed header", "svc-workspace", "svc-intruder", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := callerClient(srv.URL, c.header, c.transport)
			if c.allowed && err != nil {
				t.Fatalf("allowed caller refused: %v", err)
			}
			if !c.allowed && connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("denied caller: code = %v (err %v), want PermissionDenied", connect.CodeOf(err), err)
			}
		})
	}
}

// In production the header is never read: a caller that only claims an allowed name is refused.
func TestProductionIgnoresTheHeader(t *testing.T) {
	t.Setenv("FORGE_ENV", "production")
	if HeaderSource() != "" {
		t.Fatal("production must not read the caller from a request header")
	}
	allow := Allowlist{"svc-workspace": {"GetBalances": true}}
	path, h := ledgerv1connect.NewLedgerServiceHandler(fakeLedger{}, connect.WithInterceptors(allow.Interceptor(HeaderSource())))
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	err := callerClient(srv.URL, "svc-workspace", "")
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("header-only caller in production: code = %v, want Unauthenticated", connect.CodeOf(err))
	}
}

func TestDevelopmentStillReadsTheHeader(t *testing.T) {
	t.Setenv("FORGE_ENV", "development")
	if HeaderSource() != DefaultHeader {
		t.Fatal("development keeps the header for local stacks and tests")
	}
}
