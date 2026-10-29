package ledger

import (
	"context"
	"testing"
	"time"
)

// BenchmarkMemStoreTransfers measures the in-memory reference model, not TigerBeetle.
// It bounds the cost of the rule logic only. Cluster numbers come from the VERIFY-TB run.
func BenchmarkMemStoreTransfers(b *testing.B) {
	clock := time.Unix(1_800_000_000, 0)
	s := NewMemStore(func() time.Time { return clock })
	op, _ := NewID(clock)
	cust, _ := NewID(clock)
	_, _ = s.CreateAccounts(context.Background(), []Account{
		{ID: op, Ledger: 710, Code: 2001},
		{ID: cust, Ledger: 710, Code: 1001, DebitsMustNotExceedCredits: true},
	})
	batch := make([]Transfer, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for k := range batch {
			id, _ := NewID(clock)
			batch[k] = Transfer{ID: id, DebitAccountID: op, CreditAccountID: cust, Amount: 1, Ledger: 710, Code: 10}
		}
		if _, err := s.CreateTransfers(context.Background(), batch); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N*len(batch))/b.Elapsed().Seconds(), "transfers/s")
}
