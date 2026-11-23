module github.com/khayaai/forge-mutual/pkg/events

go 1.27.4

require (
	github.com/khayaai/forge-mutual/pkg/api v0.0.0
	github.com/khayaai/forge-mutual/pkg/uuid v0.0.0
	google.golang.org/protobuf v1.36.12
)

replace (
	github.com/khayaai/forge-mutual/pkg/api => ../api
	github.com/khayaai/forge-mutual/pkg/uuid => ../uuid
)
