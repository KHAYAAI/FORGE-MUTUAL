module github.com/khayaai/forge-mutual/services/shared/backoffice-api

go 1.24.6

require (
	connectrpc.com/connect v1.18.1
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.2.3
	github.com/khayaai/forge-mutual/pkg/oidc v0.0.0
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api

replace github.com/khayaai/forge-mutual/pkg/oidc => ../../../pkg/oidc
