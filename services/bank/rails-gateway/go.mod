module github.com/khayaai/forge-mutual/services/bank/rails-gateway

go 1.24.6

require (
	connectrpc.com/connect v1.18.1
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/callerauth v0.0.0
	github.com/khayaai/forge-mutual/pkg/money v0.0.0
	google.golang.org/protobuf v1.36.8
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.1 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250825161204-c5933d9347a5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250825161204-c5933d9347a5 // indirect
	google.golang.org/grpc v1.75.0 // indirect
)

require (
	github.com/khayaai/forge-mutual/pkg/serve v0.0.0
	golang.org/x/text v0.33.0 // indirect
)

replace github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api

replace github.com/khayaai/forge-mutual/pkg/callerauth => ../../../pkg/callerauth

replace github.com/khayaai/forge-mutual/pkg/money => ../../../pkg/money

replace github.com/khayaai/forge-mutual/pkg/serve => ../../../pkg/serve
