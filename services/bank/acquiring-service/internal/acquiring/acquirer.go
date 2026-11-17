package acquiring

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Acquirer is the acquiring bank: it hosts the checkout where the payer enters card details, reports results to the
// webhook, and refunds. No acquirer is chosen yet (VERIFY-ACQUIRER); the simulator stands in, and production refuses it.
type Acquirer interface {
	// Checkout returns the hosted page the payer is sent to, and whether it is the simulator.
	Checkout(ctx context.Context, link Link) (url string, simulated bool, err error)
	// Refund returns money to the payer's card.
	Refund(ctx context.Context, link Link, amountMinor int64, key string) error
	// VerifyWebhook authenticates a delivery and reads the result. ErrSignature for anything not signed by the acquirer.
	VerifyWebhook(body []byte, header http.Header) (Result, error)
	Simulated() bool
}

// Result is what the acquirer reports about one payment.
type Result struct {
	LinkID      string `json:"link_id"`
	Status      string `json:"status"` // APPROVED or DECLINED
	AmountMinor int64  `json:"amount_minor"`
	Reference   string `json:"acquirer_ref"`
}

// ErrSignature is returned for a webhook the acquirer did not sign.
var ErrSignature = errors.New("acquiring: the webhook signature does not verify")

// Simulator is the development acquirer. Its checkout is the web app's simulator page, and its webhooks are signed
// with HMAC-SHA256 in X-Forge-Signature, the same check a real acquirer's must pass.
type Simulator struct {
	PayBaseURL string // the web app's origin, e.g. http://127.0.0.1:3000
	Secret     []byte
	Refunds    []string // keys refunded, for tests
}

func (s *Simulator) Simulated() bool { return true }

func (s *Simulator) Checkout(_ context.Context, l Link) (string, bool, error) {
	return strings.TrimRight(s.PayBaseURL, "/") + "/pay/" + l.Token + "/simulator", true, nil
}

func (s *Simulator) Refund(_ context.Context, _ Link, _ int64, key string) error {
	s.Refunds = append(s.Refunds, key)
	return nil
}

// Sign signs a body as the simulator does.
func (s *Simulator) Sign(body []byte) string {
	m := hmac.New(sha256.New, s.Secret)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Simulator) VerifyWebhook(body []byte, header http.Header) (Result, error) {
	got, err := hex.DecodeString(header.Get("X-Forge-Signature"))
	if err != nil || len(s.Secret) < 16 || !hmac.Equal(got, mustHex(s.Sign(body))) {
		return Result{}, ErrSignature
	}
	var r Result
	if err := json.Unmarshal(body, &r); err != nil || r.LinkID == "" || (r.Status != "APPROVED" && r.Status != "DECLINED") {
		return Result{}, ErrSignature
	}
	return r, nil
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
