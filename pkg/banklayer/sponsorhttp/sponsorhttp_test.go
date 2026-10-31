package sponsorhttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/contract"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sponsorhttp"
)

// The HTTP client must pass the same contract as the in-process simulator, over a real HTTP round trip.
func TestClientPassesContractOverHTTP(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Harness {
		srv := httptest.NewServer(sponsorhttp.SimulatorHandler(sim.New(nil)))
		t.Cleanup(srv.Close)
		c, err := sponsorhttp.NewClient(context.Background(), srv.URL, nil)
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		return contract.Harness{
			Bank: c,
			Inbound: func(asset string, minor int64, ref string, partner bool) error {
				return c.InjectInbound(context.Background(), asset, minor, ref, partner)
			},
		}
	})
}

// A client must refuse to start on any route other than the sponsor route.
func TestClientRefusesNonSponsorRoute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"route":"ROUTE_OWN_LICENCE","legal_form":"LEGAL_FORM_BANK","direct_clearing":true,"tokenised_deposits":false,"member_shares":false,"legal_book_of_record":"BOOK_FORGE"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	if _, err := sponsorhttp.NewClient(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("client accepted an own-licence route; it must refuse")
	}
}

// The production handler must not expose the inbound-credit admin route. Only the simulator may.
func TestProductionHandlerHasNoAdminRoute(t *testing.T) {
	srv := httptest.NewServer(sponsorhttp.Handler(sim.New(nil)))
	t.Cleanup(srv.Close)
	resp, err := http.Post(srv.URL+"/sim/inbound", "application/json", strings.NewReader(`{"asset":"ZAR","minor":1,"reference":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("production handler answered %d on /sim/inbound, want 404", resp.StatusCode)
	}
}

// The simulator handler must inject credits that the client then sees.
func TestSimulatorInjectIsVisibleToClient(t *testing.T) {
	srv := httptest.NewServer(sponsorhttp.SimulatorHandler(sim.New(nil)))
	t.Cleanup(srv.Close)
	c, err := sponsorhttp.NewClient(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.InjectInbound(ctx, "ZAR", 42, "REF-42", false); err != nil {
		t.Fatalf("inject: %v", err)
	}
	list, err := c.StatementSource().Inbound(ctx, "ZAR", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Reference != "REF-42" || list[0].AmountMinor != 42 {
		t.Fatalf("inbound = %+v", list)
	}
	var _ banklayer.BankLayer = c
}
