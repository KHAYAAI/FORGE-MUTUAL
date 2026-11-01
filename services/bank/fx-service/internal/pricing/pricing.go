// Package pricing computes FX quotes in integers. It is pure. Rounding always favours the bank, so the customer is never
// shown a rate that gives more than the bank can deliver.
//
// Rates are ZAR per USD, scaled by 10^8. Amounts are minor units; both assets use two decimals, so a ratio of minor units
// is the same ratio as of major units.
package pricing

import (
	"errors"
	"math/big"
)

// Scale is 10^8, the rate scale.
const Scale = 100_000_000

// Pair is one currency pair, quoted as ZAR per USD.
const (
	USD = "USD"
	ZAR = "ZAR"
)

// Quote is one priced conversion.
type Quote struct {
	SellAsset   string
	BuyAsset    string
	SellAmount  int64 // minor units of the sell asset
	BuyAmount   int64 // minor units of the buy asset, after the spread
	MidScaled   int64 // market rate, ZAR per USD, scaled
	RateScaled  int64 // rate the customer gets, ZAR per USD, scaled
	SpreadBps   int64
	SpreadMinor int64 // the bank's spread, in the sell asset
}

// ErrInvalid is returned for amounts or rates that cannot be priced.
var ErrInvalid = errors.New("pricing: amount, rate and spread must be positive, and the pair must be ZAR and USD")

// Price prices selling sellAmount of sellAsset for buyAsset at the market rate mid, with a spread in basis points. The
// customer pays the spread: buying USD costs more ZAR than the market rate; selling USD returns fewer ZAR.
func Price(sellAsset, buyAsset string, sellAmount, mid, spreadBps int64) (Quote, error) {
	if sellAmount <= 0 || mid <= 0 || spreadBps < 0 || spreadBps >= 10000 {
		return Quote{}, ErrInvalid
	}
	if !((sellAsset == ZAR && buyAsset == USD) || (sellAsset == USD && buyAsset == ZAR)) {
		return Quote{}, ErrInvalid
	}
	q := Quote{SellAsset: sellAsset, BuyAsset: buyAsset, SellAmount: sellAmount, MidScaled: mid, SpreadBps: spreadBps}
	bps := big.NewInt(10000)
	scale := big.NewInt(Scale)
	m := big.NewInt(mid)
	sb := big.NewInt(spreadBps)

	if sellAsset == ZAR {
		// Buying USD: the customer's rate is mid * (1 + spread), so each USD costs more ZAR.
		// The rate is rounded up, against the customer, so the bank never delivers more USD than the rate implies.
		rateUp := new(big.Int).Mul(m, new(big.Int).Add(bps, sb))
		rateUp.Add(rateUp, new(big.Int).Sub(bps, big.NewInt(1)))
		rateUp.Quo(rateUp, bps)
		q.RateScaled = rateUp.Int64()
		// USD out (minor) = ZAR in (minor) * Scale / rate. Round down: the customer receives no more than the rate gives.
		out := new(big.Int).Mul(big.NewInt(sellAmount), scale)
		out.Quo(out, rateUp)
		q.BuyAmount = out.Int64()
		// The market value of the USD delivered, in ZAR, rounded up: the spread is what the customer paid above it.
		mkt := new(big.Int).Mul(big.NewInt(q.BuyAmount), m)
		mkt.Add(mkt, new(big.Int).Sub(scale, big.NewInt(1)))
		mkt.Quo(mkt, scale)
		q.SpreadMinor = sellAmount - mkt.Int64()
	} else {
		// Selling USD: the customer's rate is mid * (1 - spread), so each USD returns fewer ZAR.
		rate := new(big.Int).Mul(m, new(big.Int).Sub(bps, sb))
		rate.Quo(rate, bps) // rounds down: a lower rate is worse for the customer
		q.RateScaled = rate.Int64()
		out := new(big.Int).Mul(big.NewInt(sellAmount), new(big.Int).SetInt64(q.RateScaled))
		out.Quo(out, scale)
		q.BuyAmount = out.Int64()
		// The market value of the USD sold, in ZAR, is what it would fetch at mid. The spread is the difference, in USD
		// minor units, rounded up against the customer.
		mkt := new(big.Int).Mul(big.NewInt(q.BuyAmount), scale)
		mkt.Quo(mkt, m)
		q.SpreadMinor = sellAmount - mkt.Int64()
	}
	if q.BuyAmount <= 0 {
		return Quote{}, ErrInvalid
	}
	if q.SpreadMinor < 0 {
		q.SpreadMinor = 0
	}
	return q, nil
}
