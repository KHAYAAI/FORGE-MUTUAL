// Package service is the ConnectRPC surface of the consolidated view. Ownership is enforced here: a party sees only
// accounts it owns, and anything else looks the same as an account that does not exist.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/store"
)

// Service implements consolidatedv1connect.ConsolidatedViewServiceHandler.
type Service struct {
	Store *store.Store
}

// GetBalances returns the accounts the party owns, each with its holding entity and insured status.
func (s *Service) GetBalances(ctx context.Context, req *connect.Request[consolidatedv1.GetBalancesRequest]) (*connect.Response[consolidatedv1.GetBalancesResponse], error) {
	if req.Msg.PartyId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("party_id is required"))
	}
	views, err := s.Store.Balances(ctx, req.Msg.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &consolidatedv1.GetBalancesResponse{}
	for _, v := range views {
		out.Accounts = append(out.Accounts, &consolidatedv1.AccountView{
			AccountId: v.AccountID, Asset: v.Asset, AvailableMinor: fmt.Sprint(v.Available), HoldingEntity: "BANK",
			Insured: v.Insured, ProductId: v.ProductID, AsOf: v.AsOf.UTC().Format(time.RFC3339Nano), Source: v.Source,
		})
	}
	return connect.NewResponse(out), nil
}

// GetActivity returns movements on one of the party's accounts.
func (s *Service) GetActivity(ctx context.Context, req *connect.Request[consolidatedv1.GetActivityRequest]) (*connect.Response[consolidatedv1.GetActivityResponse], error) {
	m := req.Msg
	if m.PartyId == "" || m.AccountId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("party_id and account_id are required"))
	}
	if m.Limit < 1 || m.Limit > 500 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("limit must be between 1 and 500"))
	}
	rows, err := s.Store.Activity(ctx, m.PartyId, m.AccountId, m.AfterSeq, int(m.Limit)+1)
	if errors.Is(err, store.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("account not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &consolidatedv1.GetActivityResponse{}
	if len(rows) > int(m.Limit) {
		out.NextAfterSeq = rows[m.Limit-1].Seq
		rows = rows[:m.Limit]
	}
	for _, r := range rows {
		out.Entries = append(out.Entries, &consolidatedv1.ActivityEntry{
			Seq: r.Seq, TransferId: r.TransferID, Direction: r.Direction, AmountMinor: fmt.Sprint(r.Amount),
			Asset: r.Asset, Code: r.Code, At: r.At.UTC().Format(time.RFC3339Nano),
		})
	}
	return connect.NewResponse(out), nil
}

// GetReconStatus reports the last reconciliation.
func (s *Service) GetReconStatus(ctx context.Context, _ *connect.Request[consolidatedv1.GetReconStatusRequest]) (*connect.Response[consolidatedv1.GetReconStatusResponse], error) {
	at, checked, drifted, err := s.Store.LastRun(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &consolidatedv1.GetReconStatusResponse{AccountsChecked: uint32(checked), Drifted: uint32(drifted)}
	if !at.IsZero() {
		resp.LastRunAt = at.UTC().Format(time.RFC3339Nano)
	}
	return connect.NewResponse(resp), nil
}
