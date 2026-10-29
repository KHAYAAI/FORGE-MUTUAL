// Package callerauth enforces a deny-by-default caller allow-list per RPC (SPEC: section 3, rule 1
// of the ledger gateway, reused by every service). Production takes the caller from the mTLS
// workload identity; the header form is for local development only.
package callerauth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

// DefaultHeader carries the caller identity in local development.
const DefaultHeader = "X-Forge-Caller"

// Allowlist maps a caller identity to the RPC method names it may call.
type Allowlist map[string]map[string]bool

// Interceptor rejects requests with no caller (Unauthenticated) or a caller not allowed the
// method (PermissionDenied). header names the request header that carries the caller.
func (a Allowlist) Interceptor(header string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			caller := req.Header().Get(header)
			if caller == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("caller identity missing"))
			}
			procedure := req.Spec().Procedure
			method := procedure[strings.LastIndex(procedure, "/")+1:]
			if !a[caller][method] {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("caller not allowed for "+method))
			}
			return next(ctx, req)
		}
	}
}
