// Package interest computes deposit interest in integers. Interest accrues daily, on the day's closing balance, at
// rate_bps / 10000 / 365 of the balance (ACT/365, the convention South African banks quote on). It is held in
// nano-cents (10^-9 of a minor unit) so a small balance earns something every day rather than rounding to nothing,
// and is paid monthly in whole cents. What a payment rounds away is carried into the next month, so over time the
// customer is paid every nano-cent they earned and never more.
package interest

import (
	"errors"
	"math/big"
)

// Nano is the number of accrual units in one minor unit.
const Nano = 1_000_000_000

// DaysInYear is the day count: ACT/365, the same in a leap year.
const DaysInYear = 365

// ErrInvalid is returned for a negative balance or a rate outside 0 to 100%.
var ErrInvalid = errors.New("interest: the balance must not be negative and the rate must be 0 to 10000 basis points")

// Daily is one day's interest on a closing balance, in nano-cents, rounded down.
func Daily(balanceMinor int64, rateBps uint32) (int64, error) {
	if balanceMinor < 0 || rateBps > 10000 {
		return 0, ErrInvalid
	}
	n := new(big.Int).Mul(big.NewInt(balanceMinor), big.NewInt(int64(rateBps)))
	n.Mul(n, big.NewInt(Nano))
	n.Quo(n, big.NewInt(10000*DaysInYear))
	if !n.IsInt64() {
		return 0, ErrInvalid
	}
	return n.Int64(), nil
}

// Payable splits what has accrued (this month's accruals plus last month's carry) into whole cents to pay now and
// the remainder to carry.
func Payable(accruedNano, carryNano int64) (payMinor, newCarryNano int64) {
	total := accruedNano + carryNano
	return total / Nano, total % Nano
}
