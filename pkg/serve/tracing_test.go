package serve

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// installRecorder makes a tracer that keeps every span, for the test's duration.
func installRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

// An inbound request continues the caller's trace: its span shares the trace ID, and the access log records it.
func TestInboundRequestJoinsTheCallersTrace(t *testing.T) {
	sr := installRecorder(t)
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h, err := New(Config{Addr: ":0"}, http.HandlerFunc(ok), log)
	if err != nil {
		t.Fatal(err)
	}
	const callerTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
	r := httptest.NewRequest("GET", "/v1/x", nil)
	r.Header.Set("traceparent", "00-"+callerTrace+"-00f067aa0ba902b7-01")
	h.Handler.ServeHTTP(httptest.NewRecorder(), r)

	found := false
	for _, s := range sr.Ended() {
		if s.SpanContext().TraceID().String() == callerTrace {
			found = true
		}
	}
	if !found {
		t.Fatalf("no span in the caller's trace %s; spans: %d", callerTrace, len(sr.Ended()))
	}
	if !strings.Contains(logs.String(), `"trace_id":"`+callerTrace+`"`) {
		t.Fatalf("access log does not record the trace id: %s", logs.String())
	}
}

// An outbound call made from a traced context sends the trace context to the callee.
func TestOutboundCarriesTheTraceContext(t *testing.T) {
	installRecorder(t)
	var gotParent string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotParent = r.Header.Get("traceparent")
		ok(w, r)
	}))
	t.Cleanup(backend.Close)

	client, err := Outbound(Config{})
	if err != nil {
		t.Fatal(err)
	}
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "caller")
	defer span.End()
	req, _ := http.NewRequestWithContext(ctx, "GET", backend.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	want := span.SpanContext().TraceID().String()
	if !strings.Contains(gotParent, want) {
		t.Fatalf("traceparent = %q, want it to carry trace %s", gotParent, want)
	}
	if !trace.SpanContextFromContext(ctx).IsValid() {
		t.Fatal("test span is not valid")
	}
}
