package cards

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

func slogDiscard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func ledgerv1ConnectHandler(h ledgerv1connect.LedgerServiceHandler) (string, http.Handler) {
	return ledgerv1connect.NewLedgerServiceHandler(h)
}

func ledgerClient(srv *httptest.Server) ledgerv1connect.LedgerServiceClient {
	return ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL)
}
