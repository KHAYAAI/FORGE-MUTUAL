package service

import (
	"context"
	"errors"
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

// DecideKYCReview implements OnboardingServiceHandler. The workflow is told first. If it cannot be told, the review
// stays open, so the decision can be made again; once it has been told, the decision is recorded.
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
	if r := strings.TrimSpace(m.Reviewer); r == "" || len(r) > 64 {
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
	if r.Status != "OPEN" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the review is already decided"))
	}
	err = s.tc.SignalWorkflow(ctx, r.OnboardingID, "", onboarding.SignalKYCReview, onboarding.KYCReviewSignal{
		SubjectPartyID: r.SubjectPartyID, Decision: m.Decision, Reviewer: strings.TrimSpace(m.Reviewer), Note: note,
	})
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the onboarding is no longer waiting for a decision"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable; try again"))
	}
	if err := s.reviews.Decide(ctx, m.ReviewId, m.Decision, strings.TrimSpace(m.Reviewer), note, time.Now()); err != nil {
		if errors.Is(err, store.ErrNotOpen) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the review is already decided"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	decided, err := s.reviews.Get(ctx, m.ReviewId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&onboardingv1.DecideKYCReviewResponse{Review: reviewProto(decided)}), nil
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
