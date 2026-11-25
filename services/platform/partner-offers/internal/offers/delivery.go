package offers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sending a referred customer's details to the provider.
//
// What is sent is exactly SharedFields and nothing else (the consent screen shows the same list, and a test holds this code to it). It is
// sent only if, at the moment of sending, the customer's consent is still in force, the customer is still active, and the provider is still
// active: a customer who withdraws before the sender runs is never sent, and one who withdraws afterwards cannot be recalled, which the consent
// screen says. Each delivery is signed so the provider can tell it came from Forge, and carries the referral id so the provider can ignore a repeat.
// Only a hash of what was sent is kept, never the details. A failed delivery is retried with growing gaps and then marked FAILED for compliance to see.

// payload is the whole of what a provider receives.
type payload struct {
	ReferralID string            `json:"referral_id"`
	OfferID    string            `json:"offer_id"`
	ProviderID string            `json:"provider_id"`
	ConsentID  string            `json:"consent_id"`
	SentAt     string            `json:"sent_at"`
	Details    map[string]string `json:"details"`
}

// detailKeys maps each entry of SharedFields to the key it travels under. An entry with no key here cannot be sent, so adding to SharedFields
// without teaching the sender how to fill it fails loudly instead of quietly sending less (or something else) than the consent promised.
var detailKeys = map[string]string{"your name": "name"}

// backoff is the wait after the first, second and later failures. MaxAttempts failures and the delivery is FAILED.
var backoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}

// MaxAttempts is how many times a delivery is tried before it is marked FAILED.
const MaxAttempts = 6

// Names (see offers.go) fills "your name".
func (s *Service) details(ctx context.Context, party string) (map[string]string, error) {
	out := map[string]string{}
	for _, label := range SharedFields {
		key, ok := detailKeys[label]
		if !ok {
			return nil, fmt.Errorf("offers: %q is in SharedFields but the sender cannot fill it", label)
		}
		switch key {
		case "name":
			n, err := s.Names.Name(ctx, party)
			if err != nil {
				return nil, err
			}
			out[key] = n
		default:
			return nil, fmt.Errorf("offers: no source for detail %q", key)
		}
	}
	return out, nil
}

// Sign signs a body as providers verify it: HMAC-SHA256, hex, prefixed "sha256=".
func Sign(key, body []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func (s *Service) signingKey(providerID string) ([]byte, error) {
	if s.SecretsDir == "" {
		return nil, errors.New("no secrets directory is configured")
	}
	b, err := os.ReadFile(filepath.Join(s.SecretsDir, providerID+".key"))
	if err != nil {
		return nil, errors.New("no signing key for this provider")
	}
	k := []byte(strings.TrimSpace(string(b)))
	if len(k) < 16 {
		return nil, errors.New("the provider's signing key is too short")
	}
	return k, nil
}

// publicClient sends to the provider. It refuses redirects and refuses to connect to a private, loopback or link-local address, so a provider
// address (or a DNS answer) pointing inside our network gets nothing.
func publicClient() *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return errors.New("refusing to connect to a non-public address")
		}
		return nil
	}}
	return &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     &http.Transport{DialContext: d.DialContext, TLSHandshakeTimeout: 5 * time.Second, DisableKeepAlives: true},
	}
}

// Deliver sends every due, still-permitted delivery. It returns how many were sent.
func (s *Service) Deliver(ctx context.Context) (int, error) {
	if !s.Enabled || s.Names == nil {
		return 0, nil
	}
	rows, err := s.Pool.Query(ctx, `select referral_id from referral_delivery where status = 'PENDING' and next_at <= $1 order by next_at limit 50`, s.now())
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	sent := 0
	for _, id := range ids {
		ok, err := s.deliverOne(ctx, id)
		if err != nil {
			return sent, err
		}
		if ok {
			sent++
		}
	}
	return sent, nil
}

func (s *Service) deliverOne(ctx context.Context, id string) (bool, error) {
	sent := false
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var status string
		var attempts int
		if err := tx.QueryRow(ctx, `select status, attempts from referral_delivery where referral_id = $1 for update skip locked`, id).Scan(&status, &attempts); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // another worker has it
			}
			return err
		}
		if status != "PENDING" {
			return nil
		}
		var party, offerID, providerID, consentID string
		if err := tx.QueryRow(ctx, `select r.party_id, r.offer_id, o.provider_id, r.consent_id from referral r join offer o using (offer_id) where r.referral_id = $1`, id).
			Scan(&party, &offerID, &providerID, &consentID); err != nil {
			return err
		}
		p, err := s.loadProvider(ctx, tx, providerID)
		if err != nil {
			return err
		}
		finish := func(status, detail string) error {
			_, err := tx.Exec(ctx, `update referral_delivery set status = $2, detail = $3 where referral_id = $1`, id, status, detail)
			return err
		}
		// Re-check everything now, not when the customer clicked.
		if p.Status != "ACTIVE" {
			return finish("CANCELLED", "the provider is no longer active")
		}
		if p.DeliveryURL == "" {
			return finish("NOT_CONFIGURED", "the provider has no delivery address")
		}
		if _, ok, err := s.Consents.Allowed(ctx, party, ConsentPurpose, ConsentScope(providerID)); err != nil {
			return s.retry(ctx, tx, id, attempts, "could not check the customer's consent")
		} else if !ok {
			return finish("CANCELLED", "the customer withdrew their consent before the details were sent")
		}
		if s.Parties != nil {
			if ok, err := s.Parties.Active(ctx, party); err != nil {
				return s.retry(ctx, tx, id, attempts, "could not check the customer's status")
			} else if !ok {
				return finish("CANCELLED", "this action is not available on the customer's account")
			}
		}
		key, err := s.signingKey(providerID)
		if err != nil {
			return s.retry(ctx, tx, id, attempts, err.Error())
		}
		d, err := s.details(ctx, party)
		if err != nil {
			return s.retry(ctx, tx, id, attempts, "could not read the customer's details")
		}
		now := s.now()
		body, _ := json.Marshal(payload{ReferralID: id, OfferID: offerID, ProviderID: providerID, ConsentID: consentID, SentAt: now.Format(time.RFC3339), Details: d})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.DeliveryURL, bytes.NewReader(body))
		if err != nil {
			return finish("FAILED", "the provider's delivery address is not valid")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forge-Signature", Sign(key, body))
		req.Header.Set("X-Forge-Referral-Id", id)
		client := s.HTTP
		if client == nil {
			client = publicClient()
		}
		res, err := client.Do(req)
		if err != nil {
			return s.retry(ctx, tx, id, attempts, "the provider could not be reached")
		}
		res.Body.Close()
		h := sha256.Sum256(body)
		switch {
		case res.StatusCode >= 200 && res.StatusCode < 300:
			_, err := tx.Exec(ctx, `update referral_delivery set status='SENT', attempts=attempts+1, sent_at=$2, payload_hash=$3, detail='' where referral_id=$1`, id, now, hex.EncodeToString(h[:]))
			sent = err == nil
			return err
		case res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500:
			return s.retry(ctx, tx, id, attempts, fmt.Sprintf("the provider answered %d", res.StatusCode))
		default:
			_, err := tx.Exec(ctx, `update referral_delivery set status='FAILED', attempts=attempts+1, detail=$2 where referral_id=$1`, id, fmt.Sprintf("the provider refused it with %d", res.StatusCode))
			return err
		}
	})
	return sent, err
}

// retry records a failed attempt and schedules the next, or gives up.
func (s *Service) retry(ctx context.Context, tx pgx.Tx, id string, attempts int, why string) error {
	attempts++
	if attempts >= MaxAttempts {
		_, err := tx.Exec(ctx, `update referral_delivery set status='FAILED', attempts=$2, detail=$3 where referral_id=$1`, id, attempts, why+"; gave up")
		return err
	}
	wait := backoff[min(attempts-1, len(backoff)-1)]
	_, err := tx.Exec(ctx, `update referral_delivery set attempts=$2, next_at=$3, detail=$4 where referral_id=$1`, id, attempts, s.now().Add(wait), why)
	return err
}
