//go:build tbintegration

package ledger_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gatewaytest"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/memrepo"
)

// TestGatewaySuiteTigerBeetle runs the behaviour suite against a live TigerBeetle cluster.
// Requires TB_ADDRESSES (host:port list) and TB_CLUSTER_ID. Build with -tags tbintegration.
// This is VERIFY-TB until it has been run against a real server.
func TestGatewaySuiteTigerBeetle(t *testing.T) {
	addrs := os.Getenv("TB_ADDRESSES")
	if addrs == "" {
		t.Skip("TB_ADDRESSES not set")
	}
	cluster, err := strconv.ParseUint(os.Getenv("TB_CLUSTER_ID"), 10, 64)
	if err != nil {
		t.Fatalf("TB_CLUSTER_ID: %v", err)
	}
	store, err := ledger.NewTigerBeetle(cluster, strings.Split(addrs, ","))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	gatewaytest.Run(t, func(t *testing.T) gatewaytest.Env {
		// Accounts and transfers use fresh IDs per subtest, so the cluster can be shared.
		return gatewaytest.Env{Repo: memrepo.New(), Store: store}
	})
}
