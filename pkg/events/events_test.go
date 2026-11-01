package events

import (
	"testing"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
)

func TestWrapOpenDecodeRoundTrip(t *testing.T) {
	in := &eventsv1.PaymentEvent{PaymentId: "p1", AmountMinor: "100", Status: "SETTLED"}
	id, data, err := Wrap(Params{Type: "forge.bank.payment.settled.v1", Entity: "BANK", Producer: "payments", Correlation: "p1"}, in)
	if err != nil {
		t.Fatal(err)
	}
	env, err := Open(data)
	if err != nil || env.EventId != id || env.EventType != "forge.bank.payment.settled.v1" {
		t.Fatalf("open: %+v, %v", env, err)
	}
	var out eventsv1.PaymentEvent
	if err := Decode(env, "forge.bank.payment.settled.v1", &out); err != nil || out.PaymentId != "p1" || out.AmountMinor != "100" {
		t.Fatalf("decode: %+v, %v", &out, err)
	}
}

// A consumer subscribed to one topic must not accept another topic's event.
func TestDecodeRefusesAnotherTopic(t *testing.T) {
	_, data, _ := Wrap(Params{Type: "forge.bank.payment.failed.v1", Entity: "BANK", Producer: "payments"}, &eventsv1.PaymentEvent{})
	env, _ := Open(data)
	if err := Decode(env, "forge.bank.payment.settled.v1", &eventsv1.PaymentEvent{}); err == nil {
		t.Fatal("a different topic must be refused")
	}
}

// Bare bytes with no envelope type are refused.
func TestOpenRefusesABareMessage(t *testing.T) {
	bare := []byte{0x0a, 0x02, 'p', '1'}
	if _, err := Open(bare); err == nil {
		t.Fatal("bare message accepted as an envelope")
	}
}

func TestWrapRequiresTypeEntityProducer(t *testing.T) {
	if _, _, err := Wrap(Params{Entity: "BANK", Producer: "x"}, &eventsv1.PaymentEvent{}); err == nil {
		t.Fatal("missing type must be refused")
	}
}
