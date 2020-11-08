module github.com/dynatrace-oss/opentelemetry-metric-go/example/basic

go 1.14

replace github.com/dynatrace-oss/opentelemetry-metric-go => ../..

require (
	github.com/dynatrace-oss/opentelemetry-metric-go v0.1.1
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/stdout v0.13.0
	go.opentelemetry.io/otel/sdk v0.13.0
)
