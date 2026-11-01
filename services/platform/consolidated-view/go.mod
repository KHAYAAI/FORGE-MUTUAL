module github.com/khayaai/forge-mutual/services/platform/consolidated-view

go 1.24.6

require (
	connectrpc.com/connect v1.18.1
	github.com/jackc/pgx/v5 v5.7.6
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/callerauth v0.0.0-00010101000000-000000000000
	github.com/khayaai/forge-mutual/pkg/eventbus v0.0.0-00010101000000-000000000000
	github.com/khayaai/forge-mutual/pkg/events v0.0.0
)

require (
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.9.0 // indirect
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/twmb/franz-go v1.18.1 // indirect
	github.com/twmb/franz-go/pkg/kadm v1.15.0 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/net v0.49.0
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	github.com/khayaai/forge-mutual/pkg/api => ../../../pkg/api
	github.com/khayaai/forge-mutual/pkg/callerauth => ../../../pkg/callerauth
	github.com/khayaai/forge-mutual/pkg/eventbus => ../../../pkg/eventbus
	github.com/khayaai/forge-mutual/pkg/events => ../../../pkg/events
	github.com/khayaai/forge-mutual/pkg/uuid => ../../../pkg/uuid
)
