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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/label"
	metricsdk "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/export/metric/metrictest"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/sum"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
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
	type args struct {
		namespace string
		name      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"returns name",
			args{"namespace", "name"},
			"name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultFormatter(tt.args.namespace, tt.args.name); got != tt.want {
				t.Errorf("defaultFormatter() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExporter_ExportKindFor(t *testing.T) {
	type fields struct {
		opts              Options
		defaultDimensions dimensions.NormalizedDimensionList
		staticDimensions  dimensions.NormalizedDimensionList
		client            *http.Client
		logger            *zap.Logger
	}
	type args struct {
		in0 *metric.Descriptor
		in1 aggregation.Kind
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   metricsdk.ExportKind
	}{
		{
			"returns delta",
			fields{},
			args{&metric.Descriptor{}, aggregation.ExactKind},
			metricsdk.DeltaExporter,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Exporter{
				opts:              tt.fields.opts,
				defaultDimensions: tt.fields.defaultDimensions,
				staticDimensions:  tt.fields.staticDimensions,
				client:            tt.fields.client,
				logger:            tt.fields.logger,
			}
			if got := e.ExportKindFor(tt.args.in0, tt.args.in1); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Exporter.ExportKindFor() = %#v, want %#v", got, tt.want)
			}
		})
	}
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

type checkpointSet struct {
	sync.RWMutex
	records []metricsdk.Record
}

func (m *checkpointSet) ForEach(_ metricsdk.ExportKindSelector, fn func(metricsdk.Record) error) error {
	for _, r := range m.records {
		if err := fn(r); err != nil && err != aggregation.ErrNoData {
			return err
		}
	}
	return nil
}

func TestExporter_Export(t *testing.T) {
	t.Run("should not export empty chunks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			panic("This should not be called")
		}))
		defer server.Close()
		e := &Exporter{
			opts:   Options{URL: server.URL, APIToken: "token"},
			client: server.Client(),
			logger: zap.L(),
		}

		require.NoError(t, e.Export(context.Background(), &checkpointSet{records: []metricsdk.Record(nil)}))
	})

	t.Run("should export a metric", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Error("Failed to read body")
			}

			expect := "normalized_int-64-counter,normalized_key=normalized\\ value count,delta=1"
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

		recs := []metricsdk.Record{}
		desc := metric.NewDescriptor("normalized int-64-counter", metric.CounterKind, metric.Int64NumberKind)
		agg, ckpt := metrictest.Unslice2(sum.New(2))

		ctx := context.Background()
		agg.Update(ctx, metric.NewInt64Number(1), &desc)

		require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

		labs := label.NewSet(label.String("normalized key", "normalized value"))
		res := resource.New()
		intervalStart := time.Now()
		intervalEnd := intervalStart.Add(time.Hour)
		rec := metricsdk.NewRecord(&desc, &labs, res, ckpt.Aggregation(), intervalStart, intervalEnd)
		recs = append(recs, rec)
		require.NoError(t, e.Export(context.Background(), &checkpointSet{records: recs}))
	})

	t.Run("should export a metric with prefix", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Error("Failed to read body")
			}

			expect := "prefixed.normalized_int-64-counter,normalized_key=normalized\\ value count,delta=1"
			if string(body) != expect {
				t.Errorf("Expected body %#v to equal %#v", string(body), expect)
			}

			fmt.Fprintln(rw, "")
		}))
		defer server.Close()
		e := &Exporter{
			opts:   Options{URL: server.URL, APIToken: "token", Prefix: "prefixed"},
			client: server.Client(),
			logger: zap.L(),
		}

		recs := []metricsdk.Record{}
		desc := metric.NewDescriptor("normalized int-64-counter", metric.CounterKind, metric.Int64NumberKind)
		agg, ckpt := metrictest.Unslice2(sum.New(2))

		ctx := context.Background()
		agg.Update(ctx, metric.NewInt64Number(1), &desc)

		require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

		labs := label.NewSet(label.String("normalized key", "normalized value"))
		res := resource.New()
		intervalStart := time.Now()
		intervalEnd := intervalStart.Add(time.Hour)
		rec := metricsdk.NewRecord(&desc, &labs, res, ckpt.Aggregation(), intervalStart, intervalEnd)
		recs = append(recs, rec)
		require.NoError(t, e.Export(context.Background(), &checkpointSet{records: recs}))
	})

	t.Run("should prefer static labels (from OneAgent)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Error("Failed to read body")
			}

			expect := "int-64-counter,from=static count,delta=1"
			if string(body) != expect {
				t.Errorf("Expected body %#v to equal %#v", string(body), expect)
			}

			fmt.Fprintln(rw, "")
		}))
		defer server.Close()
		e := &Exporter{
			opts:              Options{URL: server.URL, APIToken: "token"},
			client:            server.Client(),
			logger:            zap.L(),
			staticDimensions:  dimensions.NewNormalizedDimensionList(dimensions.NewDimension("from", "static")),
			defaultDimensions: dimensions.NewNormalizedDimensionList(dimensions.NewDimension("from", "default")),
		}

		recs := []metricsdk.Record{}
		desc := metric.NewDescriptor("int-64-counter", metric.CounterKind, metric.Int64NumberKind)
		agg, ckpt := metrictest.Unslice2(sum.New(2))

		ctx := context.Background()
		agg.Update(ctx, metric.NewInt64Number(1), &desc)

		require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

		labs := label.NewSet(label.String("from", "metric"))
		res := resource.New()
		intervalStart := time.Now()
		intervalEnd := intervalStart.Add(time.Hour)
		rec := metricsdk.NewRecord(&desc, &labs, res, ckpt.Aggregation(), intervalStart, intervalEnd)
		recs = append(recs, rec)
		require.NoError(t, e.Export(context.Background(), &checkpointSet{records: recs}))
	})

	t.Run("should prefer metric labels over default", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Error("Failed to read body")
			}

			expect := "int-64-counter,from=metric count,delta=1"
			if string(body) != expect {
				t.Errorf("Expected body %#v to equal %#v", string(body), expect)
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

		recs := []metricsdk.Record{}
		desc := metric.NewDescriptor("int-64-counter", metric.CounterKind, metric.Int64NumberKind)
		agg, ckpt := metrictest.Unslice2(sum.New(2))

		ctx := context.Background()
		agg.Update(ctx, metric.NewInt64Number(1), &desc)

		require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

		labs := label.NewSet(label.String("from", "metric"))
		res := resource.New()
		intervalStart := time.Now()
		intervalEnd := intervalStart.Add(time.Hour)
		rec := metricsdk.NewRecord(&desc, &labs, res, ckpt.Aggregation(), intervalStart, intervalEnd)
		recs = append(recs, rec)
		require.NoError(t, e.Export(context.Background(), &checkpointSet{records: recs}))
	})

	t.Run("should use metric labels if no static or default", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Error("Failed to read body")
			}

			expect := "int-64-counter,from=metric count,delta=1"
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

		recs := []metricsdk.Record{}
		desc := metric.NewDescriptor("int-64-counter", metric.CounterKind, metric.Int64NumberKind)
		agg, ckpt := metrictest.Unslice2(sum.New(2))

		ctx := context.Background()
		agg.Update(ctx, metric.NewInt64Number(1), &desc)

		require.NoError(t, agg.SynchronizedMove(ckpt, &desc))

		labs := label.NewSet(label.String("from", "metric"))
		res := resource.New()
		intervalStart := time.Now()
		intervalEnd := intervalStart.Add(time.Hour)
		rec := metricsdk.NewRecord(&desc, &labs, res, ckpt.Aggregation(), intervalStart, intervalEnd)
		recs = append(recs, rec)
		require.NoError(t, e.Export(context.Background(), &checkpointSet{records: recs}))
	})
}
