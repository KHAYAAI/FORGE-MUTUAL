module github.com/khayaai/forge-mutual/services/bank/payments-orchestrator

go 1.27.4

require (
	connectrpc.com/connect v1.18.1
	github.com/jackc/pgx/v5 v5.13.0
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/banklayer v0.0.0
	github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap v0.0.0
	github.com/khayaai/forge-mutual/pkg/callerauth v0.0.0
	github.com/khayaai/forge-mutual/pkg/events v0.0.0
	github.com/khayaai/forge-mutual/pkg/outbox v0.0.0
	github.com/khayaai/forge-mutual/pkg/screening v0.0.0
	github.com/khayaai/forge-mutual/pkg/serve v0.0.0
	github.com/khayaai/forge-mutual/pkg/systemaccounts v0.0.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
	go.temporal.io/api v1.64.3
	go.temporal.io/sdk v1.51.0
	go.temporal.io/sdk/contrib/opentelemetry v0.9.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/klauspost/compress v1.20.4 // indirect
	github.com/nexus-rpc/nexus-proto-annotations v0.1.0 // indirect
	github.com/nexus-rpc/sdk-go v0.7.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	github.com/twmb/franz-go v1.18.1 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.9.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.85.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api

replace github.com/khayaai/forge-mutual/pkg/callerauth => ../../../pkg/callerauth

replace github.com/khayaai/forge-mutual/pkg/outbox => ../../../pkg/outbox

replace github.com/khayaai/forge-mutual/pkg/screening => ../../../pkg/screening

replace github.com/khayaai/forge-mutual/pkg/uuid => ../../../pkg/uuid

replace github.com/khayaai/forge-mutual/pkg/banklayer => ../../../pkg/banklayer

replace github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap => ../../../pkg/banklayer/bootstrap

replace github.com/khayaai/forge-mutual/pkg/systemaccounts => ../../../pkg/systemaccounts

replace github.com/khayaai/forge-mutual/pkg/events => ../../../pkg/events

replace github.com/khayaai/forge-mutual/pkg/serve => ../../../pkg/serve
