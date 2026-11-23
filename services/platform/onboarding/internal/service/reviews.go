package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/api/serviceerror"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
)

// ReviewQueue is the store the review endpoints read and decide through.
type ReviewQueue interface {
	Get(ctx context.Context, id string) (store.Review, error)
	List(ctx context.Context, openOnly bool) ([]store.Review, error)
	Decide(ctx context.Context, id, decision, reviewer, note string, now time.Time) error
	Undelivered(ctx context.Context, limit int) ([]store.Review, error)
	MarkSignalled(ctx context.Context, id string, now time.Time) error
}

// WithReviews sets the review queue the review endpoints use.
func (s *Service) WithReviews(q ReviewQueue) *Service {
	s.reviews = q
	return s
}

// ListKYCReviews implements OnboardingServiceHandler.
func (s *Service) ListKYCReviews(ctx context.Context, req *connect.Request[onboardingv1.ListKYCReviewsRequest]) (*connect.Response[onboardingv1.ListKYCReviewsResponse], error) {
	if s.reviews == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("reviews are not configured"))
	}
	rs, err := s.reviews.List(ctx, req.Msg.OpenOnly)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	out := &onboardingv1.ListKYCReviewsResponse{}
	for _, r := range rs {
		out.Reviews = append(out.Reviews, reviewProto(r))
	}
	return connect.NewResponse(out), nil
}

// DecideKYCReview implements OnboardingServiceHandler.
//
// A decision is a compliance record, so it is recorded first and does not depend on the workflow being reachable. The
// workflow is told second, and told again until it has been: a failed signal leaves the decision recorded and
// undelivered, and either a retry of the same decision or the redelivery loop finishes the job. Telling the workflow
// twice is harmless, because it ignores a decision for a subject it is not waiting on.
//
// Only one decision can be recorded. The same decision again (a retry whose response was lost) succeeds without
// changing anything; a different decision is refused.
func (s *Service) DecideKYCReview(ctx context.Context, req *connect.Request[onboardingv1.DecideKYCReviewRequest]) (*connect.Response[onboardingv1.DecideKYCReviewResponse], error) {
	if s.reviews == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("reviews are not configured"))
	}
	m := req.Msg
	if _, err := uuid.Parse(m.ReviewId); err != nil {
		return nil, invalid("review_id must be a canonical UUID")
	}
	if m.Decision != "APPROVED" && m.Decision != "DECLINED" {
		return nil, invalid("decision must be APPROVED or DECLINED")
	}
	reviewer := strings.TrimSpace(m.Reviewer)
	if reviewer == "" || len(reviewer) > 64 {
		return nil, invalid("reviewer is required, up to 64 characters")
	}
	note := strings.TrimSpace(m.Note)
	if len(note) < 20 || len(note) > 1000 {
		return nil, invalid("a note of 20 to 1000 characters says what was checked and why")
	}
	r, err := s.reviews.Get(ctx, m.ReviewId)
	if errors.Is(err, store.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("review not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	switch r.Status {
	case "OPEN":
		if err := s.reviews.Decide(ctx, r.ReviewID, m.Decision, reviewer, note, time.Now()); err != nil {
			if errors.Is(err, store.ErrNotOpen) { // another reviewer decided first
				return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the review is already decided"))
			}
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
	case "DECIDED":
		if r.Decision != m.Decision || r.Reviewer != reviewer || r.Note != note {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the review is already decided"))
		}
	default:
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	decided, err := s.reviews.Get(ctx, r.ReviewID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	if decided.SignalledAt == nil {
		if err := s.deliver(ctx, decided); err != nil {
			return nil, err
		}
		if decided, err = s.reviews.Get(ctx, r.ReviewID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
	}
	return connect.NewResponse(&onboardingv1.DecideKYCReviewResponse{Review: reviewProto(decided)}), nil
}

// deliver tells the onboarding workflow of a recorded decision and marks it delivered.
//
//   - Told: marked delivered.
//   - The onboarding no longer exists (it ended, so there is nothing to resume): marked delivered, so it is not
//     retried, and the caller is told the decision stands on the record but cannot take effect.
//   - Temporal unreachable: left undelivered, for a retry or the redelivery loop.
func (s *Service) deliver(ctx context.Context, r store.Review) error {
	err := s.tc.SignalWorkflow(ctx, r.OnboardingID, "", onboarding.SignalKYCReview, onboarding.KYCReviewSignal{
		SubjectPartyID: r.SubjectPartyID, Decision: r.Decision, Reviewer: r.Reviewer, Note: r.Note,
	})
	var notFound *serviceerror.NotFound
	switch {
	case err == nil:
	case errors.As(err, &notFound):
		_ = s.reviews.MarkSignalled(ctx, r.ReviewID, time.Now())
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the decision is recorded, but the onboarding is no longer waiting for it"))
	default:
		return connect.NewError(connect.CodeUnavailable, errors.New("the decision is recorded; telling the onboarding failed and will be retried"))
	}
	// If the mark fails the decision still stands, and the redelivery loop will tell the workflow once more.
	_ = s.reviews.MarkSignalled(ctx, r.ReviewID, time.Now())
	return nil
}

// RedeliverDecisions tells the workflow of every recorded decision it has not yet been told, and returns how many it
// told. It stops at the first sign that Temporal is unreachable and returns that error, so an outage is not hammered.
func (s *Service) RedeliverDecisions(ctx context.Context) (int, error) {
	if s.reviews == nil {
		return 0, nil
	}
	pending, err := s.reviews.Undelivered(ctx, 100)
	if err != nil {
		return 0, err
	}
	told := 0
	for _, r := range pending {
		err := s.deliver(ctx, r)
		switch {
		case err == nil:
			told++
		case connect.CodeOf(err) == connect.CodeFailedPrecondition: // the onboarding had ended; settled
		default:
			return told, err
		}
	}
	return told, nil
}

// RunRedelivery runs RedeliverDecisions every interval until ctx ends.
func (s *Service) RunRedelivery(ctx context.Context, every time.Duration, log *slog.Logger) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			told, err := s.RedeliverDecisions(ctx)
			switch {
			case err != nil:
				log.Warn("kyc decision redelivery", "told", told, "err", err)
			case told > 0:
				log.Info("kyc decisions redelivered", "told", told)
			}
		}
	}
}

func reviewProto(r store.Review) *onboardingv1.KYCReview {
	out := &onboardingv1.KYCReview{
		ReviewId: r.ReviewID, OnboardingId: r.OnboardingID, SubjectPartyId: r.SubjectPartyID, Reason: r.Reason,
		Status: r.Status, OpenedAt: r.OpenedAt.UTC().Format(time.RFC3339), Decision: r.Decision,
		Reviewer: r.Reviewer, Note: r.Note,
	}
	if r.DecidedAt != nil {
		out.DecidedAt = r.DecidedAt.UTC().Format(time.RFC3339)
	}
	return out
}
