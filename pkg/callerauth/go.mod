module github.com/khayaai/forge-mutual/pkg/callerauth

go 1.24

require (
	connectrpc.com/connect v1.18.1
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
)

require google.golang.org/protobuf v1.36.6 // indirect

replace github.com/khayaai/forge-mutual/pkg/api => ../api
