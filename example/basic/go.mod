module github.com/dynatrace-oss/opentelemetry-metric-go/example/basic

go 1.17

require (
	github.com/dynatrace-oss/opentelemetry-metric-go v0.2.0-beta
	go.opentelemetry.io/otel/metric v0.31.0
	go.opentelemetry.io/otel/sdk/metric v0.31.0
	go.uber.org/zap v1.22.0
)

require (
	github.com/dynatrace-oss/dynatrace-metric-utils-go v0.5.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/otel v1.9.0 // indirect
	go.opentelemetry.io/otel/sdk v1.9.0 // indirect
	go.opentelemetry.io/otel/sdk/export/metric v0.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.9.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/sys v0.0.0-20220808155132-1c4a2a72c664 // indirect
)

replace github.com/dynatrace-oss/opentelemetry-metric-go => ../../
