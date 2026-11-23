package recon

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

// failingBalances wraps a client and fails every balance read.
type failingBalances struct {
	ledgerv1connect.LedgerServiceClient
}

func (failingBalances) GetBalances(context.Context, *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	return nil, connect.NewError(connect.CodeUnavailable, errors.New("ledger unavailable"))
}
