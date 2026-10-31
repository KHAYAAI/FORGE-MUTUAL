// Package banklayer is the only place that knows whether Forge runs on a sponsor bank or under its own licence
// (ARCHITECTURE.md 5.2). No service outside this package may branch on the route. Behaviour that varies by route
// is expressed as Capabilities, which OPA reads as data (package forge.banklayer).
package banklayer

import (
	"context"
	"errors"
	"time"
)

// Route is the licence route in force.
type Route string

const (
	RouteSponsor    Route = "ROUTE_SPONSOR"
	RouteOwnLicence Route = "ROUTE_OWN_LICENCE"
)

// LegalForm is the legal form of the bank layer.
type LegalForm string

const (
	LegalFormNone   LegalForm = "LEGAL_FORM_NONE"
	LegalFormMutual LegalForm = "LEGAL_FORM_MUTUAL"
	LegalFormBank   LegalForm = "LEGAL_FORM_BANK"
)

// BookOfRecord names whose books hold the legal record of deposits.
type BookOfRecord string

const (
	BookSponsor BookOfRecord = "BOOK_SPONSOR"
	BookForge   BookOfRecord = "BOOK_FORGE"
)

// Capabilities describe what the active route allows. They are data, not branches.
type Capabilities struct {
	Route             Route        `json:"route"`
	LegalForm         LegalForm    `json:"legal_form"`
	DirectClearing    bool         `json:"direct_clearing"`
	TokenisedDeposits bool         `json:"tokenised_deposits"`
	MemberShares      bool         `json:"member_shares"`
	LegalBookOfRecord BookOfRecord `json:"legal_book_of_record"`
}

// SponsorCapabilities is the v1 route: a sponsor bank holds the deposits and Forge sits on its rails.
func SponsorCapabilities() Capabilities {
	return Capabilities{
		Route:             RouteSponsor,
		LegalForm:         LegalFormNone,
		DirectClearing:    false,
		TokenisedDeposits: false,
		MemberShares:      false,
		LegalBookOfRecord: BookSponsor,
	}
}

// Errors. Implementations wrap these so callers can use errors.Is.
var (
	ErrConflict     = errors.New("banklayer: request id reused with a different request")
	ErrNotFound     = errors.New("banklayer: not found")
	ErrInvalid      = errors.New("banklayer: invalid request")
	ErrClosed       = errors.New("banklayer: account is closed")
	ErrNotSupported = errors.New("banklayer: not supported on this route")
)

// Payment statuses reported by the bank.
const (
	StatusAccepted = "ACCEPTED"
	StatusPending  = "PENDING"
	StatusSettled  = "SETTLED"
	StatusRejected = "REJECTED"
	StatusReturned = "RETURNED"
)

// Terminal reports whether a status cannot change any more.
func Terminal(status string) bool {
	return status == StatusSettled || status == StatusRejected || status == StatusReturned
}

// Account is a receiving account. Reference is the number a payer quotes so the credit can be matched.
type Account struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Asset     string    `json:"asset"`
	Reference string    `json:"reference"`
	Status    string    `json:"status"`
	OpenedAt  time.Time `json:"opened_at"`
}

// OpenAccountRequest asks for a receiving account.
type OpenAccountRequest struct {
	Owner string `json:"owner"`
	Asset string `json:"asset"`
}

// PayRequest is an outbound payment. ID is the idempotency key chosen by the caller.
type PayRequest struct {
	ID            string `json:"id"`
	Asset         string `json:"asset"`
	AmountMinor   int64  `json:"amount_minor"`
	Beneficiary   string `json:"beneficiary"`
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
	Reference     string `json:"reference"`
}

// PayResult is the bank's answer about a payment.
type PayResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Movement is one booked credit or debit on a pool or partner account.
type Movement struct {
	ID          string    `json:"id"`
	Direction   string    `json:"direction"` // IN or OUT
	AmountMinor int64     `json:"amount_minor"`
	Asset       string    `json:"asset"`
	Reference   string    `json:"reference"`
	BookedAt    time.Time `json:"booked_at"`
}

// Statement is one UTC day's movements on the pool or partner account of an asset.
type Statement struct {
	Asset        string     `json:"asset"`
	Date         time.Time  `json:"date"`
	OpeningMinor int64      `json:"opening_minor"`
	ClosingMinor int64      `json:"closing_minor"`
	Movements    []Movement `json:"movements"`
}

// Rail sends and tracks outbound domestic payments through the sponsor.
type Rail interface {
	Pay(ctx context.Context, req PayRequest) (PayResult, error)
	Status(ctx context.Context, id string) (PayResult, error)
}

// ForeignAccountProvider is one partner bank for one foreign currency (USD in v1).
type ForeignAccountProvider interface {
	Partner() string
	Asset() string
	OpenAccount(ctx context.Context, req OpenAccountRequest) (Account, error)
	Payout(ctx context.Context, req PayRequest) (PayResult, error)
	PayoutStatus(ctx context.Context, id string) (PayResult, error)
	Statement(ctx context.Context, date time.Time) (Statement, error)
	Inbound(ctx context.Context, since time.Time) ([]Movement, error)
}

// StatementSource is what reconciliation reads from the bank. Sponsor and partner statements now; Forge's own
// settlement accounts later.
type StatementSource interface {
	Statement(ctx context.Context, asset string, date time.Time) (Statement, error)
	Inbound(ctx context.Context, asset string, since time.Time) ([]Movement, error)
}

// InsuranceReporter records the deposit-insurance position for an asset.
type InsuranceReporter interface {
	// Duty names who reports: "sponsor" on the sponsor route, "forge" once Forge holds the licence.
	Duty() string
	Report(ctx context.Context, asOf time.Time, asset string, insuredMinor int64) error
}

// PrudentialReporter files prudential returns. On the sponsor route nothing is required of Forge.
type PrudentialReporter interface {
	Required() bool
	Submit(ctx context.Context, period string, body []byte) error
}

// BankLayer is the whole bank boundary.
type BankLayer interface {
	Capabilities() Capabilities
	OpenAccount(ctx context.Context, req OpenAccountRequest) (Account, error)
	CloseAccount(ctx context.Context, accountID string, reason string) error
	Rails() Rail
	ForeignCurrency() []ForeignAccountProvider
	StatementSource() StatementSource
	DepositInsurance() InsuranceReporter
	PrudentialReporter() PrudentialReporter
}

// ValidAmount reports whether an amount is a positive count of minor units.
func ValidAmount(minor int64) bool { return minor > 0 }
