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
	"os"

	// "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"

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
	if err != nil {
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
	meter := global.Meter("otel.dynatrace.com/basic")
	vr := metric.Must(meter).NewFloat64ValueRecorder("otel.dynatrace.com.golang")
	vr.Record(context.Background(), 1.0)
}
