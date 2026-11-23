// Package templates renders the transactional messages that the platform sends. Each message names the event that
// caused it. Messages carry no account numbers and no personal data beyond the first name and the amount.
package templates

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
)

// Topics the service reacts to.
const (
	TopicSettled       = "forge.bank.payment.settled.v1"
	TopicFailed        = "forge.bank.payment.failed.v1"
	TopicStatusChanged = "forge.plat.party.status_changed.v1"
	TopicRevoked       = "forge.plat.consent.revoked.v1"
)

// Topics lists every topic the service subscribes to.
var Topics = []string{TopicSettled, TopicFailed, TopicStatusChanged, TopicRevoked}

// Channels.
const (
	ChannelEmail = "EMAIL"
	ChannelSMS   = "SMS"
)

// Message is one rendered message for one channel. PartyID is the recipient; the address is resolved at send time.
type Message struct {
	PartyID string
	Channel string
	Subject string
	Body    string
}

// ErrNotForUs is returned for an event that has no template.
var ErrNotForUs = errors.New("templates: no template for this event")

// Money formats minor units with two decimals, for example 12345 becomes "123.45".
func Money(minor int64, asset string) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	s := fmt.Sprintf("%s %d.%02d", asset, minor/100, minor%100)
	if neg {
		return "-" + s
	}
	return s
}

// Render returns the messages for one event. A settled or failed payment is also sent by SMS, because it is time
// sensitive; the other messages are email only.
func Render(env *eventsv1.EventEnvelope) ([]Message, error) {
	switch env.EventType {
	case TopicSettled:
		var e eventsv1.PaymentEvent
		if err := events.Decode(env, TopicSettled, &e); err != nil {
			return nil, err
		}
		amount, err := parseAmount(e.AmountMinor, e.Asset)
		if err != nil {
			return nil, err
		}
		return []Message{
			{PartyID: e.PartyId, Channel: ChannelEmail, Subject: "Your payment was sent",
				Body: fmt.Sprintf("Your payment of %s has been sent. Reference %s.", amount, e.PaymentId)},
			{PartyID: e.PartyId, Channel: ChannelSMS,
				Body: fmt.Sprintf("FORGE: payment of %s sent. Ref %s.", amount, e.PaymentId)},
		}, nil
	case TopicFailed:
		var e eventsv1.PaymentEvent
		if err := events.Decode(env, TopicFailed, &e); err != nil {
			return nil, err
		}
		amount, err := parseAmount(e.AmountMinor, e.Asset)
		if err != nil {
			return nil, err
		}
		return []Message{
			{PartyID: e.PartyId, Channel: ChannelEmail, Subject: "Your payment was not completed",
				Body: fmt.Sprintf("Your payment of %s was not completed (%s). No money has left your account. Reference %s.",
					amount, reasonText(e.Reason), e.PaymentId)},
			{PartyID: e.PartyId, Channel: ChannelSMS,
				Body: fmt.Sprintf("FORGE: payment of %s not completed. Ref %s.", amount, e.PaymentId)},
		}, nil
	case TopicStatusChanged:
		var e eventsv1.PartyStatusChanged
		if err := events.Decode(env, TopicStatusChanged, &e); err != nil {
			return nil, err
		}
		switch e.ToStatus {
		case "PARTY_STATUS_ACTIVE":
			return []Message{{PartyID: e.PartyId, Channel: ChannelEmail, Subject: "Your account is active",
				Body: "Your FORGE account is active. You can now make payments within your limits."}}, nil
		case "PARTY_STATUS_REJECTED", "PARTY_STATUS_SUSPENDED":
			return []Message{{PartyID: e.PartyId, Channel: ChannelEmail, Subject: "Action needed on your account",
				Body: "We need to speak to you about your FORGE account. Please sign in to see what is needed."}}, nil
		}
		return nil, ErrNotForUs
	case TopicRevoked:
		var e eventsv1.ConsentChanged
		if err := events.Decode(env, TopicRevoked, &e); err != nil {
			return nil, err
		}
		return []Message{{PartyID: e.PartyId, Channel: ChannelEmail, Subject: "We have stopped using your data",
			Body: fmt.Sprintf("You withdrew consent for %s. We have stopped using your data for that purpose.", e.Purpose)}}, nil
	}
	return nil, ErrNotForUs
}

func parseAmount(minor, asset string) (string, error) {
	v, err := strconv.ParseInt(minor, 10, 64)
	if err != nil || v < 0 {
		return "", fmt.Errorf("templates: amount %q is not a non-negative integer", minor)
	}
	return Money(v, asset), nil
}

// reasonText turns a machine reason into a sentence fragment the customer can act on.
func reasonText(reason string) string {
	switch {
	case strings.Contains(reason, "declined") || strings.Contains(reason, "insufficient"):
		return "insufficient available funds or a limit was reached"
	case strings.Contains(reason, "rail"):
		return "the bank did not accept the payment"
	case reason == "":
		return "a reason was not recorded"
	}
	return "it could not be processed"
}
