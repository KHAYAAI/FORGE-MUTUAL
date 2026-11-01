// Package service is the ConnectRPC surface of the partner router. It chooses a partner, records outcomes, pins
// payments, and refuses rather than guess when no partner can take an operation.
package service

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	partnerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/partner/v1"
	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/services/bank/partner-bank-router/internal/router"
	"github.com/khayaai/forge-mutual/services/bank/partner-bank-router/internal/store"
)

// Window size for health: the last this many outcomes.
const windowSize = 50

// Partner is one partner bank, with its cost.
type Partner struct {
	Provider banklayer.ForeignAccountProvider
	CostBps  int64
}

// Service implements partnerv1connect.PartnerRouterServiceHandler.
type Service struct {
	Pool     *pgxpool.Pool
	Partners []Partner
}

func (s *Service) byName(name string) (Partner, bool) {
	for _, p := range s.Partners {
		if p.Provider.Partner() == name {
			return p, true
		}
	}
	return Partner{}, false
}

// candidates reads each partner's recent outcomes for the health decision.
func (s *Service) candidates(ctx context.Context, asset string) ([]router.Candidate, error) {
	var out []router.Candidate
	for _, p := range s.Partners {
		w, err := store.Window(ctx, s.Pool, p.Provider.Partner(), windowSize)
		if err != nil {
			return nil, err
		}
		out = append(out, router.Candidate{Name: p.Provider.Partner(), Asset: p.Provider.Asset(), CostBps: p.CostBps, Window: w})
	}
	return out, nil
}

// choose returns the partner for a new operation.
func (s *Service) choose(ctx context.Context, asset string) (Partner, error) {
	cands, err := s.candidates(ctx, asset)
	if err != nil {
		return Partner{}, connect.NewError(connect.CodeInternal, err)
	}
	c, err := router.Choose(asset, cands)
	if err != nil {
		return Partner{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	p, _ := s.byName(c.Name)
	return p, nil
}

// isFailure reports whether an error is a partner failure. A refusal of the request itself (invalid, conflicting) is not:
// the partner is working, and the request was wrong.
func isFailure(err error) bool {
	return !errors.Is(err, banklayer.ErrInvalid) && !errors.Is(err, banklayer.ErrConflict) && !errors.Is(err, banklayer.ErrNotFound)
}

// OpenAccount opens a receiving account with the chosen partner.
func (s *Service) OpenAccount(ctx context.Context, req *connect.Request[partnerv1.OpenAccountRequest]) (*connect.Response[partnerv1.OpenAccountResponse], error) {
	if req.Msg.Owner == "" || req.Msg.Asset == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("owner and asset are required"))
	}
	p, err := s.choose(ctx, req.Msg.Asset)
	if err != nil {
		return nil, err
	}
	acc, err := p.Provider.OpenAccount(ctx, banklayer.OpenAccountRequest{Owner: req.Msg.Owner, Asset: req.Msg.Asset})
	if err != nil {
		if isFailure(err) {
			_ = store.Record(ctx, s.Pool, p.Provider.Partner(), req.Msg.Asset, false)
		}
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	_ = store.Record(ctx, s.Pool, p.Provider.Partner(), req.Msg.Asset, true)
	return connect.NewResponse(&partnerv1.OpenAccountResponse{
		Partner: p.Provider.Partner(), AccountId: acc.ID, Reference: acc.Reference,
	}), nil
}

// Payout sends a payment. The partner is pinned before the request leaves, and a failure that leaves the outcome unknown
// keeps the pin: the payment is never sent to another partner.
func (s *Service) Payout(ctx context.Context, req *connect.Request[partnerv1.PayoutRequest]) (*connect.Response[partnerv1.PayoutResponse], error) {
	m := req.Msg
	if m.PaymentId == "" || m.Asset == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_id and asset are required"))
	}
	var amount int64
	if _, err := fmt.Sscan(m.AmountMinor, &amount); err != nil || amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amount_minor must be a positive integer"))
	}
	chosen, err := s.choose(ctx, m.Asset)
	if err != nil {
		return nil, err
	}
	holder, err := store.Pin(ctx, s.Pool, m.PaymentId, chosen.Provider.Partner(), m.Asset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	p, ok := s.byName(holder)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the partner holding this payment is not configured"))
	}
	res, err := p.Provider.Payout(ctx, banklayer.PayRequest{
		ID: m.PaymentId, Asset: m.Asset, AmountMinor: amount, Beneficiary: m.Beneficiary,
		AccountNumber: m.AccountNumber, BankCode: m.BankCode, Reference: m.Reference,
	})
	if err != nil {
		if isFailure(err) {
			_ = store.Record(ctx, s.Pool, holder, m.Asset, false)
		}
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	_ = store.Record(ctx, s.Pool, holder, m.Asset, true)
	return connect.NewResponse(&partnerv1.PayoutResponse{Partner: holder, Status: res.Status}), nil
}

// PayoutStatus reads a payment's status from the partner that holds it.
func (s *Service) PayoutStatus(ctx context.Context, req *connect.Request[partnerv1.PayoutStatusRequest]) (*connect.Response[partnerv1.PayoutStatusResponse], error) {
	holder, err := store.PinnedPartner(ctx, s.Pool, req.Msg.PaymentId)
	if errors.Is(err, store.ErrNotPinned) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	p, ok := s.byName(holder)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the partner holding this payment is not configured"))
	}
	res, err := p.Provider.PayoutStatus(ctx, req.Msg.PaymentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&partnerv1.PayoutStatusResponse{Partner: holder, Status: res.Status}), nil
}

// GetHealth reports each partner's state, from its recent outcomes.
func (s *Service) GetHealth(ctx context.Context, _ *connect.Request[partnerv1.GetHealthRequest]) (*connect.Response[partnerv1.GetHealthResponse], error) {
	out := &partnerv1.GetHealthResponse{}
	for _, p := range s.Partners {
		w, err := store.Window(ctx, s.Pool, p.Provider.Partner(), windowSize)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out.Partners = append(out.Partners, &partnerv1.PartnerHealth{
			Partner: p.Provider.Partner(), Asset: p.Provider.Asset(), State: string(w.State()),
			ConsecutiveFailures: uint32(w.ConsecutiveFailures()), RecentOutcomes: uint32(len(w.Outcomes)),
			CostBps: uint32(p.CostBps),
		})
	}
	return connect.NewResponse(out), nil
}
