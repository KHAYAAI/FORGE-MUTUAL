// Command devidp runs the development identity provider. Local stacks and tests use it; it refuses production.
package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/khayaai/forge-mutual/tools/devidp"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("devidp stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	issuer := os.Getenv("DEVIDP_ISSUER")
	if issuer == "" {
		return errors.New("DEVIDP_ISSUER is required: the URL the apps use for this provider")
	}
	redirects := strings.Split(os.Getenv("DEVIDP_REDIRECT_URIS"), ",")
	p, err := devidp.New(os.Getenv("FORGE_ENV"), devidp.Config{
		Issuer:       issuer,
		Audience:     getenv("DEVIDP_AUDIENCE", "forge"),
		RedirectURIs: redirects,
		AccessTTL:    15 * time.Minute,
	})
	if err != nil {
		return err
	}
	addr := getenv("DEVIDP_ADDR", ":9000")
	log.Info("devidp listening", "addr", addr, "issuer", issuer)
	srv := &http.Server{Addr: addr, Handler: p.Handler(), ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
