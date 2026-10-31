module github.com/khayaai/forge-mutual/services/bank/treasury-risk

go 1.24.6

require (
	connectrpc.com/connect v1.18.1
	github.com/jackc/pgx/v5 v5.7.6
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/banklayer v0.0.0
	github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap v0.0.0
	github.com/khayaai/forge-mutual/pkg/outbox v0.0.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
	google.golang.org/protobuf v1.36.11
)

replace (
	github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api
	github.com/khayaai/forge-mutual/pkg/banklayer => ../../../pkg/banklayer
	github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap => ../../../pkg/banklayer/bootstrap
	github.com/khayaai/forge-mutual/pkg/outbox => ../../../pkg/outbox
	github.com/khayaai/forge-mutual/pkg/uuid => ../../../pkg/uuid
)
