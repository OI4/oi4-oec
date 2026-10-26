module github.com/OI4/oec/conformance

go 1.26.0

toolchain go1.27.3

replace (
	github.com/OI4/oec/schema => ../schema
	github.com/OI4/oec/spec => ../spec
)

require (
	github.com/OI4/oec/schema v0.0.0-00010101000000-000000000000
	github.com/OI4/oec/spec v0.0.0-00010101000000-000000000000
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/text v0.43.0
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
)
