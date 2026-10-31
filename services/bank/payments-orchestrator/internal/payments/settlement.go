package payments

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// systemNamespace names the business transaction used to create system accounts.
const systemNamespace = "0b6f5d3a-2c7e-5e91-a8d4-6f1e3c9b7a52"

// EnsureSettlementAccount returns the ledger account of RAIL_SETTLEMENT_EFT (code 3001), creating it on first
// use. The idempotency key is fixed, so every start returns the same account: the ledger replays the stored
// result.
func EnsureSettlementAccount(ctx context.Context, l ledgerv1connect.LedgerServiceClient, entity string) (string, error) {
	txn, err := uuid.NewV5(systemNamespace, "rail-settlement-eft")
	if err != nil {
		return "", err
	}
	res, err := l.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: "system:rail-settlement-eft:v1",
		BusinessTxnId:  txn,
		EntityCode:     entity,
		Accounts: []*ledgerv1.AccountSpec{{
			Asset:     "ZAR",
			Code:      3001, // RAIL_SETTLEMENT_EFT
			ProductId: "rail-settlement-eft",
		}},
		Actor:  "payments",
		Reason: "ensure the rail settlement account",
	}))
	if err != nil {
		return "", err
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return "", errors.New("ledger refused the settlement account")
	}
	if len(res.Msg.AccountIds) != 1 {
		return "", errors.New("ledger returned no settlement account")
	}
	return res.Msg.AccountIds[0], nil
}
