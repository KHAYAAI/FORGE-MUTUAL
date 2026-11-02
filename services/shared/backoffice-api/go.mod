module github.com/khayaai/forge-mutual/services/shared/backoffice-api

go 1.24.6

require (
	connectrpc.com/connect v1.18.1
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
)

require google.golang.org/protobuf v1.36.11 // indirect

replace github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api
