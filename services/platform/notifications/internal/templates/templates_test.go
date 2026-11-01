package templates

import (
	"strings"
	"testing"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
	"google.golang.org/protobuf/proto"
)

func wrap(t *testing.T, topic string, msg proto.Message) *eventsv1.EventEnvelope {
	t.Helper()
	_, data, err := events.Wrap(events.Params{Type: topic, Entity: "BANK", Producer: "test"}, msg)
	if err != nil {
		t.Fatal(err)
	}
	env, err := events.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestMoneyFormatsMinorUnits(t *testing.T) {
	cases := map[int64]string{0: "ZAR 0.00", 5: "ZAR 0.05", 12345: "ZAR 123.45", -250: "-ZAR 2.50"}
	for in, want := range cases {
		if got := Money(in, "ZAR"); got != want {
			t.Fatalf("Money(%d) = %q, want %q", in, got, want)
		}
	}
}

// A settled payment sends an email and an SMS, and both carry the amount and the reference, not the account number.
func TestSettledPaymentMessages(t *testing.T) {
	env := wrap(t, TopicSettled, &eventsv1.PaymentEvent{PaymentId: "pay-1", PartyId: "party-1", AmountMinor: "15000", Asset: "ZAR", FromAccountId: "acc-secret"})
	msgs, err := Render(env)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("render: %+v, %v", msgs, err)
	}
	for _, m := range msgs {
		if !strings.Contains(m.Body, "ZAR 150.00") || !strings.Contains(m.Body, "pay-1") {
			t.Fatalf("message %s lacks amount or reference: %q", m.Channel, m.Body)
		}
		if strings.Contains(m.Body, "acc-secret") {
			t.Fatal("an account number must not appear in a message")
		}
	}
}

func TestFailedPaymentExplainsWithoutInternals(t *testing.T) {
	env := wrap(t, TopicFailed, &eventsv1.PaymentEvent{PaymentId: "pay-2", PartyId: "p", AmountMinor: "100", Asset: "ZAR", Reason: "declined_insufficient_funds"})
	msgs, err := Render(env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msgs[0].Body, "No money has left your account") || strings.Contains(msgs[0].Body, "declined_") {
		t.Fatalf("body = %q", msgs[0].Body)
	}
}

// An event with no template is not an error: it is not ours. A blocked or unknown status change is left alone.
func TestEventsWithoutTemplatesAreNotForUs(t *testing.T) {
	env := wrap(t, TopicStatusChanged, &eventsv1.PartyStatusChanged{PartyId: "p", ToStatus: "PARTY_STATUS_PROSPECT"})
	if _, err := Render(env); err != ErrNotForUs {
		t.Fatalf("err = %v, want ErrNotForUs", err)
	}
}

func TestBadAmountIsRefused(t *testing.T) {
	env := wrap(t, TopicSettled, &eventsv1.PaymentEvent{PaymentId: "x", AmountMinor: "12.50", Asset: "ZAR"})
	if _, err := Render(env); err == nil {
		t.Fatal("a non-integer amount must be refused")
	}
}
