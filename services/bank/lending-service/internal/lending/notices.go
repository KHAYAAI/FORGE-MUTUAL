package lending

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// NoticeAfterDays is how long an instalment may be overdue before a default notice is issued; RespondDays is how long
// the customer then has. Both PROPOSED (threshold 27): the NCA sets the real rules for s.129 notices; verify them.
const (
	NoticeAfterDays = 20
	RespondDays     = 14
)

// NoticeText is what a default notice says. It must be checked by a lawyer against s.129 before use.
const NoticeText = "You are behind on your repayments. Before we take any further step, you may refer this agreement to a " +
	"debt counsellor, an alternative dispute resolution agent, the consumer court or an ombud, to resolve any dispute or " +
	"agree a plan to bring your payments up to date. Please contact us before the date shown."

// IssueNotices issues a default notice for every active facility whose oldest unpaid instalment is at least
// NoticeAfterDays overdue and which has no outstanding notice, and clears the notices of facilities that are up to
// date. Both steps are safe to repeat.
func (s *Service) IssueNotices(ctx context.Context) (issued int, err error) {
	if _, err := s.Pool.Exec(ctx, `update default_notice n set cleared_at = $1 from facility f
		where n.facility_id = f.facility_id and n.cleared_at is null and (f.days_past_due = 0 or f.status <> 'ACTIVE')`, s.now()); err != nil {
		return 0, err
	}
	rows, err := s.Pool.Query(ctx, `select f.facility_id, f.days_past_due,
		coalesce((select sum(principal_minor + interest_minor) from instalment i where i.facility_id = f.facility_id and i.status = 'ARREARS'), 0)
		from facility f where f.status = 'ACTIVE' and f.days_past_due >= $1
		and not exists (select 1 from default_notice n where n.facility_id = f.facility_id and n.cleared_at is null)`, NoticeAfterDays)
	if err != nil {
		return 0, err
	}
	type due struct {
		id      string
		dpd     int32
		arrears int64
	}
	var todo []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.dpd, &d.arrears); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, d)
	}
	rows.Close()
	for _, d := range todo {
		id, err := uuid.NewV4()
		if err != nil {
			return issued, err
		}
		now := s.now()
		tag, err := s.Pool.Exec(ctx, `insert into default_notice (notice_id, facility_id, issued_at, respond_by, arrears_minor, days_past_due)
			values ($1, $2, $3, $4, $5, $6) on conflict do nothing`, id, d.id, now, now.AddDate(0, 0, RespondDays), d.arrears, d.dpd)
		if err != nil {
			return issued, err
		}
		if tag.RowsAffected() == 1 {
			issued++
			if err := s.event(ctx, s.Pool, "notice:"+id, d.id, "DEFAULT_NOTICE", 0, 0, fmt.Sprintf("default notice: %d days overdue", d.dpd)); err != nil {
				return issued, err
			}
		}
	}
	return issued, nil
}

// openNotice is a facility's outstanding default notice, if any.
func (s *Service) openNotice(ctx context.Context, facility string) (*lendingv1.DefaultNotice, error) {
	var n lendingv1.DefaultNotice
	var issued, respond time.Time
	var arrears int64
	var dpd int32
	err := s.Pool.QueryRow(ctx, `select notice_id, issued_at, respond_by, arrears_minor, days_past_due from default_notice
		where facility_id = $1 and cleared_at is null`, facility).Scan(&n.NoticeId, &issued, &respond, &arrears, &dpd)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n.IssuedAt, n.RespondBy = issued.UTC().Format(time.RFC3339), respond.UTC().Format("2006-01-02")
	n.ArrearsMinor, n.DaysPastDue, n.Text = strconv.FormatInt(arrears, 10), uint32(dpd), NoticeText
	return &n, nil
}

// GetStatement implements the handler: the facility's events in a calendar month, and what is owed now.
func (s *Service) GetStatement(ctx context.Context, req *connect.Request[lendingv1.GetStatementRequest]) (*connect.Response[lendingv1.GetStatementResponse], error) {
	m := req.Msg
	start, err := time.Parse("2006-01", m.Period)
	if m.PartyId == "" || m.FacilityId == "" || err != nil {
		return nil, invalid("party_id, facility_id and a period of YYYY-MM are required")
	}
	f, err := s.facility(ctx, m.PartyId, m.FacilityId)
	if err != nil {
		return nil, wrap(err)
	}
	rows, err := s.Pool.Query(ctx, `select at, kind, principal_minor, interest_minor, description from facility_event
		where facility_id = $1 and at >= $2 and at < $3 order by at, event_key`, m.FacilityId, start, start.AddDate(0, 1, 0))
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &lendingv1.GetStatementResponse{FacilityId: f.FacilityId, ProductId: f.ProductId, Period: m.Period,
		OutstandingNowMinor: f.OutstandingMinor, RateBps: f.RateBps, GeneratedAt: s.now().Format(time.RFC3339)}
	for rows.Next() {
		var at time.Time
		var e lendingv1.StatementEntry
		var p, i int64
		if err := rows.Scan(&at, &e.Kind, &p, &i, &e.Description); err != nil {
			return nil, wrap(err)
		}
		e.At, e.PrincipalMinor, e.InterestMinor = at.UTC().Format(time.RFC3339), strconv.FormatInt(p, 10), strconv.FormatInt(i, 10)
		out.Entries = append(out.Entries, &e)
	}
	return connect.NewResponse(out), rows.Err()
}
