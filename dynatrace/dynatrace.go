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
	"strings"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/oneagentenrichment"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/label"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.uber.org/zap"
)

// NewExporter exports to the Dynatrace Metrics V2 API
func NewExporter(opts Options) (*Exporter, error) {
	if opts.URL == "" {
		opts.URL = apiconstants.GetDefaultOneAgentEndpoint()
	}
	if opts.MetricNameFormatter == nil {
		opts.MetricNameFormatter = defaultFormatter
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}

	client := &http.Client{}

	constantDimensions := dimensions.MergeLists(
		dimensions.NewNormalizedDimensionList(dimensions.NewDimension("dt.metrics.source", "opentelemetry")),
		oneagentenrichment.GetOneAgentMetadata(),
	)

	defaultDimensions := dimensions.NewNormalizedDimensionList(opts.DefaultDimensions...)

	return &Exporter{
		client:             client,
		opts:               opts,
		defaultDimensions:  defaultDimensions,
		constantDimensions: constantDimensions,
		logger:             opts.Logger,
	}, nil
}

// Options contains options for configuring the exporter.
type Options struct {
	URL               string
	APIToken          string
	Prefix            string
	DefaultDimensions []dimensions.Dimension
	Logger            *zap.Logger

	MetricNameFormatter func(namespace, name string) string
}

// Create a new dimension for use in the DefaultDimensions option
func NewDimension(key, value string) dimensions.Dimension {
	return dimensions.NewDimension(key, value)
}

// Exporter forwards metrics to a Dynatrace agent
type Exporter struct {
	opts               Options
	defaultDimensions  dimensions.NormalizedDimensionList
	constantDimensions dimensions.NormalizedDimensionList
	client             *http.Client
	logger             *zap.Logger
}

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
		itr := label.NewMergeIterator(r.Labels(), r.Resource().LabelSet())
		dims := []dimensions.Dimension{}
		for itr.Next() {
			label := itr.Label()
			dims = append(dims, dimensions.NewDimension(string(label.Key), label.Value.Emit()))
		}

		valOpt, err := getValueOption(r)

		if err != nil {
			e.logger.Warn(fmt.Sprintf("failed to normalize metric: %s - %s", r.Descriptor().Name(), err.Error()))
			return nil
		}

		metric, err := dtMetric.NewMetric(
			r.Descriptor().Name(),
			dtMetric.WithPrefix(e.opts.Prefix),
			dtMetric.WithDimensions(
				dimensions.MergeLists(
					e.defaultDimensions,
					dimensions.NewNormalizedDimensionList(dims...),
					e.constantDimensions,
				),
			),
			valOpt,
		)

		if err != nil {
			e.logger.Warn(fmt.Sprintf("failed to normalize metric: %s - %s", r.Descriptor().Name(), err.Error()))
			return nil
		}

		line, err := metric.Serialize()

		if err != nil {
			e.logger.Warn(fmt.Sprintf("failed to serialize metric: %s - %s", r.Descriptor().Name(), err.Error()))
			return nil
		}

		lines = append(lines, line)

		return nil
	})

	if err != nil {
		return fmt.Errorf("error generating metric lines: %s", err.Error())
	}
	limit := apiconstants.GetPayloadLinesLimit()
	for i := 0; i < len(lines); i += limit {
		batch := lines[i:min(i+limit, len(lines))]

		output := strings.Join(batch, "\n")
		if output != "" {
			err = e.send(output)
			if err != nil {
				return fmt.Errorf("error processing data:, %s", err.Error())
			}
		}
	}

	return nil
}

func getValueOption(r export.Record) (dtMetric.MetricOption, error) {
	switch agg := r.Aggregation().(type) {
	case aggregation.MinMaxSumCount:
		min, err := agg.Min()
		if err != nil {
			return nil, fmt.Errorf("error getting min for %s: %w", r.Descriptor().Name(), err)
		}

		max, err := agg.Max()
		if err != nil {
			return nil, fmt.Errorf("error getting max for %s: %w", r.Descriptor().Name(), err)
		}

		sum, err := agg.Sum()
		if err != nil {
			return nil, fmt.Errorf("error getting sum for %s: %w", r.Descriptor().Name(), err)
		}

		count, err := agg.Count()
		if err != nil {
			return nil, fmt.Errorf("error getting count for %s: %w", r.Descriptor().Name(), err)
		}

		switch r.Descriptor().NumberKind() {
		case metric.Float64NumberKind:
			return dtMetric.WithFloatSummaryValue(min.AsFloat64(), max.AsFloat64(), sum.AsFloat64(), count), nil
		case metric.Int64NumberKind:
			return dtMetric.WithIntSummaryValue(min.AsInt64(), max.AsInt64(), sum.AsInt64(), count), nil
		}

	case aggregation.Sum:
		val, err := agg.Sum()
		if err != nil {
			return nil, fmt.Errorf("error getting LastValue for %s: %w", r.Descriptor().Name(), err)
		}

		switch r.Descriptor().NumberKind() {
		case metric.Float64NumberKind:
			return dtMetric.WithFloatCounterValueDelta(val.AsFloat64()), nil
		case metric.Int64NumberKind:
			return dtMetric.WithIntCounterValueDelta(val.AsInt64()), nil
		}
	}

	return nil, fmt.Errorf("unknown aggregation for %s: %s", r.Descriptor().Name(), r.Aggregation().Kind().String())
}

func (e *Exporter) send(message string) error {
	e.logger.Debug("Sending lines to Dynatrace\n" + message)
	req, err := http.NewRequest("POST", e.opts.URL, bytes.NewBufferString(message))
	if err != nil {
		return fmt.Errorf("dynatrace error while creating HTTP request: %s", err.Error())
	}

	req.Header.Add("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Add("Authorization", "Api-Token "+e.opts.APIToken)
	req.Header.Add("User-Agent", "opentelemetry-metric-go")

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

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}
