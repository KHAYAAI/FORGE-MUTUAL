package gateway

import (
	"context"
	"log/slog"
	"time"
)

// RunRetention deletes COMMITTED idempotency records older than retention, every interval,
// until ctx is cancelled. The retention must be longer than any client's retry window: after
// a purge, a replay of the same key would move money again (ARCHITECTURE.md 7.6, 30 days).
func RunRetention(ctx context.Context, repo Repository, retention, interval time.Duration, now func() time.Time, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := repo.PurgeRecords(ctx, now().Add(-retention))
			if err != nil {
				log.Error("idempotency purge failed", "err", err)
				continue
			}
			if n > 0 {
				log.Info("idempotency records purged", "count", n)
			}
		}
	}
}
