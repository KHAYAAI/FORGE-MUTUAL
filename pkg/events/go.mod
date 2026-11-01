module github.com/khayaai/forge-mutual/pkg/events

go 1.24.6

require (
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
	google.golang.org/protobuf v1.36.11
)

replace (
	github.com/khayaai/forge-mutual/pkg/api => ../api
	github.com/khayaai/forge-mutual/pkg/uuid => ../uuid
)
