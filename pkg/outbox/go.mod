module github.com/khayaai/forge-mutual/pkg/outbox

go 1.27.4

require (
	github.com/jackc/pgx/v5 v5.13.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
	github.com/twmb/franz-go v1.18.1
	github.com/twmb/franz-go/pkg/kadm v1.15.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/klauspost/compress v1.20.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.9.0 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace github.com/khayaai/forge-mutual/pkg/uuid => ../uuid
