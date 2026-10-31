// Package sponsorhttp is the HTTP boundary between Forge and a sponsor bank: a server that exposes a BankLayer
// (the sponsor simulator uses it) and a client that implements BankLayer for Forge services.
//
// The wire format is Forge's own, for the simulator. The real sponsor's API is unknown and not verified
// (VERIFY-SPONSOR); when it is confirmed, only this package changes, and the contract suite must still pass.
package sponsorhttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
)

const maxBody = 1 << 20

// Handler serves the sponsor's v1 API over HTTP. It never mounts admin routes, so it is safe to put in front of a
// real sponsor adapter.
func Handler(b *sim.Bank) http.Handler {
	s := &server{bank: b}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/capabilities", s.capabilities)
	mux.HandleFunc("POST /v1/accounts", s.openAccount)
	mux.HandleFunc("POST /v1/accounts/{id}/close", s.closeAccount)
	mux.HandleFunc("POST /v1/payments", s.pay)
	mux.HandleFunc("GET /v1/payments/{id}", s.payStatus)
	mux.HandleFunc("GET /v1/statements/{asset}", s.statement)
	mux.HandleFunc("GET /v1/inbound/{asset}", s.inbound)
	mux.HandleFunc("POST /v1/insurance/{asset}", s.insurance)
	mux.HandleFunc("POST /v1/prudential", s.prudential)
	mux.HandleFunc("POST /v1/partner/usd/accounts", s.partnerOpen)
	mux.HandleFunc("POST /v1/partner/usd/payouts", s.partnerPay)
	mux.HandleFunc("GET /v1/partner/usd/payouts/{id}", s.partnerPayStatus)
	mux.HandleFunc("GET /v1/partner/usd/statements", s.partnerStatement)
	mux.HandleFunc("GET /v1/partner/usd/inbound", s.partnerInbound)
	return mux
}

// SimulatorHandler is Handler plus the admin route that injects inbound credits. Only the simulator serves it.
// It must never be mounted in a shared environment.
func SimulatorHandler(b *sim.Bank) http.Handler {
	s := &server{bank: b}
	mux := http.NewServeMux()
	mux.Handle("/", Handler(b))
	mux.HandleFunc("POST /sim/inbound", s.injectInbound)
	return mux
}

type server struct{ bank *sim.Bank }

// wireError carries the sentinel so the client can map it back with errors.Is.
type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeErr(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL"
	switch {
	case errors.Is(err, banklayer.ErrInvalid):
		status, code = http.StatusBadRequest, "INVALID"
	case errors.Is(err, banklayer.ErrNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, banklayer.ErrConflict):
		status, code = http.StatusConflict, "CONFLICT"
	case errors.Is(err, banklayer.ErrNotSupported):
		status, code = http.StatusNotImplemented, "NOT_SUPPORTED"
	case errors.Is(err, banklayer.ErrClosed):
		status, code = http.StatusConflict, "CLOSED"
	}
	writeJSON(w, status, wireError{Code: code, Message: err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.Join(banklayer.ErrInvalid, err)
	}
	return nil
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, errors.Join(banklayer.ErrInvalid, err)
	}
	return t, nil
}

func (s *server) capabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.bank.Capabilities())
}

func (s *server) openAccount(w http.ResponseWriter, r *http.Request) {
	var req banklayer.OpenAccountRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	acc, err := s.bank.OpenAccount(r.Context(), req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, acc)
}

func (s *server) closeAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.bank.CloseAccount(r.Context(), r.PathValue("id"), body.Reason); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) pay(w http.ResponseWriter, r *http.Request) {
	var req banklayer.PayRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	res, err := s.bank.Rails().Pay(r.Context(), req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *server) payStatus(w http.ResponseWriter, r *http.Request) {
	res, err := s.bank.Rails().Status(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *server) statement(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		writeErr(w, err)
		return
	}
	st, err := s.bank.StatementSource().Statement(r.Context(), r.PathValue("asset"), date)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *server) inbound(w http.ResponseWriter, r *http.Request) {
	since, err := time.Parse(time.RFC3339, r.URL.Query().Get("since"))
	if err != nil {
		writeErr(w, errors.Join(banklayer.ErrInvalid, err))
		return
	}
	list, err := s.bank.StatementSource().Inbound(r.Context(), r.PathValue("asset"), since)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) insurance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AsOf         time.Time `json:"as_of"`
		InsuredMinor int64     `json:"insured_minor"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.bank.DepositInsurance().Report(r.Context(), body.AsOf, r.PathValue("asset"), body.InsuredMinor); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) prudential(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Period string          `json:"period"`
		Body   json.RawMessage `json:"body"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.bank.PrudentialReporter().Submit(r.Context(), body.Period, body.Body); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) partner(w http.ResponseWriter) (banklayer.ForeignAccountProvider, bool) {
	p := s.bank.ForeignCurrency()
	if len(p) == 0 {
		writeErr(w, banklayer.ErrNotSupported)
		return nil, false
	}
	return p[0], true
}

func (s *server) partnerOpen(w http.ResponseWriter, r *http.Request) {
	p, ok := s.partner(w)
	if !ok {
		return
	}
	var req banklayer.OpenAccountRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	acc, err := p.OpenAccount(r.Context(), req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, acc)
}

func (s *server) partnerPay(w http.ResponseWriter, r *http.Request) {
	p, ok := s.partner(w)
	if !ok {
		return
	}
	var req banklayer.PayRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	res, err := p.Payout(r.Context(), req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *server) partnerPayStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := s.partner(w)
	if !ok {
		return
	}
	res, err := p.PayoutStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *server) partnerStatement(w http.ResponseWriter, r *http.Request) {
	p, ok := s.partner(w)
	if !ok {
		return
	}
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		writeErr(w, err)
		return
	}
	st, err := p.Statement(r.Context(), date)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *server) partnerInbound(w http.ResponseWriter, r *http.Request) {
	p, ok := s.partner(w)
	if !ok {
		return
	}
	since, err := time.Parse(time.RFC3339, r.URL.Query().Get("since"))
	if err != nil {
		writeErr(w, errors.Join(banklayer.ErrInvalid, err))
		return
	}
	list, err := p.Inbound(r.Context(), since)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) injectInbound(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Asset     string `json:"asset"`
		Minor     int64  `json:"minor"`
		Reference string `json:"reference"`
		Partner   bool   `json:"partner"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	m, err := s.bank.InjectInbound(body.Asset, body.Minor, body.Reference, body.Partner)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}
