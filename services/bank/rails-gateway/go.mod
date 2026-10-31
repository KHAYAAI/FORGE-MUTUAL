module github.com/khayaai/forge-mutual/services/bank/rails-gateway

go 1.24.6

require (
	connectrpc.com/connect v1.18.1
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/callerauth v0.0.0
	github.com/khayaai/forge-mutual/pkg/money v0.0.0
	golang.org/x/net v0.23.0
	google.golang.org/protobuf v1.36.6
)

require golang.org/x/text v0.14.0 // indirect

replace github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api

replace github.com/khayaai/forge-mutual/pkg/callerauth => ../../../pkg/callerauth

replace github.com/khayaai/forge-mutual/pkg/money => ../../../pkg/money
