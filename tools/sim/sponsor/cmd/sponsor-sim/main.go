// Command sponsor-sim is the sponsor and USD partner-bank simulator. It is not a bank. It refuses to start
// unless SPONSOR_SIMULATION=1 is set, and it serves the inbound-credit admin route, so it must never run in a
// shared environment.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sponsorhttp"
)

func main() {
	if os.Getenv("SPONSOR_SIMULATION") != "1" {
		log.Fatal("sponsor-sim: refusing to start; set SPONSOR_SIMULATION=1 to run the simulator")
	}
	addr := os.Getenv("SPONSOR_SIM_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8091"
	}
	bank := sim.New(nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/", sponsorhttp.SimulatorHandler(bank))
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("sponsor-sim: SIMULATION ONLY, listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
