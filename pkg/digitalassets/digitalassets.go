// Package digitalassets holds the two checks any future deposit-token or stablecoin service must pass before it may touch a
// customer: that every token in circulation is backed by reserves (proof of reserves), and that a customer's external
// wallet is well formed, tied to them, and screened before money may move to it.
//
// This package moves nothing and writes to no ledger. The architecture forbids it for now: the bank ledger rejects token
// assets unless an own licence is approved, and stablecoin services belong in a separate entity with its own ledger
// (docs/ARCHITECTURE.md, rules 6 and 8). So the invariants are written, tested and waiting, ahead of the decision
// (docs/products/DIGITAL_ASSETS.md). Anything marked VERIFY is unconfirmed against the regulator's current rules.
package digitalassets

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Attestation is the result of comparing what is owed in tokens with what backs it.
type Attestation struct {
	SupplyMinor  int64 // tokens in circulation, in the token's smallest unit
	ReserveMinor int64 // reserve assets that back them, in the same unit
	// SurplusMinor is reserve less supply. Negative means the tokens are not fully backed.
	SurplusMinor int64
	FullyBacked  bool
	// Halt is true when issuance and redemption must stop and the board be told: any shortfall at all.
	Halt bool
}

// ErrBadAmount is returned for a negative amount.
var ErrBadAmount = errors.New("digitalassets: amounts cannot be negative")

// Attest checks that reserves cover supply one to one. There is no tolerance: a shortfall of one unit halts issuance.
// A surplus is allowed (it is the bank's own money, kept as a buffer) but is reported.
func Attest(supply, reserve int64) (Attestation, error) {
	if supply < 0 || reserve < 0 {
		return Attestation{}, ErrBadAmount
	}
	a := Attestation{SupplyMinor: supply, ReserveMinor: reserve, SurplusMinor: reserve - supply}
	a.FullyBacked = reserve >= supply
	a.Halt = !a.FullyBacked
	return a, nil
}

// Wallet is an external wallet a customer asks to send to or receive from.
type Wallet struct {
	CustomerID string
	Chain      string // "ethereum", "polygon", "base" (EVM chains only here)
	Address    string
	// OwnershipProof is a signature of a message Forge chose, made with the wallet's key: proof the customer controls it.
	OwnershipProof string
	// Provider is the hosted wallet or exchange that holds it, if not self-hosted, which matters for the travel rule.
	Provider string
	// ScreenedAt is when the address was last screened for sanctions and illicit exposure; zero means never.
	ScreenedAt time.Time
	Clear      bool // the last screening found nothing
}

var evmAddress = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

var supported = map[string]bool{"ethereum": true, "polygon": true, "base": true}

// Errors a wallet check returns, so a caller can tell what is missing.
var (
	ErrChain       = errors.New("digitalassets: unsupported chain")
	ErrAddress     = errors.New("digitalassets: not a valid address")
	ErrNoProof     = errors.New("digitalassets: no proof that the customer controls this wallet")
	ErrNotScreened = errors.New("digitalassets: the address has not been screened recently")
	ErrNotClear    = errors.New("digitalassets: the address did not pass screening")
)

// MaxScreeningAge is how fresh a screening must be before money moves (PROPOSED: 24 hours).
const MaxScreeningAge = 24 * time.Hour

// Usable reports whether money may move to or from the wallet now. It fails closed: no proof, no screening, a stale
// screening or a bad screening result each refuse.
func (w Wallet) Usable(now time.Time) error {
	if !supported[strings.ToLower(w.Chain)] {
		return fmt.Errorf("%w: %q", ErrChain, w.Chain)
	}
	if !evmAddress.MatchString(w.Address) || strings.EqualFold(w.Address, "0x0000000000000000000000000000000000000000") {
		return ErrAddress
	}
	if strings.TrimSpace(w.OwnershipProof) == "" {
		return ErrNoProof
	}
	if w.ScreenedAt.IsZero() || now.Sub(w.ScreenedAt) > MaxScreeningAge {
		return ErrNotScreened
	}
	if !w.Clear {
		return ErrNotClear
	}
	return nil
}

// TravelRuleThresholdUSD is the value above which originator and beneficiary details must travel with a crypto-asset
// transfer (the FATF figure is 1 000 US dollars or euros; South Africa's FIC directive for crypto-asset transfers
// follows it, VERIFY the current figure and effective date).
const TravelRuleThresholdUSD = 1000

// Party is one side of a transfer, as the travel rule needs it.
type Party struct {
	Name    string
	Account string // wallet address or account reference
	// Identifier: national ID or registration number, or date and place of birth for a person; sealed in storage.
	Identifier string
}

// ErrTravelRule is returned when required originator or beneficiary information is missing.
var ErrTravelRule = errors.New("digitalassets: originator and beneficiary information is required for this transfer")

// TravelRule checks that a transfer of valueUSD carries the information the rule needs. Below the threshold only the
// names and accounts are needed; at or above it the originator's identifier is needed too. It checks completeness, not
// truth: the customer's details were verified at onboarding (Didit), the counterparty's cannot be.
func TravelRule(valueUSD int64, originator, beneficiary Party) error {
	if strings.TrimSpace(originator.Name) == "" || strings.TrimSpace(originator.Account) == "" ||
		strings.TrimSpace(beneficiary.Name) == "" || strings.TrimSpace(beneficiary.Account) == "" {
		return ErrTravelRule
	}
	if valueUSD >= TravelRuleThresholdUSD && strings.TrimSpace(originator.Identifier) == "" {
		return fmt.Errorf("%w: the originator's identifier", ErrTravelRule)
	}
	return nil
}
