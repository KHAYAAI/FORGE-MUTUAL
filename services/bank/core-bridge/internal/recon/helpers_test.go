package recon

import (
	"time"

	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/fineract"
)

type fineractJournal = fineract.Journal
type fineractLine = fineract.Line

func nowT() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }
