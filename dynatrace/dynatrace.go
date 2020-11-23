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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/dynatrace-oss/opentelemetry-metric-go/mint"

	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/label"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.uber.org/zap"
)

const (
	// DefaultDynatraceURL defines the default local metrics endpoint
	DefaultDynatraceURL = "http://127.0.0.1:14499/metrics/ingest"
)

// NewExporter exports to a Dynatrace MINT API
func NewExporter(opts Options) (*Exporter, error) {
	if opts.URL == "" {
		opts.URL = DefaultDynatraceURL
	}
	if opts.MetricNameFormatter == nil {
		opts.MetricNameFormatter = defaultFormatter
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}

	client := &http.Client{}

	return &Exporter{
		client: client,
		opts:   opts,
		logger: opts.Logger,
	}, nil
}

// Options contains options for configuring the exporter.
type Options struct {
	URL      string
	APIToken string
	Prefix   string
	Tags     []string
	Logger   *zap.Logger

	MetricNameFormatter func(namespace, name string) string
}

// Exporter forwards metrics to a Dynatrace agent
type Exporter struct {
	opts   Options
	client *http.Client
	logger *zap.Logger
}

var (
	reNameAllowedCharList = regexp.MustCompile("[^A-Za-z0-9.-]+")
	maxDimKeyLen          = 100
	maxMetricKeyLen       = 250
)

func defaultFormatter(namespace, name string) string {
	return name
}

// ExportKindFor returns export.DeltaExporter for statsd-derived exporters
func (e *Exporter) ExportKindFor(*metric.Descriptor, aggregation.Kind) export.ExportKind {
	return export.DeltaExporter
}

// Export given CheckpointSet
func (e *Exporter) Export(ctx context.Context, cs export.CheckpointSet) error {
	// TODO tags
	// TODO normalize
	lines := []string{}

	err := cs.ForEach(e, func(r export.Record) error {
		agg := r.Aggregation()

		itr := label.NewMergeIterator(r.Labels(), r.Resource().LabelSet())
		dimensions := []mint.Dimension{}
		for itr.Next() {
			label := itr.Label()
			dim := mint.NewDimension(string(label.Key), label.Value.Emit())
			dimensions = append(dimensions, dim)
		}

		descriptor := mint.SerializeDescriptor(r.Descriptor().Name(), e.opts.Prefix, dimensions, e.opts.Tags)
		if descriptor == "" {
			e.logger.Warn(fmt.Sprintf("failed to normalize metric name: %s", r.Descriptor().Name()))
			return nil
		}

		switch agg := agg.(type) {
		case aggregation.MinMaxSumCount:
			min, err := agg.Min()
			if err != nil {
				return fmt.Errorf("error getting min for %s: %w", descriptor, err)
			}

			max, err := agg.Max()
			if err != nil {
				return fmt.Errorf("error getting max for %s: %w", descriptor, err)
			}

			sum, err := agg.Sum()
			if err != nil {
				return fmt.Errorf("error getting sum for %s: %w", descriptor, err)
			}

			count, err := agg.Count()
			if err != nil {
				return fmt.Errorf("error getting count for %s: %w", descriptor, err)
			}

			switch r.Descriptor().NumberKind() {
			case metric.Float64NumberKind:
				lines = append(lines, mint.SerializeRecord(descriptor, mint.SerializeDoubleSummaryValue(min.AsFloat64(), max.AsFloat64(), sum.AsFloat64(), count)))
			case metric.Int64NumberKind:
				lines = append(lines, mint.SerializeRecord(descriptor, mint.SerializeIntSummaryValue(min.AsInt64(), max.AsInt64(), sum.AsInt64(), count)))
			}

		case aggregation.Sum:
			val, err := agg.Sum()
			if err != nil {
				return fmt.Errorf("error getting LastValue for %s: %w", descriptor, err)
			}

			switch r.Descriptor().NumberKind() {
			case metric.Float64NumberKind:
				lines = append(lines, mint.SerializeRecord(descriptor, mint.SerializeDoubleCountValue(val.AsFloat64())))
			case metric.Int64NumberKind:
				lines = append(lines, mint.SerializeRecord(descriptor, mint.SerializeIntCountValue(val.AsInt64())))
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error generating metric lines: %s", err.Error())
	}
	output := strings.Join(lines, "\n")
	if output != "" {
		err = e.send(output)
		if err != nil {
			return fmt.Errorf("error processing data:, %s", err.Error())
		}
	}
	return nil
}

func (e *Exporter) send(message string) error {
	e.logger.Debug("Sending lines to Dynatrace\n" + message)
	req, err := http.NewRequest("POST", e.opts.URL, bytes.NewBufferString(message))
	if err != nil {
		return fmt.Errorf("dynatrace error while creating HTTP request: %s", err.Error())
	}

	req.Header.Add("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Add("Authorization", "Api-Token "+e.opts.APIToken)
	req.Header.Add("User-Agent", "opentelemetry-collector")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %s", err.Error())
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error while receiving HTTP response: %s", err.Error())
	}

	responseBody := metricsResponse{}
	if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
		e.logger.Error(fmt.Sprintf("failed to unmarshal response: %s", err.Error()))
	} else {
		e.logger.Debug(fmt.Sprintf("Exported %d lines to Dynatrace", responseBody.Ok))

		if responseBody.Invalid > 0 {
			e.logger.Debug(fmt.Sprintf("Failed to export %d lines to Dynatrace", responseBody.Invalid))
		}

		if responseBody.Error != "" {
			e.logger.Error(fmt.Sprintf("Error from Dynatrace: %s", responseBody.Error))
		}
	}

	if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted) {
		return fmt.Errorf("request failed with response code:, %d", resp.StatusCode)
	}

	return nil
}

// Close the exporter
func (e *Exporter) Close() error {
	e.client = nil
	return nil
}

// Response from Dynatrace is expected to be in JSON format
type metricsResponse struct {
	Ok      int64  `json:"linesOk"`
	Invalid int64  `json:"linesInvalid"`
	Error   string `json:"error"`
}
