package inbound

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
)

// References serves paymentsv1connect.InboundReferenceServiceHandler over the worker's reference table.
type References struct {
	Pool *pgxpool.Pool
}

// RegisterInboundReference implements the handler.
func (r *References) RegisterInboundReference(ctx context.Context, req *connect.Request[paymentsv1.RegisterInboundReferenceRequest]) (*connect.Response[paymentsv1.RegisterInboundReferenceResponse], error) {
	m := req.Msg
	if m.Reference == "" || len(m.Reference) > 35 || strings.TrimSpace(m.Reference) != m.Reference {
		// 35 characters is the longest reference SWIFT and EFT both carry.
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reference must be 1 to 35 characters with no surrounding spaces"))
	}
	w := &Worker{Pool: r.Pool}
	if err := w.RegisterReference(ctx, m.Reference, m.Asset, m.AccountId, m.PartyId); err != nil {
		if strings.Contains(err.Error(), "already bound") {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		if strings.Contains(err.Error(), "required") {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&paymentsv1.RegisterInboundReferenceResponse{}), nil
}

// ListInboundReferences implements the handler. Only the party's own references are returned.
func (r *References) ListInboundReferences(ctx context.Context, req *connect.Request[paymentsv1.ListInboundReferencesRequest]) (*connect.Response[paymentsv1.ListInboundReferencesResponse], error) {
	if req.Msg.PartyId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("party_id is required"))
	}
	rows, err := r.Pool.Query(ctx, `select reference, asset, account_id from inbound_reference where party_id = $1 order by asset, reference`, req.Msg.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	defer rows.Close()
	out := &paymentsv1.ListInboundReferencesResponse{}
	for rows.Next() {
		var ref paymentsv1.InboundReference
		if err := rows.Scan(&ref.Reference, &ref.Asset, &ref.AccountId); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		out.References = append(out.References, &ref)
	}
	return connect.NewResponse(out), rows.Err()
}
