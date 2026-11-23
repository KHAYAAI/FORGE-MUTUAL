package cards

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Processor is the card processor: it holds card numbers, gives each card a token, asks Forge to approve every purchase,
// and reports clearing, reversals and refunds. No processor or programme manager is chosen yet (VERIFY-PROCESSOR); the
// simulator stands in, and production refuses it.
type Processor interface {
	// CreateCard issues a card. The same key returns the same card: a retry after a lost answer must not issue a second.
	CreateCard(ctx context.Context, key, label string) (ProcCard, error)
	// CloseCard cancels the card at the processor. Closing twice is not an error.
	CloseCard(ctx context.Context, token string) error
	// DisplayURL returns a short-lived address on the processor where the cardholder sees the number and security code.
	DisplayURL(ctx context.Context, token string) (url string, expires time.Time, err error)
	// VerifyWebhook authenticates a delivery and reads it. ErrSignature for anything the processor did not sign.
	VerifyWebhook(body []byte, header http.Header) (Event, error)
	Simulated() bool
}

// ProcCard is what the processor tells Forge about a new card. Never a card number.
type ProcCard struct {
	Token       string
	Last4       string
	ExpiryMonth int
	ExpiryYear  int
}

// Event types the processor sends.
const (
	EventAuthorisation = "AUTHORISATION"
	EventClearing      = "CLEARING"
	EventReversal      = "REVERSAL"
	EventRefund        = "REFUND"
)

// Event is one thing the processor reports. For an authorisation, ID is the authorisation id; for clearing and refund
// it is that event's own id and AuthID names the authorisation it belongs to.
type Event struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	AuthID      string `json:"auth_id"`
	Token       string `json:"card_token"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Merchant    string `json:"merchant_name"`
	MCC         string `json:"mcc"`
	Country     string `json:"country"`
}

// ErrSignature is returned for a delivery the processor did not sign.
var ErrSignature = errors.New("cards: the webhook signature does not verify")

// Simulator is the development processor. Its webhooks are signed with HMAC-SHA256 in X-Forge-Signature, the same check
// a real processor's must pass. Its display address carries no card number.
type Simulator struct {
	Secret []byte
	mu     sync.Mutex
	byKey  map[string]ProcCard
	closed map[string]bool
}

func (s *Simulator) Simulated() bool { return true }

func (s *Simulator) CreateCard(_ context.Context, key, _ string) (ProcCard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byKey == nil {
		s.byKey, s.closed = map[string]ProcCard{}, map[string]bool{}
	}
	if c, ok := s.byKey[key]; ok {
		return c, nil
	}
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return ProcCard{}, err
	}
	l4 := make([]byte, 2)
	if _, err := rand.Read(l4); err != nil {
		return ProcCard{}, err
	}
	c := ProcCard{Token: "tok_" + hex.EncodeToString(b), Last4: fmt.Sprintf("%04d", (int(l4[0])<<8|int(l4[1]))%10000),
		ExpiryMonth: int(time.Now().Month()), ExpiryYear: time.Now().Year() + 3}
	s.byKey[key] = c
	return c, nil
}

func (s *Simulator) CloseCard(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed == nil {
		s.closed = map[string]bool{}
	}
	s.closed[token] = true
	return nil
}

// Closed reports whether a token was closed, for tests.
func (s *Simulator) Closed(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed[token]
}

func (s *Simulator) DisplayURL(_ context.Context, token string) (string, time.Time, error) {
	exp := time.Now().Add(time.Minute).UTC()
	return fmt.Sprintf("/simulator/card/%s?expires=%d", token, exp.Unix()), exp, nil
}

// Sign signs a body as the simulator does.
func (s *Simulator) Sign(body []byte) string {
	m := hmac.New(sha256.New, s.Secret)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Simulator) VerifyWebhook(body []byte, header http.Header) (Event, error) {
	got, err := hex.DecodeString(header.Get("X-Forge-Signature"))
	want, _ := hex.DecodeString(s.Sign(body))
	if err != nil || len(s.Secret) < 16 || !hmac.Equal(got, want) {
		return Event{}, ErrSignature
	}
	var e Event
	if err := json.Unmarshal(body, &e); err != nil {
		return Event{}, ErrSignature
	}
	switch e.Type {
	case EventAuthorisation:
		if e.ID == "" || e.Token == "" {
			return Event{}, ErrSignature
		}
	case EventClearing, EventRefund:
		if e.ID == "" || e.AuthID == "" {
			return Event{}, ErrSignature
		}
	case EventReversal:
		if e.AuthID == "" {
			return Event{}, ErrSignature
		}
	default:
		return Event{}, ErrSignature
	}
	e.Type, e.Currency = strings.ToUpper(e.Type), strings.ToUpper(e.Currency)
	return e, nil
}
