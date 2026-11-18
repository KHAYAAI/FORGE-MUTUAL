// Package sender delivers pending messages through a provider. A failed delivery is retried until its attempts run
// out, then it is marked FAILED and stays visible. Addresses are opened only here, and never logged.
package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/crypt"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/store"
)

// Provider delivers one message to an address. Real SMTP and SMS providers are not chosen yet (VERIFY).
type Provider interface {
	Send(ctx context.Context, channel, address, subject, body string) error
}

// Sender runs delivery passes.
type Sender struct {
	Pool        *pgxpool.Pool
	Key         crypt.Key
	Provider    Provider
	MaxAttempts int // default 5
	Batch       int // default 50
}

// Pass delivers one batch. It returns how many were sent and how many failed this time.
func (s *Sender) Pass(ctx context.Context) (sent, failed int, err error) {
	max, batch := s.MaxAttempts, s.Batch
	if max <= 0 {
		max = 5
	}
	if batch <= 0 {
		batch = 50
	}
	msgs, err := store.Claim(ctx, s.Pool, batch, max)
	if err != nil {
		return 0, 0, err
	}
	for _, m := range msgs {
		sealed, err := store.Contact(ctx, s.Pool, m.PartyID, m.Channel)
		if err != nil {
			failed++
			_ = store.MarkFailed(ctx, s.Pool, m.MessageID, max, "contact unavailable: "+err.Error())
			continue
		}
		address, err := s.Key.Open(sealed)
		if err != nil {
			failed++
			_ = store.MarkFailed(ctx, s.Pool, m.MessageID, max, "contact could not be opened")
			continue
		}
		if err := s.Provider.Send(ctx, m.Channel, string(address), m.Subject, m.Body); err != nil {
			failed++
			_ = store.MarkFailed(ctx, s.Pool, m.MessageID, max, "delivery failed: "+err.Error())
			continue
		}
		if err := store.MarkSent(ctx, s.Pool, m.MessageID); err != nil {
			return sent, failed, fmt.Errorf("sender: mark sent: %w", err)
		}
		sent++
	}
	return sent, failed, nil
}

// FileSink writes each message as a JSON line. It is for development and tests only: the file holds plaintext addresses.
type FileSink struct {
	Path string
	mu   sync.Mutex
}

// Send implements Provider.
func (f *FileSink) Send(_ context.Context, channel, address, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fh, err := os.OpenFile(f.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer fh.Close()
	b, err := json.Marshal(map[string]string{
		"channel": channel, "address": address, "subject": subject, "body": body,
		"at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	_, err = fh.Write(append(b, '\n'))
	return err
}
