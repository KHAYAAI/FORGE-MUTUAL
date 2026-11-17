package lending

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/services/bank/lending-service/internal/credit"
)

func dayOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Collect takes every instalment due on or before today from the customer's current account: principal to the loan
// account and interest to interest income, as one chain. An instalment the account cannot cover goes into arrears and
// is tried again on the next run; the facility records how many days its oldest unpaid instalment is overdue. The key
// is the instalment, so a retry after a crash collects once.
func (s *Service) Collect(ctx context.Context, today time.Time) (paid, arrears int, err error) {
	today = dayOf(today)
	rows, err := s.Pool.Query(ctx, `select i.facility_id, i.number, i.principal_minor, i.interest_minor, f.current_account_id, f.loan_account_id
		from instalment i join facility f using (facility_id)
		where f.status = 'ACTIVE' and i.status in ('DUE', 'ARREARS') and i.due_on <= $1 order by i.due_on, i.facility_id, i.number`, today)
	if err != nil {
		return 0, 0, err
	}
	type due struct {
		facility, current, loan string
		number                  int32
		principal, interest     int64
	}
	var todo []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.facility, &d.number, &d.principal, &d.interest, &d.current, &d.loan); err != nil {
			rows.Close()
			return 0, 0, err
		}
		todo = append(todo, d)
	}
	rows.Close()
	for _, d := range todo {
		var legs []*ledgerv1.TransferLeg
		if d.principal > 0 {
			legs = append(legs, leg(d.current, d.loan, d.principal, codeRepayment))
		}
		if d.interest > 0 {
			legs = append(legs, leg(d.current, s.Income, d.interest, codeInterestCharge))
		}
		status := "PAID"
		if len(legs) > 0 {
			if err := s.transfer(ctx, fmt.Sprintf("instalment:%s:%d", d.facility, d.number), "lending-service", "instalment", legs...); err != nil {
				if connect.CodeOf(err) != connect.CodeFailedPrecondition {
					return paid, arrears, err
				}
				status = "ARREARS"
			}
		}
		if _, err := s.Pool.Exec(ctx, `update instalment set status = $3 where facility_id = $1 and number = $2`, d.facility, d.number, status); err != nil {
			return paid, arrears, err
		}
		if status == "PAID" {
			paid++
		} else {
			arrears++
		}
	}
	// Days past due, and loans with nothing left to pay are settled.
	if _, err := s.Pool.Exec(ctx, `update facility f set days_past_due = coalesce((
		select ($1::date - min(i.due_on)) from instalment i where i.facility_id = f.facility_id and i.status = 'ARREARS'), 0)
		where f.status = 'ACTIVE' and f.product_id = 'term-loan'`, today); err != nil {
		return paid, arrears, err
	}
	_, err = s.Pool.Exec(ctx, `update facility f set status = 'SETTLED' where f.status = 'ACTIVE' and f.product_id = 'term-loan'
		and not exists (select 1 from instalment i where i.facility_id = f.facility_id and i.status in ('DUE', 'ARREARS'))`)
	return paid, arrears, err
}

// AccrueOverdrafts records a day's interest on each active overdraft's drawn balance (ACT/365, nano-cents).
func (s *Service) AccrueOverdrafts(ctx context.Context, day time.Time) error {
	day = dayOf(day)
	rows, err := s.Pool.Query(ctx, `select facility_id, loan_account_id, rate_bps from facility f
		where status = 'ACTIVE' and product_id = 'overdraft' and accepted_at < $1
		and not exists (select 1 from od_accrual a where a.facility_id = f.facility_id and a.day = $2)`, day.AddDate(0, 0, 1), day)
	if err != nil {
		return err
	}
	type od struct {
		id, loan string
		rate     int32
	}
	var todo []od
	for rows.Next() {
		var o od
		if err := rows.Scan(&o.id, &o.loan, &o.rate); err != nil {
			rows.Close()
			return err
		}
		todo = append(todo, o)
	}
	rows.Close()
	for _, o := range todo {
		bal, err := s.balance(ctx, o.loan)
		if err != nil {
			return err
		}
		drawn := max(-bal, 0)
		if _, err := s.Pool.Exec(ctx, `insert into od_accrual (facility_id, day, drawn_minor, accrued_nano) values ($1, $2, $3, $4)
			on conflict do nothing`, o.id, day, drawn, credit.DailyNano(drawn, uint32(o.rate))); err != nil {
			return err
		}
	}
	return nil
}

// ChargeOverdrafts charges a month's overdraft interest (period YYYY-MM) by adding it to what is drawn, once per
// facility and month; the fraction of a cent left over carries.
func (s *Service) ChargeOverdrafts(ctx context.Context, period string) error {
	if _, err := s.Pool.Exec(ctx, `
		insert into od_charge (facility_id, period, accrued_nano, carry_in_nano, charged_minor, carry_out_nano, status)
		select a.facility_id, $1, sum(a.accrued_nano),
		       coalesce((select c.carry_out_nano from od_charge c where c.facility_id = a.facility_id and c.period < $1 order by c.period desc limit 1), 0),
		       0, 0, 'PENDING'
		from od_accrual a where to_char(a.day, 'YYYY-MM') = $1 group by a.facility_id
		on conflict do nothing`, period); err != nil {
		return err
	}
	if _, err := s.Pool.Exec(ctx, `update od_charge set charged_minor = (accrued_nano + carry_in_nano) / 1000000000,
		carry_out_nano = (accrued_nano + carry_in_nano) % 1000000000 where period = $1 and status = 'PENDING'`, period); err != nil {
		return err
	}
	rows, err := s.Pool.Query(ctx, `select c.facility_id, c.charged_minor, f.loan_account_id from od_charge c join facility f using (facility_id)
		where c.period = $1 and c.status = 'PENDING'`, period)
	if err != nil {
		return err
	}
	type ch struct {
		id, loan string
		amount   int64
	}
	var todo []ch
	for rows.Next() {
		var c ch
		if err := rows.Scan(&c.id, &c.amount, &c.loan); err != nil {
			rows.Close()
			return err
		}
		todo = append(todo, c)
	}
	rows.Close()
	for _, c := range todo {
		if c.amount > 0 {
			if err := s.transfer(ctx, "od-interest:"+c.id+":"+period, "lending-service", "overdraft interest "+period, leg(c.loan, s.Income, c.amount, codeInterestCharge)); err != nil {
				return err
			}
		}
		if _, err := s.Pool.Exec(ctx, `update od_charge set status = 'CHARGED' where facility_id = $1 and period = $2`, c.id, period); err != nil {
			return err
		}
	}
	return nil
}

// Sweep repays each revenue-based advance with its share of the money received since the last sweep. Each payment in
// is swept once: the cursor moves only after the sweep for it is posted, and the key is the payment's sequence.
func (s *Service) Sweep(ctx context.Context) (int, error) {
	rows, err := s.Pool.Query(ctx, `select facility_id, party_id, current_account_id, loan_account_id, sweep_bps, fee_owed_minor, sweep_after_seq
		from facility where status = 'ACTIVE' and product_id = 'revenue-based'`)
	if err != nil {
		return 0, err
	}
	type rbf struct {
		id, party, current, loan string
		bps                      int32
		feeOwed, after           int64
	}
	var todo []rbf
	for rows.Next() {
		var r rbf
		if err := rows.Scan(&r.id, &r.party, &r.current, &r.loan, &r.bps, &r.feeOwed, &r.after); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, r)
	}
	rows.Close()
	swept := 0
	for _, r := range todo {
		res, err := s.Activity.GetActivity(ctx, connect.NewRequest(&consolidatedv1.GetActivityRequest{PartyId: r.party, AccountId: r.current, AfterSeq: uint64(r.after), Limit: 500}))
		if err != nil {
			return swept, err
		}
		for _, e := range res.Msg.Entries {
			if e.Direction == "IN" && InflowCodes[e.Code] {
				received, _ := strconv.ParseInt(e.AmountMinor, 10, 64)
				bal, err := s.balance(ctx, r.loan)
				if err != nil {
					return swept, err
				}
				pp, fp := credit.Sweep(received, uint32(r.bps), -bal, r.feeOwed)
				var legs []*ledgerv1.TransferLeg
				if pp > 0 {
					legs = append(legs, leg(r.current, r.loan, pp, codeRepayment))
				}
				if fp > 0 {
					legs = append(legs, leg(r.current, s.Income, fp, codeInterestCharge))
				}
				if len(legs) > 0 {
					if err := s.transfer(ctx, fmt.Sprintf("sweep:%s:%d", r.id, e.Seq), "lending-service", "revenue share", legs...); err != nil {
						// The money was spent before the sweep ran: stop here and try this payment again next time.
						if connect.CodeOf(err) == connect.CodeFailedPrecondition {
							break
						}
						return swept, err
					}
					swept++
				}
				r.feeOwed -= fp
				if _, err := s.Pool.Exec(ctx, `update facility set fee_owed_minor = $2, sweep_after_seq = $3 where facility_id = $1`, r.id, r.feeOwed, int64(e.Seq)); err != nil {
					return swept, err
				}
				if -bal-pp == 0 && r.feeOwed == 0 {
					if _, err := s.Pool.Exec(ctx, `update facility set status = 'SETTLED' where facility_id = $1`, r.id); err != nil {
						return swept, err
					}
					break
				}
				continue
			}
			if _, err := s.Pool.Exec(ctx, `update facility set sweep_after_seq = $2 where facility_id = $1`, r.id, int64(e.Seq)); err != nil {
				return swept, err
			}
		}
	}
	return swept, nil
}

// RunDaily is the daily work. Every step does nothing the second time.
func (s *Service) RunDaily(ctx context.Context) error {
	now := s.now()
	today := dayOf(now)
	for back := 7; back >= 1; back-- {
		if err := s.AccrueOverdrafts(ctx, today.AddDate(0, 0, -back)); err != nil {
			return fmt.Errorf("overdraft accrual: %w", err)
		}
	}
	prev := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01")
	if err := s.ChargeOverdrafts(ctx, prev); err != nil {
		return fmt.Errorf("overdraft interest: %w", err)
	}
	if _, _, err := s.Collect(ctx, now); err != nil {
		return fmt.Errorf("instalments: %w", err)
	}
	if _, err := s.Sweep(ctx); err != nil {
		return fmt.Errorf("sweep: %w", err)
	}
	return nil
}
