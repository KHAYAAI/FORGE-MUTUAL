package gateway

import (
	"strings"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

// Rejection codes returned to callers (stable strings; ARCHITECTURE.md section 19 style).
const (
	CodeEntityMismatch      = "ENTITY_MISMATCH"
	CodeAssetNotAllowed     = "ASSET_NOT_ALLOWED"
	CodeCodeNotAllowed      = "CODE_NOT_ALLOWED"
	CodeAccountNotFound     = "ACCOUNT_NOT_FOUND"
	CodeAccountNotInEntity  = "ACCOUNT_NOT_IN_ENTITY"
	CodeAssetMismatch       = "ASSET_MISMATCH"
	CodeAmountLimitExceeded = "AMOUNT_LIMIT_EXCEEDED"
	CodeTimeoutRequired     = "TIMEOUT_REQUIRED"
	CodeTimeoutLimit        = "TIMEOUT_LIMIT_EXCEEDED"
	CodeTimeoutNotPending   = "TIMEOUT_ON_NON_PENDING"
	CodeTooManyLegs         = "TOO_MANY_LEGS"
	CodeNoLegs              = "NO_LEGS"
	CodeSameAccount         = "SAME_ACCOUNT"
	CodePendingNotFound     = "PENDING_NOT_FOUND"
	CodeAccountConflict     = "ACCOUNT_CONFLICT"
	CodeInsufficientFunds   = "INSUFFICIENT_FUNDS"
	CodeLinkedEventFailed   = "LINKED_EVENT_FAILED"
	CodeInvalidAmount       = "INVALID_AMOUNT"
)

// rejectionCode maps a ledger reason to the code returned to callers.
func rejectionCode(r ledger.Reason) string {
	switch r {
	case ledger.ReasonInsufficientFunds:
		return CodeInsufficientFunds
	case ledger.ReasonAccountNotFound:
		return CodeAccountNotFound
	case ledger.ReasonSameAccount:
		return CodeSameAccount
	case ledger.ReasonPendingNotFound:
		return CodePendingNotFound
	case ledger.ReasonLinkedEventFailed:
		return CodeLinkedEventFailed
	case ledger.ReasonInvalidAmount:
		return CodeInvalidAmount
	case ledger.ReasonExistsWithDifferentFields:
		return "EXISTS_WITH_DIFFERENT_FIELDS"
	default:
		return strings.ToUpper(string(r))
	}
}
