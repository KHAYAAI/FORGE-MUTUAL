package projection

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/eventbus"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/store"
)

// This test runs the real path: a ledger event goes through the broker and the consumer into the read model. It needs
// KAFKA_TEST_BROKERS and CONSOLIDATED_TEST_DSN.
func TestTransferEventReachesTheReadModelThroughKafka(t *testing.T) {
	brokers := os.Getenv("KAFKA_TEST_BROKERS")
	dsn := os.Getenv("CONSOLIDATED_TEST_DSN")
	if brokers == "" || dsn == "" {
		t.Skip("KAFKA_TEST_BROKERS and CONSOLIDATED_TEST_DSN are not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := eventbus.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	account := "proj-a-" + suffix
	other := "proj-b-" + suffix
	bs := strings.Split(brokers, ",")

	topic := "forge.test.projection." + suffix
	admin, err := kgo.NewClient(kgo.SeedBrokers(bs...))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if resp, err := kadm.NewClient(admin).CreateTopic(ctx, 1, 1, nil, topic); err != nil || resp.Err != nil {
		t.Fatalf("topic: %v %v", err, resp.Err)
	}

	producer, err := kgo.NewClient(kgo.SeedBrokers(bs...))
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()
	var recs []*kgo.Record
	for i, amount := range []string{"1000", "250"} {
		_, data, err := events.Wrap(events.Params{
			Type: TopicTransferCommitted, Entity: "BANK", Producer: "ledger-gateway-bank", Correlation: fmt.Sprint(i),
		}, &eventsv1.TransferCommitted{
			TransferId: fmt.Sprintf("tr-%s-%d", suffix, i), DebitAccountId: other, CreditAccountId: account,
			Asset: "ZAR", AmountMinor: amount, Code: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		recs = append(recs, &kgo.Record{Topic: topic, Value: data})
	}
	if err := producer.ProduceSync(ctx, recs...).FirstErr(); err != nil {
		t.Fatal(err)
	}

	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	consumer := &eventbus.Consumer{Name: "projection-test-" + suffix, Pool: pool, Brokers: bs, Group: "g-" + suffix,
		Topics: []string{topic}, Handle: Handle}
	done := make(chan error, 1)
	go func() { done <- consumer.Run(cctx) }()
	deadline := time.Now().Add(50 * time.Second)
	for {
		var credit int64
		err := pool.QueryRow(ctx, `select credit_minor from cv_balance where account_id = $1`, account).Scan(&credit)
		if err == nil && credit == 1250 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("read model did not reach 1250 (last %d, err %v)", credit, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	cancel()
	<-done

	var debit int64
	_ = pool.QueryRow(ctx, `select debit_minor from cv_balance where account_id = $1`, other).Scan(&debit)
	if debit != 1250 {
		t.Fatalf("paying account debit = %d, want 1250", debit)
	}
}

// An event of another type must be refused, so a misrouted message cannot change balances.
func TestHandleRefusesAnotherType(t *testing.T) {
	_, data, _ := events.Wrap(events.Params{Type: "forge.bank.payment.settled.v1", Entity: "BANK", Producer: "x"}, &eventsv1.PaymentEvent{})
	env, _ := events.Open(data)
	if err := Handle(context.Background(), (pgx.Tx)(nil), env); err == nil {
		t.Fatal("another type must be refused")
	}
}
