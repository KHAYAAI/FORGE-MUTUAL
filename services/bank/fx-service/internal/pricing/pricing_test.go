package pricing

import (
	"testing"
)

// 18.2500 ZAR per USD at 50 bps.
const mid = 1825000000

// Buying USD with ZAR: the customer pays the spread, and receives fewer USD than at mid.
func TestBuyingUSDCostsTheSpread(t *testing.T) {
	q, err := Price(ZAR, USD, 1825000, mid, 50) // R18,250.00
	if err != nil {
		t.Fatal(err)
	}
	atMid := int64(1825000) * Scale / mid // USD cents at the market rate: 10000 (USD 100.00)
	if q.BuyAmount >= atMid {
		t.Fatalf("buy amount %d must be below the mid amount %d", q.BuyAmount, atMid)
	}
	if q.SpreadMinor <= 0 {
		t.Fatalf("spread = %d, want positive", q.SpreadMinor)
	}
	if q.RateScaled <= mid {
		t.Fatalf("buying USD: customer rate %d must be above mid %d", q.RateScaled, mid)
	}
}

// Selling USD for ZAR: the customer receives fewer ZAR than at mid.
func TestSellingUSDReturnsLessThanMid(t *testing.T) {
	q, err := Price(USD, ZAR, 10000, mid, 50) // USD 100.00
	if err != nil {
		t.Fatal(err)
	}
	atMid := int64(10000) * mid / Scale
	if q.BuyAmount >= atMid {
		t.Fatalf("sell USD: got %d ZAR minor, must be below mid %d", q.BuyAmount, atMid)
	}
	if q.RateScaled >= mid {
		t.Fatalf("selling USD: customer rate %d must be below mid %d", q.RateScaled, mid)
	}
}

// Round-trip: buying then selling the same amount at the same spread must lose money, never make it. A pricing bug
// that gave the customer a free conversion would show up here.
func TestRoundTripNeverGainsForTheCustomer(t *testing.T) {
	for _, zar := range []int64{100, 99_999, 1_234_567, 9_999_999} {
		buy, err := Price(ZAR, USD, zar, mid, 75)
		if err != nil {
			t.Fatal(err)
		}
		sell, err := Price(USD, ZAR, buy.BuyAmount, mid, 75)
		if err != nil {
			t.Fatal(err)
		}
		if sell.BuyAmount > zar {
			t.Fatalf("round trip of %d ZAR returned %d: the customer gained", zar, sell.BuyAmount)
		}
	}
}

// The spread the bank keeps is never negative and never more than the sell amount.
func TestSpreadIsBoundedBySellAmount(t *testing.T) {
	q, err := Price(ZAR, USD, 50_000, mid, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if q.SpreadMinor < 0 || q.SpreadMinor > q.SellAmount {
		t.Fatalf("spread %d out of range for sell %d", q.SpreadMinor, q.SellAmount)
	}
}

func TestInvalidInputsAreRefused(t *testing.T) {
	cases := []struct {
		name         string
		sell, buy    string
		amount, rate int64
		spread       int64
	}{
		{"zero amount", ZAR, USD, 0, mid, 10},
		{"zero rate", ZAR, USD, 100, 0, 10},
		{"negative spread", ZAR, USD, 100, mid, -1},
		{"spread at 100 percent", ZAR, USD, 100, mid, 10000},
		{"unsupported pair", ZAR, "EUR", 100, mid, 10},
		{"same asset", USD, USD, 100, mid, 10},
	}
	for _, c := range cases {
		if _, err := Price(c.sell, c.buy, c.amount, c.rate, c.spread); err == nil {
			t.Fatalf("%s: accepted", c.name)
		}
	}
}

// A tiny amount that rounds to nothing is refused rather than booked as a zero deal.
func TestAmountThatBuysNothingIsRefused(t *testing.T) {
	if _, err := Price(ZAR, USD, 1, mid, 10); err == nil {
		t.Fatal("a one-cent conversion that delivers zero USD must be refused")
	}
}
