// Package events wraps every published message in the EventEnvelope from ARCHITECTURE.md 18.2. Producers call Wrap;
// consumers call Open and then Decode, and check that the envelope's type is the topic they subscribed to. A bare
// message does not carry a type, so it is refused rather than guessed at.
package events

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// Params describe one event. EventID may be empty, in which case a new UUID is generated.
type Params struct {
	EventID     string
	Type        string // the topic, which is also the event type
	Entity      string // PLAT, BANK, SHARED
	Producer    string
	Correlation string
	Causation   string
	At          time.Time
}

// Wrap returns the event id and the encoded envelope. The outbox row must carry the same id.
func Wrap(p Params, msg proto.Message) (string, []byte, error) {
	if p.Type == "" || p.Entity == "" || p.Producer == "" {
		return "", nil, errors.New("events: type, entity and producer are required")
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return "", nil, err
	}
	id := p.EventID
	if id == "" {
		if id, err = uuid.NewV4(); err != nil {
			return "", nil, err
		}
	}
	at := p.At
	if at.IsZero() {
		at = time.Now()
	}
	env, err := proto.Marshal(&eventsv1.EventEnvelope{
		EventId:       id,
		EventType:     p.Type,
		SchemaVersion: 1,
		EntityCode:    p.Entity,
		CorrelationId: p.Correlation,
		CausationId:   p.Causation,
		Producer:      p.Producer,
		OccurredAt:    timestamppb.New(at.UTC()),
		Payload:       payload,
	})
	if err != nil {
		return "", nil, err
	}
	return id, env, nil
}

// Open parses an envelope and refuses one without a type or id.
func Open(data []byte) (*eventsv1.EventEnvelope, error) {
	var env eventsv1.EventEnvelope
	if err := proto.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("events: not an envelope: %w", err)
	}
	if env.EventType == "" || env.EventId == "" {
		return nil, errors.New("events: envelope has no type or id")
	}
	return &env, nil
}

// Decode unpacks the payload into msg, after checking that it is the type the consumer expects.
func Decode(env *eventsv1.EventEnvelope, wantType string, msg proto.Message) error {
	if env.EventType != wantType {
		return fmt.Errorf("events: got %s, want %s", env.EventType, wantType)
	}
	return proto.Unmarshal(env.Payload, msg)
}
