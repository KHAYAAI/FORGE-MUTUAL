// Package metrics exports gateway RPC metrics for Prometheus (ARCHITECTURE.md section 22).
// Labels hold the RPC name, the outcome and the rejection code. They never hold account,
// party or transfer IDs, so cardinality stays bounded and no personal data reaches the metrics.
package metrics

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	ledgerv1 "github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/gen/forge/ledger/v1"
)

// Collector holds the gateway's metrics. Register it once with a registry.
type Collector struct {
	rpcs       *prometheus.CounterVec
	latency    *prometheus.HistogramVec
	rejections *prometheus.CounterVec
}

// New creates the metrics. Call Register on a registry before serving.
func New() *Collector {
	return &Collector{
		rpcs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ledger_rpc_total", Help: "Gateway RPCs by method and outcome.",
		}, []string{"rpc", "outcome"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ledger_rpc_duration_seconds", Help: "Gateway RPC latency.",
			Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		}, []string{"rpc"}),
		rejections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ledger_rejections_total", Help: "Business rejections by code.",
		}, []string{"rpc", "code"}),
	}
}

// Register adds the collectors to r.
func (c *Collector) Register(r prometheus.Registerer) {
	r.MustRegister(c.rpcs, c.latency, c.rejections)
}

// Interceptor records every unary RPC. A response with a status field tells us whether the
// request was OK, replayed or rejected; a Connect error counts as "error".
func (c *Collector) Interceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			rpc := req.Spec().Procedure[lastSlash(req.Spec().Procedure)+1:]
			start := time.Now()
			resp, err := next(ctx, req)
			c.latency.WithLabelValues(rpc).Observe(time.Since(start).Seconds())
			if err != nil {
				c.rpcs.WithLabelValues(rpc, "error").Inc()
				return resp, err
			}
			outcome, codes := outcomeOf(resp.Any())
			c.rpcs.WithLabelValues(rpc, outcome).Inc()
			for _, code := range codes {
				c.rejections.WithLabelValues(rpc, code).Inc()
			}
			return resp, nil
		}
	}
}

func outcomeOf(msg any) (string, []string) {
	m, ok := msg.(proto.Message)
	if !ok {
		return "ok", nil
	}
	r := m.ProtoReflect()
	fd := r.Descriptor().Fields().ByName("status")
	if fd == nil {
		return "ok", nil
	}
	var codes []string
	if cf := r.Descriptor().Fields().ByName("rejection_codes"); cf != nil {
		list := r.Get(cf).List()
		for i := 0; i < list.Len(); i++ {
			codes = append(codes, list.Get(i).String())
		}
	}
	switch protoreflect.EnumNumber(r.Get(fd).Enum()) {
	case protoreflect.EnumNumber(ledgerv1.Status_STATUS_REPLAYED):
		return "replayed", nil
	case protoreflect.EnumNumber(ledgerv1.Status_STATUS_REJECTED):
		return "rejected", codes
	default:
		return "ok", nil
	}
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
