module github.com/khayaai/forge-mutual/services/bank/reconciliation

go 1.24.6

require (
	connectrpc.com/connect v1.18.1
	github.com/jackc/pgx/v5 v5.7.6
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/banklayer v0.0.0
	github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap v0.0.0
	github.com/khayaai/forge-mutual/pkg/callerauth v0.0.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
	golang.org/x/net v0.49.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api
	github.com/khayaai/forge-mutual/pkg/banklayer => ../../../pkg/banklayer
	github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap => ../../../pkg/banklayer/bootstrap
	github.com/khayaai/forge-mutual/pkg/callerauth => ../../../pkg/callerauth
	github.com/khayaai/forge-mutual/pkg/uuid => ../../../pkg/uuid
)
