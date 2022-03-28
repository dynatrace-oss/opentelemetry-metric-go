module github.com/dynatrace-oss/opentelemetry-metric-go/example/basic

go 1.15

require (
	github.com/dynatrace-oss/opentelemetry-metric-go v0.1.0-beta
	go.opentelemetry.io/otel v1.5.0 // indirect
	go.opentelemetry.io/otel/metric v0.27.0
	go.opentelemetry.io/otel/sdk/metric v0.27.0
	go.uber.org/zap v1.21.0
)

replace github.com/dynatrace-oss/opentelemetry-metric-go => ../../
