// Package service is the ConnectRPC surface of the FX service. A quote is priced from a fresh market rate and disclosed.
// A booking checks that the customer owns both accounts, locks the quote, and runs the saga.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	fxv1 "github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/pricing"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/rates"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/saga"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/store"
)

// QuoteValidity is how long a quote can be booked.
const QuoteValidity = 60 * time.Second

// Ownership reports the accounts a party owns, and their assets.
type Ownership interface {
	Accounts(ctx context.Context, party string) (map[string]string, error)
}

// Service implements fxv1connect.FXServiceHandler.
type Service struct {
	Pool      *pgxpool.Pool
	Rates     rates.Source
	MaxAge    time.Duration // the freshness limit for the rate
	SpreadBps int64
	Owners    Ownership
	Saga      *saga.Saga
	Now       func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Quote prices a conversion from a fresh rate and returns it with its spread disclosed.
func (s *Service) Quote(ctx context.Context, req *connect.Request[fxv1.QuoteRequest]) (*connect.Response[fxv1.QuoteResponse], error) {
	m := req.Msg
	if m.PartyId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("party_id is required"))
	}
	var sell int64
	if _, err := fmt.Sscan(m.SellAmountMinor, &sell); err != nil || sell <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sell_amount_minor must be a positive integer"))
	}
	mid, asOf, err := s.Rates.Mid(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	now := s.now()
	if err := rates.Fresh(asOf, now, s.MaxAge); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	q, err := pricing.Price(m.SellAsset, m.BuyAsset, sell, mid, s.SpreadBps)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	id, err := uuid.NewV4()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	expires := now.Add(QuoteValidity)
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		return store.SaveQuote(ctx, tx, store.Quote{QuoteID: id, PartyID: m.PartyId, SellAsset: m.SellAsset, BuyAsset: m.BuyAsset,
			SellAmount: sell, BuyAmount: q.BuyAmount, Mid: mid, Rate: q.RateScaled, SpreadBps: s.SpreadBps,
			SpreadMinor: q.SpreadMinor, ExpiresAt: expires})
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fxv1.QuoteResponse{
		QuoteId: id, SellAsset: m.SellAsset, BuyAsset: m.BuyAsset, SellAmountMinor: m.SellAmountMinor,
		BuyAmountMinor: fmt.Sprint(q.BuyAmount), RateScaled: fmt.Sprint(q.RateScaled), MidRateScaled: fmt.Sprint(mid),
		SpreadBps: uint32(s.SpreadBps), SpreadMinor: fmt.Sprint(q.SpreadMinor),
		ExpiresAt: expires.UTC().Format(time.RFC3339Nano),
		Disclosure: fmt.Sprintf("The rate includes a spread of %d basis points, which is %d %s minor units on this conversion. "+
			"The quote is valid until %s.", s.SpreadBps, q.SpreadMinor, m.SellAsset, expires.UTC().Format("15:04:05 UTC")),
	}), nil
}

// Book locks a quote and runs the conversion. The customer must own both accounts, in the right assets.
func (s *Service) Book(ctx context.Context, req *connect.Request[fxv1.BookRequest]) (*connect.Response[fxv1.BookResponse], error) {
	m := req.Msg
	if m.PartyId == "" || m.QuoteId == "" || m.SellAccountId == "" || m.BuyAccountId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("party, quote and both accounts are required"))
	}
	owned, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	q, err := s.quoteFor(ctx, m.PartyId, m.QuoteId)
	if err != nil {
		return nil, err
	}
	if owned[m.SellAccountId] != q.SellAsset || owned[m.BuyAccountId] != q.BuyAsset {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("both accounts must belong to the party, in the quote's assets"))
	}
	dealID, err := uuid.NewV4()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if _, err := store.LockQuote(ctx, s.Pool, m.PartyId, m.QuoteId, dealID, m.SellAccountId, m.BuyAccountId, s.now()); err != nil {
		return nil, bookingError(err)
	}
	state, err := s.Saga.Run(ctx, dealID)
	if err != nil {
		// The deal is recorded and the resume loop, or another worker holding it, will finish it. The caller sees the
		// state the deal has reached.
		d, derr := store.GetDealByID(ctx, s.Pool, dealID)
		if derr != nil {
			return nil, bookingError(derr)
		}
		state = d.State
	}
	return connect.NewResponse(&fxv1.BookResponse{DealId: dealID, State: state}), nil
}

// GetDeal returns a deal the party holds.
func (s *Service) GetDeal(ctx context.Context, req *connect.Request[fxv1.GetDealRequest]) (*connect.Response[fxv1.GetDealResponse], error) {
	d, err := store.GetDeal(ctx, s.Pool, req.Msg.PartyId, req.Msg.DealId)
	if errors.Is(err, store.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("deal not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fxv1.GetDealResponse{
		DealId: d.DealID, State: d.State, Reason: d.Reason, SellAsset: d.SellAsset, BuyAsset: d.BuyAsset,
		SellAmountMinor: fmt.Sprint(d.SellAmount), BuyAmountMinor: fmt.Sprint(d.BuyAmount),
	}), nil
}

func (s *Service) quoteFor(ctx context.Context, party, quoteID string) (store.Quote, error) {
	var q store.Quote
	err := s.Pool.QueryRow(ctx, `select sell_asset, buy_asset from fx_quote where quote_id = $1 and party_id = $2`,
		quoteID, party).Scan(&q.SellAsset, &q.BuyAsset)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Quote{}, connect.NewError(connect.CodeNotFound, errors.New("quote not found"))
	}
	if err != nil {
		return store.Quote{}, connect.NewError(connect.CodeInternal, err)
	}
	return q, nil
}

func bookingError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("quote not found"))
	case errors.Is(err, store.ErrNotBookable):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
