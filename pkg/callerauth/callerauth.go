// Package callerauth enforces a deny-by-default caller allow-list per RPC (SPEC: section 3, rule 1
// of the ledger gateway, reused by every service).
//
// The caller is the service identity the transport established. In production that is the mutual
// TLS certificate the peer presented (see pkg/serve); the request header is never read there. In
// development, with no certificates, the caller comes from DefaultHeader so that local stacks and
// tests need no PKI. HeaderSource returns which of the two applies to this process.
package callerauth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
)

// DefaultHeader carries the caller identity in development only.
const DefaultHeader = "X-Forge-Caller"

// HeaderSource is the header Interceptor reads the caller from in this process. In production it is
// empty: the caller must come from the transport, and a request header is ignored.
func HeaderSource() string {
	if IsProduction() {
		return ""
	}
	return DefaultHeader
}

// IsProduction reports whether FORGE_ENV names production. Only the exact value counts, so a typo
// falls back to development rules and is caught by the stricter production checks elsewhere.
func IsProduction() bool { return os.Getenv("FORGE_ENV") == "production" }

type identityKey struct{}

// WithIdentity returns ctx carrying the service identity the transport established for this request.
func WithIdentity(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, identityKey{}, name)
}

// IdentityFrom returns the transport identity on ctx, or "" when none was established.
func IdentityFrom(ctx context.Context) string {
	s, _ := ctx.Value(identityKey{}).(string)
	return s
}

// Allowlist maps a caller identity to the RPC method names it may call.
type Allowlist map[string]map[string]bool

// Interceptor rejects requests with no caller (Unauthenticated) or a caller not allowed the
// method (PermissionDenied). The caller is the transport identity when there is one; otherwise it is
// read from header, which is ignored when header is empty.
func (a Allowlist) Interceptor(header string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			caller := IdentityFrom(ctx)
			if caller == "" && header != "" {
				caller = req.Header().Get(header)
			}
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

// Caller returns the caller of a request: the transport identity when there is one, otherwise the
// development header. Code that records who acted (audit actors) must use this rather than reading
// the header itself, so that a client cannot name itself in production.
func Caller(ctx context.Context, h http.Header) string {
	if id := IdentityFrom(ctx); id != "" {
		return id
	}
	if src := HeaderSource(); src != "" {
		return h.Get(src)
	}
	return ""
}
