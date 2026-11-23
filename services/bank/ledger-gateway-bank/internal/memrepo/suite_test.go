package memrepo_test

import (
	"testing"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gatewaytest"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/memrepo"
)

func TestGatewaySuiteInMemory(t *testing.T) {
	gatewaytest.Run(t, func(t *testing.T) gatewaytest.Env {
		return gatewaytest.Env{Repo: memrepo.New(), Store: ledger.NewMemStore(nil)}
	})
}
