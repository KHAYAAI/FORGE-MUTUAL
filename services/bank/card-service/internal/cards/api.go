package cards

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	cardsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cards/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
)

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }

func notFound() error { return connect.NewError(connect.CodeNotFound, errors.New("no such card")) }

func wrap(err error) error {
	var ce *connect.Error
	if err == nil || errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("cards: %w", err))
}

const cardColumns = `card_id, party_id, account_id, label, last4, expiry_month, expiry_year, status, per_txn_limit, daily_limit, created_at, token`

func scanCard(row pgx.Row) (*cardsv1.Card, string, error) {
	var c cardsv1.Card
	var perTxn, daily int64
	var created time.Time
	var token string
	var m, y int
	if err := row.Scan(&c.CardId, &c.PartyId, &c.AccountId, &c.Label, &c.Last4, &m, &y, &c.Status, &perTxn, &daily, &created, &token); err != nil {
		return nil, "", err
	}
	c.ExpiryMonth, c.ExpiryYear = uint32(m), uint32(y)
	c.PerTransactionLimitMinor, c.DailyLimitMinor = strconv.FormatInt(perTxn, 10), strconv.FormatInt(daily, 10)
	c.CreatedAt = created.UTC().Format(time.RFC3339)
	c.Network = "REAL"
	return &c, token, nil
}

// owned loads a card only if the party owns it; anyone else gets "no such card", never "not yours".
func (s *Service) owned(ctx context.Context, party, id string) (*cardsv1.Card, string, error) {
	if party == "" || id == "" {
		return nil, "", invalid("party_id and card_id are required")
	}
	c, tok, err := scanCard(s.Pool.QueryRow(ctx, `select `+cardColumns+` from card where card_id = $1 and party_id = $2`, id, party))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", notFound()
	}
	if err != nil {
		return nil, "", wrap(err)
	}
	if s.Processor != nil && s.Processor.Simulated() {
		c.Network = "SIMULATED"
	}
	return c, tok, nil
}

func (s *Service) allowed() error {
	if err := s.Licence.Allow(licence.Cards); err != nil {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return nil
}

func (s *Service) IssueCard(ctx context.Context, r *connect.Request[cardsv1.IssueCardRequest]) (*connect.Response[cardsv1.IssueCardResponse], error) {
	m := r.Msg
	if err := s.allowed(); err != nil {
		return nil, err
	}
	label := strings.TrimSpace(m.Label)
	if m.IdempotencyKey == "" || m.PartyId == "" || m.AccountId == "" || len(label) == 0 || len(label) > 60 {
		return nil, invalid("idempotency_key, party_id, account_id and a label of up to 60 characters are required")
	}
	// A retry returns the card already issued.
	if c, _, err := scanCard(s.Pool.QueryRow(ctx, `select `+cardColumns+` from card where party_id = $1 and idempotency_key = $2`, m.PartyId, m.IdempotencyKey)); err == nil {
		if c.AccountId != m.AccountId {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("this key was used for a different request"))
		}
		return connect.NewResponse(&cardsv1.IssueCardResponse{Card: c}), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, wrap(err)
	}
	accts, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if asset, ok := accts[m.AccountId]; !ok || asset != "ZAR" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cards are issued on a rand account you own"))
	}
	if s.Parties != nil {
		ok, err := s.Parties.Active(ctx, m.PartyId)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		if !ok {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this action is not available on your account; please contact us"))
		}
	}
	var open int
	if err := s.Pool.QueryRow(ctx, `select count(*) from card where party_id = $1 and status <> 'CLOSED'`, m.PartyId).Scan(&open); err != nil {
		return nil, wrap(err)
	}
	if open >= s.MaxCardsPerParty {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("you can have up to %d open cards", s.MaxCardsPerParty))
	}
	pc, err := s.Processor.CreateCard(ctx, m.PartyId+":"+m.IdempotencyKey, label)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	id := "card-" + strings.TrimPrefix(pc.Token, "tok_")[:16]
	if _, err := s.Pool.Exec(ctx, `insert into card (card_id, party_id, account_id, idempotency_key, token, last4, expiry_month, expiry_year, label, status, per_txn_limit, daily_limit, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ACTIVE',$10,$11,$12) on conflict (party_id, idempotency_key) do nothing`,
		id, m.PartyId, m.AccountId, m.IdempotencyKey, pc.Token, pc.Last4, pc.ExpiryMonth, pc.ExpiryYear, label, s.DefaultPerTxn, s.DefaultDaily, s.now()); err != nil {
		return nil, wrap(err)
	}
	c, _, err := scanCard(s.Pool.QueryRow(ctx, `select `+cardColumns+` from card where party_id = $1 and idempotency_key = $2`, m.PartyId, m.IdempotencyKey))
	if err != nil {
		return nil, wrap(err)
	}
	if s.Processor.Simulated() {
		c.Network = "SIMULATED"
	}
	return connect.NewResponse(&cardsv1.IssueCardResponse{Card: c}), nil
}

func (s *Service) ListCards(ctx context.Context, r *connect.Request[cardsv1.ListCardsRequest]) (*connect.Response[cardsv1.ListCardsResponse], error) {
	if r.Msg.PartyId == "" {
		return nil, invalid("party_id is required")
	}
	rows, err := s.Pool.Query(ctx, `select `+cardColumns+` from card where party_id = $1 order by created_at desc`, r.Msg.PartyId)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &cardsv1.ListCardsResponse{}
	for rows.Next() {
		c, _, err := scanCard(rows)
		if err != nil {
			return nil, wrap(err)
		}
		if s.Processor.Simulated() {
			c.Network = "SIMULATED"
		}
		out.Cards = append(out.Cards, c)
	}
	return connect.NewResponse(out), wrap(rows.Err())
}

func (s *Service) GetCard(ctx context.Context, r *connect.Request[cardsv1.GetCardRequest]) (*connect.Response[cardsv1.GetCardResponse], error) {
	c, _, err := s.owned(ctx, r.Msg.PartyId, r.Msg.CardId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&cardsv1.GetCardResponse{Card: c}), nil
}

// SetCardStatus: ACTIVE and FROZEN swap freely; CLOSED is final and also closes the card at the processor.
func (s *Service) SetCardStatus(ctx context.Context, r *connect.Request[cardsv1.SetCardStatusRequest]) (*connect.Response[cardsv1.SetCardStatusResponse], error) {
	c, token, err := s.owned(ctx, r.Msg.PartyId, r.Msg.CardId)
	if err != nil {
		return nil, err
	}
	to := strings.ToUpper(r.Msg.Status)
	if to != "ACTIVE" && to != "FROZEN" && to != "CLOSED" {
		return nil, invalid("status must be ACTIVE, FROZEN or CLOSED")
	}
	if c.Status == "CLOSED" && to != "CLOSED" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("a closed card cannot be used again; ask for a new one"))
	}
	if to == "ACTIVE" {
		if err := s.allowed(); err != nil {
			return nil, err
		}
		if s.Parties != nil {
			ok, err := s.Parties.Active(ctx, r.Msg.PartyId)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnavailable, err)
			}
			if !ok {
				return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this action is not available on your account; please contact us"))
			}
		}
	}
	if to == "CLOSED" {
		if err := s.Processor.CloseCard(ctx, token); err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
	}
	if _, err := s.Pool.Exec(ctx, `update card set status = $2 where card_id = $1 and status <> 'CLOSED'`, c.CardId, to); err != nil {
		return nil, wrap(err)
	}
	c, _, err = s.owned(ctx, r.Msg.PartyId, r.Msg.CardId)
	return connect.NewResponse(&cardsv1.SetCardStatusResponse{Card: c}), err
}

func (s *Service) SetLimits(ctx context.Context, r *connect.Request[cardsv1.SetLimitsRequest]) (*connect.Response[cardsv1.SetLimitsResponse], error) {
	c, _, err := s.owned(ctx, r.Msg.PartyId, r.Msg.CardId)
	if err != nil {
		return nil, err
	}
	if c.Status == "CLOSED" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this card is closed"))
	}
	perTxn, e1 := strconv.ParseInt(r.Msg.PerTransactionLimitMinor, 10, 64)
	daily, e2 := strconv.ParseInt(r.Msg.DailyLimitMinor, 10, 64)
	if e1 != nil || e2 != nil || perTxn <= 0 || daily < perTxn {
		return nil, invalid("limits must be positive amounts in cents, with the daily limit at least the per-transaction limit")
	}
	if perTxn > s.MaxPerTxn || daily > s.MaxDaily {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("limits above %d per transaction or %d a day need us to arrange them; please contact us", s.MaxPerTxn, s.MaxDaily))
	}
	if _, err := s.Pool.Exec(ctx, `update card set per_txn_limit = $2, daily_limit = $3 where card_id = $1`, c.CardId, perTxn, daily); err != nil {
		return nil, wrap(err)
	}
	c, _, err = s.owned(ctx, r.Msg.PartyId, r.Msg.CardId)
	return connect.NewResponse(&cardsv1.SetLimitsResponse{Card: c}), err
}

func (s *Service) ListAuthorisations(ctx context.Context, r *connect.Request[cardsv1.ListAuthorisationsRequest]) (*connect.Response[cardsv1.ListAuthorisationsResponse], error) {
	if _, _, err := s.owned(ctx, r.Msg.PartyId, r.Msg.CardId); err != nil {
		return nil, err
	}
	limit := int(r.Msg.Limit)
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return nil, invalid("limit must be 1 to 200")
	}
	rows, err := s.Pool.Query(ctx, `select auth_id, card_id, merchant, mcc, country, currency, amount_minor, status, decline_reason, captured_minor, refunded_minor, created_at
		from card_auth where card_id = $1 order by created_at desc, auth_id limit $2`, r.Msg.CardId, limit)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &cardsv1.ListAuthorisationsResponse{}
	for rows.Next() {
		var a cardsv1.Authorisation
		var amt, cap, ref int64
		var at time.Time
		if err := rows.Scan(&a.AuthId, &a.CardId, &a.MerchantName, &a.Mcc, &a.Country, &a.Currency, &amt, &a.Status, &a.DeclineReason, &cap, &ref, &at); err != nil {
			return nil, wrap(err)
		}
		a.AmountMinor, a.CapturedMinor, a.RefundedMinor = strconv.FormatInt(amt, 10), strconv.FormatInt(cap, 10), strconv.FormatInt(ref, 10)
		a.At = at.UTC().Format(time.RFC3339)
		if a.Status == "PENDING" {
			a.Status = "HELD"
		}
		out.Authorisations = append(out.Authorisations, &a)
	}
	return connect.NewResponse(out), wrap(rows.Err())
}

func (s *Service) DisplayCard(ctx context.Context, r *connect.Request[cardsv1.DisplayCardRequest]) (*connect.Response[cardsv1.DisplayCardResponse], error) {
	c, token, err := s.owned(ctx, r.Msg.PartyId, r.Msg.CardId)
	if err != nil {
		return nil, err
	}
	if c.Status != "ACTIVE" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("details can be shown only for an active card"))
	}
	url, exp, err := s.Processor.DisplayURL(ctx, token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&cardsv1.DisplayCardResponse{Url: url, ExpiresAt: exp.UTC().Format(time.RFC3339)}), nil
}
