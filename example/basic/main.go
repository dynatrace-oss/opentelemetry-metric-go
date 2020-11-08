package main

import (
	"context"
	"os"
	// "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"

	"github.com/dynatrace-oss/opentelemetry-metric-go/dynatrace"
)

func getEnv(name, def string) string {
	if value, exists := os.LookupEnv(name); exists {
		return value
	}
	return def
}

func main() {

	opts := dynatrace.Options{}
	if token, exists := os.LookupEnv("API_TOKEN"); exists {
		opts.APIToken = token
		opts.URL = os.Getenv("ENDPOINT")
	}

	exporter, err := dynatrace.NewExporter(opts)
	if err != nil{
		panic(err)
	}

	defer exporter.Close()

	processor := basic.New(
		simple.NewWithExactDistribution(),
		exporter,
	)

	pusher := push.New(
		processor,
		exporter,
	)

	pusher.Start()
	defer pusher.Stop()
	
	global.SetMeterProvider(pusher.MeterProvider())
	meter := global.Meter("ex.com/basic")
	vr := metric.Must(meter).NewFloat64ValueRecorder("ex.com.two")
	vr.Record(context.Background(), 1.0)
}