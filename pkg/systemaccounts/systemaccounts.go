// Package systemaccounts returns the ledger accounts that the bank's own flows share, such as the sponsor pool and
// suspense. Each is created under a fixed key the first time anything asks for it, so every service and every start
// gets the same account. The ledger replays the stored result for a repeated key.
package systemaccounts

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// namespace names the business transaction used to create system accounts.
const namespace = "3b1f0c2e-7d4a-5f6e-9b8c-2a1d0e9f8b7c"

// Ensure returns the account for key, creating it with the given asset and code if it does not exist. key must be
// stable: it is the idempotency key, so changing it creates a second account.
func Ensure(ctx context.Context, l ledgerv1connect.LedgerServiceClient, entity, asset string, code uint32, key string) (string, error) {
	if key == "" || asset == "" || code == 0 {
		return "", errors.New("systemaccounts: key, asset and code are required")
	}
	txn, err := uuid.NewV5(namespace, key)
	if err != nil {
		return "", err
	}
	res, err := l.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: key,
		BusinessTxnId:  txn,
		EntityCode:     entity,
		Accounts:       []*ledgerv1.AccountSpec{{Asset: asset, Code: code, ProductId: key}},
		Actor:          "system",
		Reason:         "ensure a system account",
	}))
	if err != nil {
		return "", fmt.Errorf("systemaccounts: %s: %w", key, err)
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return "", fmt.Errorf("systemaccounts: ledger refused %s with status %v", key, res.Msg.Status)
	}
	if len(res.Msg.AccountIds) != 1 {
		return "", fmt.Errorf("systemaccounts: ledger returned no account for %s", key)
	}
	return res.Msg.AccountIds[0], nil
}
