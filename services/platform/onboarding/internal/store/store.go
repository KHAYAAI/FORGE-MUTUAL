// Package store holds KYC reviews: verifications a person must decide before an onboarding can continue.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/uuid"
)

//go:embed schema.sql
var schema string

// Errors.
var (
	ErrNotFound = errors.New("store: review not found")
	ErrNotOpen  = errors.New("store: review is not open")
)

// Review is one KYC review.
type Review struct {
	ReviewID       string
	OnboardingID   string
	SubjectPartyID string
	Reason         string
	Status         string
	OpenedAt       time.Time
	Decision       string
	Reviewer       string
	Note           string
	DecidedAt      *time.Time
	// SignalledAt is when the onboarding workflow was told of the decision, or found to have ended. A decided
	// review with no SignalledAt is waiting for delivery.
	SignalledAt *time.Time
}

// columns is every column of a review, in the order scanReview reads them.
const columns = `review_id, onboarding_id, subject_party_id, reason, status, opened_at, decision, reviewer, note, decided_at, signalled_at`

func scanReview(row pgx.Row) (Review, error) {
	var r Review
	err := row.Scan(&r.ReviewID, &r.OnboardingID, &r.SubjectPartyID, &r.Reason, &r.Status, &r.OpenedAt,
		&r.Decision, &r.Reviewer, &r.Note, &r.DecidedAt, &r.SignalledAt)
	return r, err
}

// Store holds reviews.
type Store struct {
	pool *pgxpool.Pool
}

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) (*Store, error) {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Pool returns the pool, for tests that need to reset tables.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Open records a review for a subject. While a subject has an open review, a repeat returns that review, so a
// repeated provider verdict never queues two reviews.
func (s *Store) OpenReview(ctx context.Context, onboardingID, subjectID, reason string, now time.Time) (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	var out string
	err = s.pool.QueryRow(ctx, `
		insert into kyc_review (review_id, onboarding_id, subject_party_id, reason, status, opened_at)
		values ($1, $2, $3, $4, 'OPEN', $5)
		on conflict (onboarding_id, subject_party_id) where status = 'OPEN' do nothing
		returning review_id`, id, onboardingID, subjectID, reason, now.UTC()).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			select review_id from kyc_review where onboarding_id = $1 and subject_party_id = $2 and status = 'OPEN'`,
			onboardingID, subjectID).Scan(&out)
	}
	if err != nil {
		return "", fmt.Errorf("store: open review: %w", err)
	}
	return out, nil
}

// Get returns one review.
func (s *Store) Get(ctx context.Context, id string) (Review, error) {
	r, err := scanReview(s.pool.QueryRow(ctx, `select `+columns+` from kyc_review where review_id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	return r, err
}

// List returns reviews, oldest first. With openOnly, only open reviews.
func (s *Store) List(ctx context.Context, openOnly bool) ([]Review, error) {
	q := `select ` + columns + ` from kyc_review`
	if openOnly {
		q += ` where status = 'OPEN'`
	}
	q += ` order by opened_at, review_id`
	return s.reviews(ctx, "list", q)
}

// reviews runs a query that selects columns and returns every row.
func (s *Store) reviews(ctx context.Context, what, query string, args ...any) ([]Review, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: %s: %w", what, err)
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Decide records a decision on an open review. A review that is not open is refused, so a decision is recorded once.
func (s *Store) Decide(ctx context.Context, id, decision, reviewer, note string, now time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		update kyc_review set status = 'DECIDED', decision = $2, reviewer = $3, note = $4, decided_at = $5
		where review_id = $1 and status = 'OPEN'`, id, decision, reviewer, note, now.UTC())
	if err != nil {
		return fmt.Errorf("store: decide: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return ErrNotOpen
}

// Undelivered returns decided reviews whose decision the onboarding workflow has not yet been told, oldest first.
func (s *Store) Undelivered(ctx context.Context, limit int) ([]Review, error) {
	return s.reviews(ctx, "undelivered", `select `+columns+` from kyc_review
		where status = 'DECIDED' and signalled_at is null order by decided_at, review_id limit $1`, limit)
}

// MarkSignalled records that the workflow has been told of a decision, or has ended and cannot be. It changes nothing
// for a review that is open, or already marked, so it is safe to repeat.
func (s *Store) MarkSignalled(ctx context.Context, id string, now time.Time) error {
	if _, err := s.pool.Exec(ctx, `
		update kyc_review set signalled_at = $2 where review_id = $1 and status = 'DECIDED' and signalled_at is null`,
		id, now.UTC()); err != nil {
		return fmt.Errorf("store: mark signalled: %w", err)
	}
	return nil
}
