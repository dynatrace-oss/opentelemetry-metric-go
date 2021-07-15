module github.com/dynatrace-oss/opentelemetry-metric-go/example/basic

go 1.15

require (
	github.com/dynatrace-oss/opentelemetry-metric-go v0.1.0-beta
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/sdk v0.13.0
	go.uber.org/zap v1.16.0
)

replace github.com/dynatrace-oss/opentelemetry-metric-go => ../../
