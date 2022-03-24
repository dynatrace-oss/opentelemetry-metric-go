// Copyright 2020 Dynatrace LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.uber.org/zap"

	"github.com/dynatrace-oss/opentelemetry-metric-go/dynatrace"
)

func mustEnv(name string) string {
	if val, exists := os.LookupEnv(name); exists {
		return val
	}

	panic(fmt.Sprintf("Missing required environment variable: %s", name))
}

func main() {
	logger, err := zap.NewDevelopment()

	if err != nil {
		log.Fatalf("Failed to start %v", err)
	}

	opts := dynatrace.Options{
		Logger: logger, // optional
	}
	// If no endpoint is provided, metrics will be exported to the local OneAgent endpoint
	if endpoint, exists := os.LookupEnv("ENDPOINT"); exists {
		opts.URL = endpoint
		// API token is only required if an endpoint is provided
		opts.APIToken = mustEnv("API_TOKEN")
	} else {
		logger.Sugar().Infow("Using local OneAgent API")
	}

	exporter, err := dynatrace.NewExporter(opts)
	if err != nil {
		panic(err)
	}

	defer exporter.Close()

	c := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries([]float64{1.0, 2.0, 4.0, 8.0}),
			),
			exporter,
			processor.WithMemory(true),
		),
		controller.WithExporter(exporter),
		controller.WithCollectPeriod(time.Second*10),
	)

	// global.SetMeterProvider(c)
	_ = c.Start(context.Background())

	meter := c.Meter("otel.dynatrace.com/basic")
	vr := metric.Must(meter).NewFloat64Histogram("golang_histogram")

	counter := metric.Must(meter).NewInt64Counter("golang_counter")
	_ = metric.Must(meter).NewFloat64GaugeObserver("golang_gauge", func(ctx context.Context, result metric.Float64ObserverResult) {
		result.Observe(rand.Float64() * 100)
	})

	for {
		counter.Add(context.Background(), int64(rand.Intn(20)))
		vr.Record(context.Background(), rand.Float64()*10)
		vr.Record(context.Background(), rand.Float64()*10)
		vr.Record(context.Background(), rand.Float64()*10)
		time.Sleep(time.Second * 5)
	}
}
