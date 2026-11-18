// Package money provides integer-only money arithmetic in minor units.
// No floating point is used anywhere (ARCHITECTURE.md invariant 5).
package money

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var (
	ErrOverflow       = errors.New("money: overflow")
	ErrAssetMismatch  = errors.New("money: asset mismatch")
	ErrNegativeAmount = errors.New("money: negative amount not allowed here")
	ErrInvalidAmount  = errors.New("money: invalid amount string")
	ErrUnknownAsset   = errors.New("money: unknown asset")
)

// Asset describes a currency and its scale (decimal places in minor units).
type Asset struct {
	Code  string // ISO 4217 alphabetic code, e.g. "ZAR"
	Scale uint8  // 2 for ZAR and USD
}

// Registry is the set of assets known to the platform. ZAR and USD are active in v1
// (config/assets.yaml, ARCHITECTURE.md 7.2).
var Registry = map[string]Asset{
	"ZAR": {Code: "ZAR", Scale: 2},
	"USD": {Code: "USD", Scale: 2},
}

// Lookup returns the registered asset for code or ErrUnknownAsset.
func Lookup(code string) (Asset, error) {
	a, ok := Registry[code]
	if !ok {
		return Asset{}, fmt.Errorf("%w: %q", ErrUnknownAsset, code)
	}
	return a, nil
}

// Money is an amount in minor units of one asset. The zero value is not a valid asset;
// construct with New or Parse.
type Money struct {
	asset string
	minor int64
}

// New creates a Money value from minor units.
func New(asset string, minor int64) (Money, error) {
	if _, err := Lookup(asset); err != nil {
		return Money{}, err
	}
	return Money{asset: asset, minor: minor}, nil
}

// Parse reads a decimal string of minor units, e.g. "12345" for R123.45 in ZAR.
// It accepts only an optional leading minus and digits.
func Parse(asset, s string) (Money, error) {
	if _, err := Lookup(asset); err != nil {
		return Money{}, err
	}
	if s == "" || strings.ContainsAny(s, ". eE+") {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	return Money{asset: asset, minor: v}, nil
}

// Asset returns the asset code.
func (m Money) Asset() string { return m.asset }

// Minor returns the amount in minor units.
func (m Money) Minor() int64 { return m.minor }

// String returns the amount in minor units with the asset code, e.g. "12345 ZAR".
func (m Money) String() string { return strconv.FormatInt(m.minor, 10) + " " + m.asset }

// Add returns m + o, failing on asset mismatch or overflow.
func (m Money) Add(o Money) (Money, error) {
	if m.asset != o.asset {
		return Money{}, ErrAssetMismatch
	}
	if (o.minor > 0 && m.minor > math.MaxInt64-o.minor) || (o.minor < 0 && m.minor < math.MinInt64-o.minor) {
		return Money{}, ErrOverflow
	}
	return Money{asset: m.asset, minor: m.minor + o.minor}, nil
}

// Sub returns m - o, failing on asset mismatch or overflow.
func (m Money) Sub(o Money) (Money, error) {
	if m.asset != o.asset {
		return Money{}, ErrAssetMismatch
	}
	if (o.minor < 0 && m.minor > math.MaxInt64+o.minor) || (o.minor > 0 && m.minor < math.MinInt64+o.minor) {
		return Money{}, ErrOverflow
	}
	return Money{asset: m.asset, minor: m.minor - o.minor}, nil
}

// NonNegative returns an error if the amount is negative.
func (m Money) NonNegative() error {
	if m.minor < 0 {
		return ErrNegativeAmount
	}
	return nil
}
