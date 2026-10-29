// Package auth enforces the per-RPC caller allow-list (SPEC.md section 3, rule 1). Deny is the default.
package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
)

// Allowlist maps a caller identity to the RPC method names it may call, e.g.
// {"svc-payments": {"CreateTransfers": true}}. Unlisted callers and methods are denied.
type Allowlist map[string]map[string]bool

// Interceptor rejects requests from callers that are not authenticated (Unauthenticated) or
// not allowed the method (PermissionDenied).
//
// Local development trusts the X-Forge-Caller header. Production must take the caller from the
// mTLS workload identity instead; that is not implemented here (SPEC.md section 9).
func (a Allowlist) Interceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			caller := req.Header().Get(gateway.CallerHeader)
			if caller == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("caller identity missing"))
			}
			method := req.Spec().Procedure[strings.LastIndex(req.Spec().Procedure, "/")+1:]
			if !a[caller][method] {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("caller not allowed for "+method))
			}
			return next(ctx, req)
		}
	}
}
