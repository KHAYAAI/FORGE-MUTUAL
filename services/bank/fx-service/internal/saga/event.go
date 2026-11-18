package saga

import (
	"strconv"
	"time"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/store"
)

// bookedEvent is written in the same transaction as the completed state, so a completed deal always has its event.
func bookedEvent(dealID string, d store.Deal, entity string, at time.Time) outbox.Event {
	id, payload, err := events.Wrap(events.Params{Type: Topic, Entity: entity, Producer: "fx-service", Correlation: dealID, At: at},
		&eventsv1.FXBooked{DealId: dealID, PartyId: d.PartyID, SellAsset: d.SellAsset, BuyAsset: d.BuyAsset,
			SellAmountMinor: strconv.FormatInt(d.SellAmount, 10), BuyAmountMinor: strconv.FormatInt(d.BuyAmount, 10),
			SpreadMinor: strconv.FormatInt(d.SpreadMinor, 10)})
	if err != nil {
		panic(err) // a fixed message type always marshals
	}
	return outbox.Event{EventID: id, Topic: Topic, Key: []byte(dealID), Payload: payload}
}
