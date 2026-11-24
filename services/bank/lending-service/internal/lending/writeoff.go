package lending

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
)

// DefaultWriteOffAfterDays is how long a facility must have been past due before it may be written off. PROPOSED (threshold 31).
const DefaultWriteOffAfterDays = 90

// WriteOffFacility writes off a facility's outstanding balance: the loan account is credited and WRITE_OFF debited, so the
// balance sheet stops carrying it as an asset. The debt is still owed in law. It needs: an active facility at least
// WriteOffAfterDays past due, a default notice already issued (nothing is written off that the customer was never warned
// about), and two different people, neither of whom is the applicant. Doing it again returns the same result.
func (s *Service) WriteOffFacility(ctx context.Context, req *connect.Request[lendingv1.WriteOffFacilityRequest]) (*connect.Response[lendingv1.WriteOffFacilityResponse], error) {
	m := req.Msg
	reason := strings.TrimSpace(m.Reason)
	if m.FacilityId == "" || strings.TrimSpace(m.Officer) == "" || strings.TrimSpace(m.Approver) == "" || len(reason) < 10 || len(reason) > 1000 {
		return nil, invalid("facility_id, officer, approver and a reason of 10 to 1000 characters are required")
	}
	if strings.EqualFold(strings.TrimSpace(m.Officer), strings.TrimSpace(m.Approver)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("a write-off needs two different people"))
	}
	after := s.WriteOffAfterDays
	if after <= 0 {
		after = DefaultWriteOffAfterDays
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var status, appliedBy, loan string
		var dpd int
		err := tx.QueryRow(ctx, `select status, applied_by, loan_account_id, days_past_due from facility where facility_id = $1 for update`, m.FacilityId).
			Scan(&status, &appliedBy, &loan, &dpd)
		if errors.Is(err, pgx.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("no such facility"))
		}
		if err != nil {
			return err
		}
		if status == "WRITTEN_OFF" {
			return nil
		}
		if status != "ACTIVE" {
			return refused("only an active facility can be written off")
		}
		for _, who := range []string{m.Officer, m.Approver} {
			if strings.EqualFold(strings.TrimSpace(who), appliedBy) {
				return connect.NewError(connect.CodePermissionDenied, errors.New("the applicant cannot take part in writing off their own facility"))
			}
		}
		if dpd < after {
			return refused("this facility is not yet far enough past due to be written off")
		}
		var notices int
		if err := tx.QueryRow(ctx, `select count(*) from default_notice where facility_id = $1`, m.FacilityId).Scan(&notices); err != nil {
			return err
		}
		if notices == 0 {
			return refused("no default notice has been issued for this facility")
		}
		bal, err := s.balance(ctx, loan)
		if err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
		owed := -bal
		if owed <= 0 {
			return refused("nothing is owed on this facility")
		}
		key := "lending:writeoff:" + m.FacilityId
		if err := s.transfer(ctx, key, m.Approver, "write-off: "+reason, leg(s.WriteOff, loan, owed, codeWriteOff)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `update facility set status = 'WRITTEN_OFF', written_off_minor = $2, written_off_by = $3, written_off_at = $4, write_off_reason = $5
			where facility_id = $1`, m.FacilityId, owed, m.Officer+" and "+m.Approver, s.now(), reason); err != nil {
			return err
		}
		return s.event(ctx, tx, key, m.FacilityId, "WRITTEN_OFF", owed, 0, "Written off: still owed, no longer carried as an asset")
	})
	if err != nil {
		return nil, wrap(err)
	}
	f, err := s.facility(ctx, "", m.FacilityId)
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&lendingv1.WriteOffFacilityResponse{Facility: f}), nil
}
