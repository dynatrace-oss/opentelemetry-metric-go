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

package dynatrace

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/lastvalue"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/sum"
	"go.opentelemetry.io/otel/sdk/metric/export"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metrictest"
	"go.opentelemetry.io/otel/sdk/metric/number"
	"go.opentelemetry.io/otel/sdk/metric/processor/processortest"
	"go.opentelemetry.io/otel/sdk/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
)

var (
	// Timestamps used in this test:

	intervalStart = time.Now()
	intervalEnd   = intervalStart.Add(time.Hour)
)

func TestNewExporter(t *testing.T) {
	t.Run("construct with URL", func(t *testing.T) {
		got, err := NewExporter(Options{URL: "https://example.com"})
		if err != nil {
			t.Error("Should not return error")
		}
		if got.opts.URL != "https://example.com" {
			t.Errorf("Expected %#v but got %#v", "https://example.com", got.opts.URL)
		}
	})

	t.Run("create a logger when missing", func(t *testing.T) {
		got, err := NewExporter(Options{})
		if err != nil {
			t.Error("Should not return error")
		}
		if got.logger == nil {
			t.Error("Exporter missing logger")
		}
	})

	t.Run("create a metric name formatter when missing", func(t *testing.T) {
		got, err := NewExporter(Options{})
		if err != nil {
			t.Error("Should not return error")
		}
		if got.opts.MetricNameFormatter == nil {
			t.Error("Exporter missing metric name formatter")
		}
	})

	t.Run("use default url when missing", func(t *testing.T) {
		got, err := NewExporter(Options{})
		if err != nil {
			t.Error("Should not return error")
		}
		if got.opts.URL != apiconstants.GetDefaultOneAgentEndpoint() {
			t.Errorf("Expected %#v but got %#v", apiconstants.GetDefaultOneAgentEndpoint(), got.opts.URL)
		}
	})
}

func Test_defaultFormatter(t *testing.T) {
	t.Run("returns name", func(t *testing.T) {
		if got := defaultFormatter("namespace", "name"); got != "name" {
			t.Errorf("defaultFormatter() = %#v, want %#v", got, "name")
		}
	})
}

func TestExporter_send(t *testing.T) {
	t.Run("authenticates requests", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if auth := req.Header.Get("Authorization"); auth != "Api-Token token" {
				t.Errorf("Expected auth header %s to equal %s", auth, "Api-Token token")
			}
			rw.Write([]byte(`OK`))
		}))
		e := &Exporter{
			opts:   Options{URL: server.URL, APIToken: "token"},
			client: server.Client(),
			logger: zap.L(),
		}

		e.send("body text")
	})

	t.Run("posts requests", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if req.Method != "POST" {
				t.Errorf("Expected method %s to be POST", req.Method)
			}
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Error("Failed to read body")
			}

			if string(body) != "body text" {
				t.Errorf("Expected body %#v to equal %s", string(body), "body text")
			}
			rw.Write([]byte(`OK`))
		}))
		e := &Exporter{
			opts:   Options{URL: server.URL, APIToken: "token"},
			client: server.Client(),
			logger: zap.L(),
		}

		e.send("body text")
	})
}

func TestExporter_TemporalityFor(t *testing.T) {
	e := &Exporter{}
	if temporality := e.TemporalityFor(&sdkapi.Descriptor{}, aggregation.HistogramKind); temporality != aggregation.DeltaTemporality {
		t.Errorf("Should return delta temporality for histogram - got %v", temporality.String())
	}

	counterDescriptor := sdkapi.NewDescriptor("", sdkapi.CounterInstrumentKind, number.Float64Kind, "", unit.Bytes)
	if temporality := e.TemporalityFor(&counterDescriptor, aggregation.SumKind); temporality != aggregation.DeltaTemporality {
		t.Errorf("Should return delta temporality for monotonic counter - got %v", temporality.String())
	}

	asyncCounterDescriptor := sdkapi.NewDescriptor("", sdkapi.CounterObserverInstrumentKind, number.Float64Kind, "", unit.Bytes)
	if temporality := e.TemporalityFor(&asyncCounterDescriptor, aggregation.SumKind); temporality != aggregation.DeltaTemporality {
		t.Errorf("Should return delta temporality for monotonic counter observer - got %v", temporality.String())
	}

	upDownCounterDescriptor := sdkapi.NewDescriptor("", sdkapi.UpDownCounterInstrumentKind, number.Float64Kind, "", unit.Bytes)
	if temporality := e.TemporalityFor(&upDownCounterDescriptor, aggregation.SumKind); temporality != aggregation.CumulativeTemporality {
		t.Errorf("Should return delta temporality for monotonic UpDownCounter - got %v", temporality.String())
	}

	asyncUpDownCounterDescriptor := sdkapi.NewDescriptor("", sdkapi.UpDownCounterInstrumentKind, number.Float64Kind, "", unit.Bytes)
	if temporality := e.TemporalityFor(&asyncUpDownCounterDescriptor, aggregation.SumKind); temporality != aggregation.CumulativeTemporality {
		t.Errorf("Should return delta temporality for monotonic UpDownCounter observer - got %v", temporality.String())
	}
}

func TestExporter_Export_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Error("should not be called")
		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Authorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Api-Token token" {
			t.Fatal("did not send authorization header")
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.CounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, attribute.EmptySet(), ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Counter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		expect := "name count,delta=11"
		if string(body) != expect {
			t.Errorf("Expected body %#v to equal %#v", string(body), expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.CounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, attribute.EmptySet(), ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_UpDownCounter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		expect := "name gauge,18"
		if string(body) != expect {
			t.Errorf("Expected body %#v to equal %#v", string(body), expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.UpDownCounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(30), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(-12), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, attribute.EmptySet(), ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Gauge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		expect := `^name gauge,-12 \d+$`
		if !regexp.MustCompile(expect).MatchString(string(body)) {
			t.Errorf("Expected body %#v to match regex %#v", string(body), expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.GaugeObserverInstrumentKind, number.Float64Kind)
	aggs := lastvalue.New(2)
	agg, ckpt := &aggs[0], &aggs[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(30), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(-12), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, attribute.EmptySet(), ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Histogram(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		expect := "name gauge,min=2,max=8,sum=21,count=4"
		if expect != string(body) {
			t.Errorf("Expected body %#v to equal %#v", string(body), expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.GaugeObserverInstrumentKind, number.Float64Kind)
	aggs := histogram.New(2, &desc, histogram.WithExplicitBoundaries([]float64{2.0, 4.0, 8.0}))
	agg, ckpt := &aggs[0], &aggs[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(3), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(7), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, attribute.EmptySet(), ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Prefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		expect := "someprefix.name count,delta=11"
		if string(body) != expect {
			t.Errorf("Expected body %#v to equal %#v", string(body), expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token", Prefix: "someprefix"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.CounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, attribute.EmptySet(), ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Counter_DefaultDims(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		descriptor := strings.Split(string(body), " ")[0]
		expect := "name,from=default"
		if descriptor != expect {
			t.Errorf("Expected metric descriptor %#v to equal %#v", descriptor, expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:              Options{URL: server.URL, APIToken: "token"},
		client:            server.Client(),
		logger:            zap.L(),
		defaultDimensions: dimensions.NewNormalizedDimensionList(dimensions.NewDimension("from", "default")),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.CounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, attribute.EmptySet(), ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Counter_MetricDims(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		descriptor := strings.Split(string(body), " ")[0]
		expect := "name,from=metric"
		if descriptor != expect {
			t.Errorf("Expected metric descriptor %#v to equal %#v", descriptor, expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.CounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	attrs := attribute.NewSet(attribute.KeyValue{Key: "from", Value: attribute.StringValue("metric")})
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, &attrs, ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_Counter_StaticDims(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		descriptor := strings.Split(string(body), " ")[0]
		expect := "name,from=static"
		if descriptor != expect {
			t.Errorf("Expected metric descriptor %#v to equal %#v", descriptor, expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),

		staticDimensions: dimensions.NewNormalizedDimensionList(dimensions.NewDimension("from", "static")),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.CounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	attrs := attribute.NewSet(attribute.KeyValue{Key: "from", Value: attribute.StringValue("metric")})
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, &attrs, ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}

func TestExporter_Export_NonStringDims(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error("Failed to read body")
		}

		descriptor := strings.Split(string(body), " ")[0]
		expect := "name"
		if descriptor != expect {
			t.Errorf("Expected metric descriptor %#v to equal %#v", descriptor, expect)
		}

		fmt.Fprintln(rw, "")
	}))
	defer server.Close()
	e := &Exporter{
		opts:   Options{URL: server.URL, APIToken: "token"},
		client: server.Client(),
		logger: zap.L(),
	}

	desc := metrictest.NewDescriptor("name", sdkapi.CounterInstrumentKind, number.Float64Kind)
	sums := sum.New(2)
	attrs := attribute.NewSet(attribute.KeyValue{Key: "int_dim", Value: attribute.Int64Value(10)})
	agg, ckpt := &sums[0], &sums[1]

	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(1), &desc))
	require.NoError(t, agg.Update(context.Background(), number.NewFloat64Number(10), &desc))

	require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

	reader := processortest.MultiInstrumentationLibraryReader(map[instrumentation.Library][]export.Record{
		{
			Name: "mylib",
		}: {export.NewRecord(&desc, &attrs, ckpt.Aggregation(), intervalStart, intervalEnd)},
	})

	e.Export(context.Background(), resource.Empty(), reader)
}
