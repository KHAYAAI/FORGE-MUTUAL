module github.com/khayaai/forge-mutual/pkg/callerauth

go 1.27.4

require (
	connectrpc.com/connect v1.18.1
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
)

require (
	golang.org/x/net v0.62.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/khayaai/forge-mutual/pkg/api => ../api
