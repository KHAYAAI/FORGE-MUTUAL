module github.com/khayaai/forge-mutual/services/platform/consent-service

go 1.24

require (
	connectrpc.com/connect v1.18.1
	github.com/jackc/pgx/v5 v5.7.6
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/callerauth v0.0.0
	github.com/khayaai/forge-mutual/pkg/outbox v0.0.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
	golang.org/x/net v0.23.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/twmb/franz-go v1.18.1 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.9.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api

replace github.com/khayaai/forge-mutual/pkg/callerauth => ../../../pkg/callerauth

replace github.com/khayaai/forge-mutual/pkg/outbox => ../../../pkg/outbox

replace github.com/khayaai/forge-mutual/pkg/uuid => ../../../pkg/uuid
