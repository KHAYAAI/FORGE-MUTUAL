package inbound

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
)

// Reason is one reason a customer may give for a foreign payment they received. The codes are Forge's own. Each maps
// to a SARB balance-of-payments category, but the mapping, like the reporting format, is not yet confirmed with the
// authorised dealer (VERIFY-FINSURV), so no official category code is claimed here.
type Reason struct {
	Code  string
	Label string
}

// Reasons are the reasons offered, in the order shown. They cover what a technology business usually receives.
var Reasons = []Reason{
	{"services_software", "Payment for software or IT services we provided"},
	{"services_other", "Payment for other services we provided"},
	{"goods_export", "Payment for goods we exported"},
	{"royalties_licences", "Royalties or licence fees for our software or IP"},
	{"investment_equity", "Investment into our company (shares)"},
	{"loan_received", "A loan to us from abroad"},
	{"loan_repayment", "Repayment of a loan we made abroad"},
	{"own_funds_transfer", "Our own money moved from an account abroad"},
	{"salary_contract", "Salary or contract income for work done"},
	{"refund", "A refund of a payment we made abroad"},
	{"other", "Something else (describe it in the note)"},
}

func validReason(code string) bool {
	for _, r := range Reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

// CrossBorder serves the cross-border credits of a party (paymentsv1connect.CrossBorderServiceHandler).
type CrossBorder struct {
	Pool *pgxpool.Pool
	Now  func() time.Time
}

const creditColumns = `asset, movement_id, account_id, amount_minor, reference, booked_at, status, reason_code, reason_note,
	classified_by, classified_at`

func scanCredit(row pgx.Row) (*paymentsv1.CrossBorderCredit, error) {
	var (
		asset, movement, account, ref, status, code, note, by string
		amount                                                int64
		booked                                                time.Time
		classified                                            *time.Time
	)
	if err := row.Scan(&asset, &movement, &account, &amount, &ref, &booked, &status, &code, &note, &by, &classified); err != nil {
		return nil, err
	}
	c := &paymentsv1.CrossBorderCredit{
		CreditId: asset + ":" + movement, AccountId: account, Asset: asset, AmountMinor: strconv.FormatInt(amount, 10),
		Reference: ref, BookedAt: booked.UTC().Format(time.RFC3339), Status: status, ReasonCode: code, ReasonNote: note,
		ClassifiedBy: by,
	}
	if classified != nil {
		c.ClassifiedAt = classified.UTC().Format(time.RFC3339)
	}
	return c, nil
}

// ListCrossBorderCredits implements the handler. Only the party's own credits are returned.
func (c *CrossBorder) ListCrossBorderCredits(ctx context.Context, req *connect.Request[paymentsv1.ListCrossBorderCreditsRequest]) (*connect.Response[paymentsv1.ListCrossBorderCreditsResponse], error) {
	if req.Msg.PartyId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("party_id is required"))
	}
	rows, err := c.Pool.Query(ctx, `select `+creditColumns+` from cross_border_credit where party_id = $1
		order by booked_at desc, movement_id limit 200`, req.Msg.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	defer rows.Close()
	out := &paymentsv1.ListCrossBorderCreditsResponse{}
	for rows.Next() {
		cr, err := scanCredit(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		out.Credits = append(out.Credits, cr)
	}
	return connect.NewResponse(out), rows.Err()
}

// ClassifyCrossBorderCredit records the reason for a credit. It is refused for another party's credit (as not found,
// so a guess learns nothing), for an unknown reason, and for a credit that already has one. Sending the same reason
// again returns the credit unchanged.
func (c *CrossBorder) ClassifyCrossBorderCredit(ctx context.Context, req *connect.Request[paymentsv1.ClassifyCrossBorderCreditRequest]) (*connect.Response[paymentsv1.ClassifyCrossBorderCreditResponse], error) {
	m := req.Msg
	asset, movement, ok := strings.Cut(m.CreditId, ":")
	switch {
	case m.PartyId == "" || m.Actor == "":
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("party_id and actor are required"))
	case !ok || asset == "" || movement == "":
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("credit_id must be <asset>:<movement>"))
	case !validReason(m.ReasonCode):
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason_code is not one of the listed reasons"))
	case len(m.Note) > 200:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("note must be at most 200 characters"))
	case m.ReasonCode == "other" && strings.TrimSpace(m.Note) == "":
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("describe the payment in the note when the reason is other"))
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	var out *paymentsv1.CrossBorderCredit
	err := pgx.BeginFunc(ctx, c.Pool, func(tx pgx.Tx) error {
		cr, err := scanCredit(tx.QueryRow(ctx, `select `+creditColumns+` from cross_border_credit
			where asset = $1 and movement_id = $2 and party_id = $3 for update`, asset, movement, m.PartyId))
		if errors.Is(err, pgx.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("no such credit"))
		}
		if err != nil {
			return err
		}
		if cr.Status == "CLASSIFIED" {
			if cr.ReasonCode == m.ReasonCode && cr.ReasonNote == strings.TrimSpace(m.Note) {
				out = cr
				return nil
			}
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("this credit already has a reason; ask operations to correct it"))
		}
		out, err = scanCredit(tx.QueryRow(ctx, `update cross_border_credit
			set status = 'CLASSIFIED', reason_code = $4, reason_note = $5, classified_by = $6, classified_at = $7
			where asset = $1 and movement_id = $2 and party_id = $3 returning `+creditColumns,
			asset, movement, m.PartyId, m.ReasonCode, strings.TrimSpace(m.Note), m.Actor, now().UTC()))
		return err
	})
	var ce *connect.Error
	if errors.As(err, &ce) {
		return nil, ce
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("classify: %w", err))
	}
	return connect.NewResponse(&paymentsv1.ClassifyCrossBorderCreditResponse{Credit: out}), nil
}

// ListPaymentReasons implements the handler.
func (c *CrossBorder) ListPaymentReasons(context.Context, *connect.Request[paymentsv1.ListPaymentReasonsRequest]) (*connect.Response[paymentsv1.ListPaymentReasonsResponse], error) {
	out := &paymentsv1.ListPaymentReasonsResponse{}
	for _, r := range Reasons {
		out.Reasons = append(out.Reasons, &paymentsv1.PaymentReason{Code: r.Code, Label: r.Label})
	}
	return connect.NewResponse(out), nil
}
