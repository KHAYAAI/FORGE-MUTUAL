// Package credit is the arithmetic of lending, in integers and exact rationals. It is pure.
//
// A term loan is repaid in equal monthly instalments (an annuity). Each month's interest is the outstanding
// principal times rate/12, rounded down, so rounding never costs the customer; the last instalment clears whatever
// principal is left. Revenue-based financing is an advance plus a fixed fee, repaid as a share of each payment
// received. Affordability here is a placeholder rule for the credit policy owner to replace (docs/THRESHOLDS.md).
package credit

import (
	"errors"
	"math/big"
)

// ErrInvalid is returned for an amount, rate or term that cannot be priced.
var ErrInvalid = errors.New("credit: the amount and term must be positive and the rate 0 to 100%")

// Instalment is one scheduled repayment.
type Instalment struct {
	Number    int
	Principal int64
	Interest  int64
}

// Total is what the instalment collects.
func (i Instalment) Total() int64 { return i.Principal + i.Interest }

// Schedule returns the instalments for a term loan of principal (minor units) at an annual rate over months.
func Schedule(principal int64, rateBps uint32, months int) ([]Instalment, error) {
	if principal <= 0 || months <= 0 || months > 360 || rateBps > 10000 {
		return nil, ErrInvalid
	}
	// r = rateBps / 10000 / 12, exactly.
	r := new(big.Rat).SetFrac64(int64(rateBps), 120000)
	payment := new(big.Rat)
	if rateBps == 0 {
		payment.SetFrac64(principal, int64(months))
	} else {
		// payment = P * r / (1 - (1 + r)^-n) = P * r * (1+r)^n / ((1+r)^n - 1)
		onePlusR := new(big.Rat).Add(big.NewRat(1, 1), r)
		pow := big.NewRat(1, 1)
		for i := 0; i < months; i++ {
			pow.Mul(pow, onePlusR)
		}
		num := new(big.Rat).Mul(new(big.Rat).SetInt64(principal), r)
		num.Mul(num, pow)
		den := new(big.Rat).Sub(pow, big.NewRat(1, 1))
		payment.Quo(num, den)
	}
	// Round the payment up to a whole cent, so the schedule repays within the term.
	pay := ceil(payment)
	out := make([]Instalment, 0, months)
	balance := principal
	for n := 1; n <= months; n++ {
		interest := floor(new(big.Rat).Mul(new(big.Rat).SetInt64(balance), r))
		p := pay - interest
		if n == months || p > balance {
			p = balance
		}
		out = append(out, Instalment{Number: n, Principal: p, Interest: interest})
		balance -= p
		if balance == 0 {
			break
		}
	}
	return out, nil
}

// TotalRepayable is the sum of every instalment: the principal plus the total interest.
func TotalRepayable(s []Instalment) int64 {
	var t int64
	for _, i := range s {
		t += i.Total()
	}
	return t
}

// Fee is the fixed fee on a revenue-based advance, rounded down.
func Fee(advance int64, feeBps uint32) int64 {
	return mulBps(advance, feeBps)
}

// Sweep is the share of a payment received that goes to repay a revenue-based advance, never more than owed.
// It returns the fee part and the principal part, in proportion to what is owed of each.
func Sweep(received int64, sweepBps uint32, principalOwed, feeOwed int64) (principal, fee int64) {
	owed := principalOwed + feeOwed
	if received <= 0 || owed <= 0 {
		return 0, 0
	}
	take := min(mulBps(received, sweepBps), owed)
	fee = take * feeOwed / owed
	return take - fee, fee
}

// Interest on an overdraft for one day, in nano-cents (ACT/365), rounded down: the same rule as deposit interest.
func DailyNano(drawn int64, rateBps uint32) int64 {
	if drawn <= 0 {
		return 0
	}
	n := new(big.Int).Mul(big.NewInt(drawn), big.NewInt(int64(rateBps)))
	n.Mul(n, big.NewInt(1_000_000_000))
	n.Quo(n, big.NewInt(10000*365))
	return n.Int64()
}

// Affordability multiples of monthly inflow, per product. Placeholders for the credit policy owner.
var Affordability = map[string]int64{"overdraft": 1, "term-loan": 3, "revenue-based": 2}

// SuggestedMax is the most the placeholder rule allows for a product, given 90 days of money received.
func SuggestedMax(product string, inflow90d int64) int64 {
	return inflow90d / 3 * Affordability[product]
}

func mulBps(v int64, bps uint32) int64 {
	n := new(big.Int).Mul(big.NewInt(v), big.NewInt(int64(bps)))
	return n.Quo(n, big.NewInt(10000)).Int64()
}

func floor(r *big.Rat) int64 { return new(big.Int).Quo(r.Num(), r.Denom()).Int64() }

func ceil(r *big.Rat) int64 {
	q, m := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	if m.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return q.Int64()
}
