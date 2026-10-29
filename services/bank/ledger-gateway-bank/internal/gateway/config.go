package gateway

import "time"

// AssetConfig is the tb-bank setup for one asset.
type AssetConfig struct {
	Ledger         uint32
	MaxAmountMinor uint64 // per leg; placeholder limit, real limits come from policy (tiers.yaml)
}

// Config is the gateway's static setup. Values here are enforcement, not policy.
type Config struct {
	Entity            string                 // entity this gateway writes for, "BANK"
	Assets            map[string]AssetConfig // assets allowed on this cluster
	AccountCodes      map[uint16]bool        // Appendix B account codes enabled in v1
	TransferCodes     map[uint16]bool        // Appendix B transfer codes enabled in v1
	MaxLegs           int
	MaxPendingTimeout uint32 // seconds
	Producer          string // service name written into event envelopes
	Clock             func() time.Time
}

// DefaultConfig returns the v1 bank setup from ARCHITECTURE.md 7.2 and Appendix B.
// Codes deferred to later phases are left out, so they are denied by default.
func DefaultConfig() Config {
	return Config{
		Entity: "BANK",
		Assets: map[string]AssetConfig{
			"ZAR": {Ledger: 710, MaxAmountMinor: 1_000_000_000}, // R10,000,000.00
			"USD": {Ledger: 840, MaxAmountMinor: 1_000_000_000}, // $10,000,000.00
		},
		AccountCodes: map[uint16]bool{
			1001: true, // CUSTOMER_DEPOSIT
			2001: true, // OPERATING
			2002: true, // FEES_INCOME
			2003: true, // FX_POOL
			2004: true, // FX_POSITION
			2005: true, // SUSPENSE
			2006: true, // WRITE_OFF
			3001: true, // RAIL_SETTLEMENT_EFT
			3002: true, // RAIL_SETTLEMENT_PAYSHAP
			3003: true, // SPONSOR_POOLED
			3010: true, // AD_PARTNER_SETTLEMENT
			5001: true, // PARTNER_BANK_MIRROR
		},
		TransferCodes: map[uint16]bool{
			10: true, // DEPOSIT_IN
			11: true, // PAYMENT_OUT
			12: true, // INTERNAL_TRANSFER
			13: true, // CROSS_BORDER_OUT
			20: true, // FX_LEG
			21: true, // FX_SPREAD
			30: true, // FEE
			31: true, // RAIL_SETTLE
			62: true, // INVOICE_PAYMENT
			63: true, // PERSON_ENTITY_TRANSFER (policy-approved upstream)
			64: true, // PARTNER_BANK_SWEEP
			90: true, // REVERSAL
			91: true, // ADJUSTMENT (dual control upstream)
		},
		MaxLegs:           100,
		MaxPendingTimeout: 7 * 24 * 60 * 60,
		Producer:          "ledger-gateway-bank",
		Clock:             time.Now,
	}
}
