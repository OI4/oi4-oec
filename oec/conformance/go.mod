module github.com/OI4/oec/conformance

go 1.26.0

toolchain go1.27.2

replace (
	github.com/OI4/oec/schema => ../schema
	github.com/OI4/oec/spec => ../spec
)

require (
	github.com/OI4/oec/schema v0.0.0-00010101000000-000000000000
	github.com/OI4/oec/spec v0.0.0-00010101000000-000000000000
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/text v0.43.0
)
