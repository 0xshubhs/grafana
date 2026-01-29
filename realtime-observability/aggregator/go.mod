module github.com/yourorg/aggregator

go 1.22

require (
	github.com/gorilla/websocket v1.5.1
	github.com/prometheus/client_golang v1.19.0
	github.com/yourorg/telemetry/gen v0.0.0
	google.golang.org/grpc v1.64.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	github.com/prometheus/common v0.50.0 // indirect
	github.com/prometheus/procfs v0.13.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

replace github.com/yourorg/telemetry/gen => ../gen
