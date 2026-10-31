package sim_test

import (
	"testing"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/contract"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
)

// The simulator must pass the same contract as every real implementation.
func TestSimPassesContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Harness {
		b := sim.New(nil)
		return contract.Harness{
			Bank: b,
			Inbound: func(asset string, minor int64, ref string, partner bool) error {
				_, err := b.InjectInbound(asset, minor, ref, partner)
				return err
			},
		}
	})
}

// Compile-time check that the simulator satisfies the whole boundary.
var _ banklayer.BankLayer = (*sim.Bank)(nil)
