module github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap

go 1.24.6

require (
	github.com/khayaai/forge-mutual/pkg/banklayer v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/khayaai/forge-mutual/pkg/banklayer => ../
