package serve

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	var last error
	for range 50 {
		res, err := http.Get(url)
		if err == nil {
			defer res.Body.Close()
			b, _ := io.ReadAll(res.Body)
			return res.StatusCode, string(b)
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(last)
	return 0, ""
}

func TestHealthIsOffUnlessAsked(t *testing.T) {
	stop, err := startHealth("", &atomic.Bool{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	stop()
	if _, err := startHealth("not-an-address", &atomic.Bool{}, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("a bad address must be refused")
	}
}

func TestProbesNeedNoCertificateAndExposeNothingElse(t *testing.T) {
	addr := freeAddr(t)
	ready := &atomic.Bool{}
	ready.Store(true)
	stop, err := startHealth(addr, ready, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if code, body := get(t, "http://"+addr+"/livez"); code != 200 || body != "ok" {
		t.Fatalf("livez = %d %q", code, body)
	}
	if code, _ := get(t, "http://"+addr+"/readyz"); code != 200 {
		t.Fatalf("readyz = %d", code)
	}
	ready.Store(false)
	if code, _ := get(t, "http://"+addr+"/readyz"); code != 503 {
		t.Fatalf("a draining service must not be ready: %d", code)
	}
	if code, _ := get(t, "http://"+addr+"/livez"); code != 200 {
		t.Fatal("a draining service is still alive")
	}
	for _, path := range []string{"/", "/metrics", "/debug/pprof/", "/v1/anything"} {
		if code, _ := get(t, "http://"+addr+path); code != 404 {
			t.Fatalf("%s must not exist on the health listener: %d", path, code)
		}
	}
	res, err := http.Post("http://"+addr+"/livez", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 405 {
		t.Fatalf("probes are GET only: %d", res.StatusCode)
	}
}

func TestRunDrainsBeforeItCloses(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv("HEALTH_ADDR", addr)
	srv := &http.Server{Addr: freeAddr(t), Handler: http.NotFoundHandler()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, srv, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
	if code, _ := get(t, "http://"+addr+"/readyz"); code != 200 {
		t.Fatalf("readyz = %d", code)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
