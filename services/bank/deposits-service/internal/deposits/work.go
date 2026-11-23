package deposits

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/services/bank/deposits-service/internal/interest"
)

// EnsureSystemAccounts creates (or finds) the INTEREST_EXPENSE account interest is paid from.
func (s *Service) EnsureSystemAccounts(ctx context.Context) error {
	id, err := systemaccounts.Ensure(ctx, s.Ledger, s.Entity, "ZAR", 2007, "system:interest-expense:ZAR:v1")
	if err != nil {
		return err
	}
	s.Expense = id
	return nil
}

func dayOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Accrue records a day's interest for every account open before the end of that day, on its balance now. It is run
// just after midnight for the day that ended; running it again changes nothing. An account the run missed (the
// service was down) is accrued late, on the balance when it ran, and marked late.
func (s *Service) Accrue(ctx context.Context, day time.Time) (int, error) {
	day = dayOf(day)
	late := s.now().UTC().Sub(day.AddDate(0, 0, 1)) > 26*time.Hour
	rows, err := s.Pool.Query(ctx, `select account_id, product_id from deposit_account d
		where opened_at < $1 and not exists (select 1 from accrual a where a.account_id = d.account_id and a.day = $2)
		order by account_id`, day.AddDate(0, 0, 1), day)
	if err != nil {
		return 0, err
	}
	type acc struct{ id, product string }
	var todo []acc
	for rows.Next() {
		var a acc
		if err := rows.Scan(&a.id, &a.product); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, a)
	}
	rows.Close()
	done := 0
	for start := 0; start < len(todo); start += 500 {
		end := min(start+500, len(todo))
		ids := make([]string, 0, end-start)
		for _, a := range todo[start:end] {
			ids = append(ids, a.id)
		}
		bal, err := s.balances(ctx, ids)
		if err != nil {
			return done, err
		}
		for _, a := range todo[start:end] {
			p, ok := s.product(a.product)
			if !ok {
				return done, fmt.Errorf("account %s: unknown product %q", a.id, a.product)
			}
			b := max(bal[a.id], 0)
			nano, err := interest.Daily(b, p.RateBps)
			if err != nil {
				return done, err
			}
			if _, err := s.Pool.Exec(ctx, `insert into accrual (account_id, day, balance_minor, rate_bps, accrued_nano, late)
				values ($1, $2, $3, $4, $5, $6) on conflict do nothing`, a.id, day, b, p.RateBps, nano, late); err != nil {
				return done, err
			}
			done++
		}
	}
	return done, nil
}

// PayInterest pays a month's interest (period YYYY-MM) into every account that accrued in it. The payment is recorded
// PENDING before the ledger is asked, under a key of account and month, so a crash is finished with the same key and
// pays once. What rounds away is carried to the next month.
func (s *Service) PayInterest(ctx context.Context, period string) (int, error) {
	if _, err := time.Parse("2006-01", period); err != nil {
		return 0, fmt.Errorf("period %q: %w", period, err)
	}
	if s.Expense == "" {
		return 0, errors.New("deposits: EnsureSystemAccounts has not run")
	}
	if _, err := s.Pool.Exec(ctx, `
		insert into interest_payment (account_id, period, accrued_nano, carry_in_nano, paid_minor, carry_out_nano, status)
		select a.account_id, $1, sum(a.accrued_nano),
		       coalesce((select p.carry_out_nano from interest_payment p where p.account_id = a.account_id and p.period < $1
		                 order by p.period desc limit 1), 0),
		       0, 0, 'PENDING'
		from accrual a where to_char(a.day, 'YYYY-MM') = $1 group by a.account_id
		on conflict do nothing`, period); err != nil {
		return 0, err
	}
	if _, err := s.Pool.Exec(ctx, `update interest_payment
		set paid_minor = (accrued_nano + carry_in_nano) / 1000000000, carry_out_nano = (accrued_nano + carry_in_nano) % 1000000000
		where period = $1 and status = 'PENDING'`, period); err != nil {
		return 0, err
	}
	rows, err := s.Pool.Query(ctx, `select account_id, paid_minor from interest_payment where period = $1 and status = 'PENDING' order by account_id`, period)
	if err != nil {
		return 0, err
	}
	type due struct {
		id     string
		amount int64
	}
	var todo []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.amount); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, d)
	}
	rows.Close()
	paid := 0
	for _, d := range todo {
		if d.amount > 0 {
			if err := s.transfer(ctx, "interest:"+d.id+":"+period, s.Expense, d.id, d.amount, codeInterestCredit, "deposits-service", "interest "+period); err != nil {
				return paid, err
			}
		}
		if _, err := s.Pool.Exec(ctx, `update interest_payment set status = 'PAID' where account_id = $1 and period = $2`, d.id, period); err != nil {
			return paid, err
		}
		paid++
	}
	return paid, nil
}

// PayNotices pays every notice due on or before today. A notice the ledger refuses (the money is no longer there,
// which the service's own checks should make impossible) is marked FAILED and left for operations.
func (s *Service) PayNotices(ctx context.Context, today time.Time) (paid, failed int, err error) {
	rows, err := s.Pool.Query(ctx, `select notice_id, account_id, to_account_id, amount_minor from notice
		where status = 'PENDING' and due_on <= $1 order by due_on, notice_id`, dayOf(today))
	if err != nil {
		return 0, 0, err
	}
	type n struct {
		id, from, to string
		amount       int64
	}
	var todo []n
	for rows.Next() {
		var x n
		if err := rows.Scan(&x.id, &x.from, &x.to, &x.amount); err != nil {
			rows.Close()
			return 0, 0, err
		}
		todo = append(todo, x)
	}
	rows.Close()
	for _, x := range todo {
		status := "PAID"
		if err := s.transfer(ctx, "notice:"+x.id, x.from, x.to, x.amount, codeInternal, "deposits-service", "notice "+x.id); err != nil {
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				return paid, failed, err
			}
			status = "FAILED"
		}
		if _, err := s.Pool.Exec(ctx, `update notice set status = $2 where notice_id = $1 and status = 'PENDING'`, x.id, status); err != nil {
			return paid, failed, err
		}
		if status == "PAID" {
			paid++
		} else {
			failed++
		}
	}
	return paid, failed, nil
}

// CatchUpDays is how far back RunDaily accrues days a run missed (the service was down).
const CatchUpDays = 7

// RunDaily does the daily work: accrue the days that ended (catching up on any missed in the last week, oldest
// first), pay notices due today, then pay last month's interest. Every step does nothing the second time, so the
// loop runs it often. Accruals come first, so a month is paid only after its last day has been accrued.
func (s *Service) RunDaily(ctx context.Context) error {
	now := s.now().UTC()
	today := dayOf(now)
	for back := CatchUpDays; back >= 1; back-- {
		if _, err := s.Accrue(ctx, today.AddDate(0, 0, -back)); err != nil {
			return fmt.Errorf("accrue: %w", err)
		}
	}
	if _, _, err := s.PayNotices(ctx, now); err != nil {
		return fmt.Errorf("notices: %w", err)
	}
	prev := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01")
	if _, err := s.PayInterest(ctx, prev); err != nil {
		return fmt.Errorf("interest %s: %w", prev, err)
	}
	return nil
}
