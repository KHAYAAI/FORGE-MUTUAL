module github.com/khayaai/forge-mutual/services/shared/compliance-engine

go 1.24.6

require (
	github.com/jackc/pgx/v5 v5.7.6
	github.com/khayaai/forge-mutual/pkg/screening v0.0.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace (
	github.com/khayaai/forge-mutual/pkg/outbox => ../../../pkg/outbox
	github.com/khayaai/forge-mutual/pkg/screening => ../../../pkg/screening
	github.com/khayaai/forge-mutual/pkg/uuid => ../../../pkg/uuid
)
