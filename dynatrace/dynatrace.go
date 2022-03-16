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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/export"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
)

// NewExporter creates an exporter for the Dynatrace Metrics v2 API
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

	staticDimensions := dimensions.NewNormalizedDimensionList(dimensions.NewDimension("dt.metrics.source", "opentelemetry"))

	if !opts.DisableDynatraceMetadataEnrichment {
		staticDimensions = dimensions.MergeLists(staticDimensions, oneagentenrichment.GetOneAgentMetadata())
	}

	defaultDimensions := dimensions.NewNormalizedDimensionList(opts.DefaultDimensions...)

	return &Exporter{
		client:            client,
		opts:              opts,
		defaultDimensions: defaultDimensions,
		staticDimensions:  staticDimensions,
		logger:            opts.Logger,
	}, nil
}

// Options contains options for configuring the exporter.
type Options struct {
	URL                                string
	APIToken                           string
	Prefix                             string
	DefaultDimensions                  []dimensions.Dimension
	Logger                             *zap.Logger
	DisableDynatraceMetadataEnrichment bool

	MetricNameFormatter func(namespace, name string) string
}

// Create a new dimension for use in the DefaultDimensions option
func NewDimension(key, value string) dimensions.Dimension {
	return dimensions.NewDimension(key, value)
}

// Exporter forwards metrics to a Dynatrace agent
type Exporter struct {
	opts              Options
	defaultDimensions dimensions.NormalizedDimensionList
	staticDimensions  dimensions.NormalizedDimensionList
	client            *http.Client
	logger            *zap.Logger
}

func defaultFormatter(namespace, name string) string {
	return name
}

// TemporalityFor returns delta for histograms and monotonic counters, else cumulative
func (e *Exporter) TemporalityFor(desc *sdkapi.Descriptor, kind aggregation.Kind) aggregation.Temporality {
	if kind == aggregation.HistogramKind {
		return aggregation.DeltaTemporality
	}

	if desc.InstrumentKind().Adding() && desc.InstrumentKind().Monotonic() {
		return aggregation.DeltaTemporality
	}

	return aggregation.CumulativeTemporality
}

// Export given CheckpointSet
func (e *Exporter) Export(ctx context.Context, res *resource.Resource, reader export.InstrumentationLibraryReader) error {
	lines := []string{}

	_ = reader.ForEach(func(l instrumentation.Library, reader export.Reader) error {
		return reader.ForEach(e, func(record export.Record) error {
			dims := []dimensions.Dimension{}
			iter := attribute.NewMergeIterator(record.Labels(), res.Set())

			for iter.Next() {
				label := iter.Label()
				dims = append(dims, NewDimension(string(label.Key), label.Value.AsString()))
			}

			agg := record.Aggregation()

			dtDimensions := dimensions.MergeLists(
				e.defaultDimensions,
				dimensions.NewNormalizedDimensionList(dims...),
				e.staticDimensions,
			)

			if hist, ok := agg.(aggregation.Histogram); ok {
				summary, err := summaryFromHistogram(hist, record.Descriptor().NumberKind())

				if err != nil {
					e.logger.Sugar().Errorw("error converting histogram to dt summary",
						"name", record.Descriptor().Name(),
						"error", err)
					return nil
				}

				if summary != nil {
					metric, err := dtMetric.NewMetric(
						record.Descriptor().Name(),
						dtMetric.WithPrefix(e.opts.Prefix),
						dtMetric.WithDimensions(dtDimensions),
						summary,
					)

					if err != nil {
						e.logger.Sugar().Errorw("error creating summary metric from histogram summary",
							"name", record.Descriptor().Name(),
							"error", err)
					}

					line, err := metric.Serialize()
					if err != nil {
						e.logger.Sugar().Errorw("error serializing histogram summary metric",
							"name", record.Descriptor().Name(),
							"error", err)
					}
					if line != "" {
						lines = append(lines, line)
					}
				}
			} else if sum, ok := agg.(aggregation.Sum); ok {
				valOpt, err := valueOptForSum(sum, record.Descriptor().InstrumentKind().Monotonic(), record.Descriptor().NumberKind())

				if err != nil {
					e.logger.Sugar().Errorw("error creating dtMetric option for sum",
						"name", record.Descriptor().Name(),
						"error", err)
					return nil
				}

				metric, err := dtMetric.NewMetric(
					record.Descriptor().Name(),
					dtMetric.WithPrefix(e.opts.Prefix),
					dtMetric.WithDimensions(dtDimensions),
					valOpt,
				)

				if err != nil {
					e.logger.Sugar().Errorw("error creating count metric from sum",
						"name", record.Descriptor().Name(),
						"error", err)
				}

				line, err := metric.Serialize()
				if err != nil {
					e.logger.Sugar().Errorw("error serializing count metric",
						"name", record.Descriptor().Name(),
						"error", err)
				}
				if line != "" {
					lines = append(lines, line)
				}
			} else if agg, ok := agg.(aggregation.LastValue); ok {
				lastValue, ts, err := agg.LastValue()
				if err != nil {
					e.logger.Sugar().Errorw("error converting sum to dt counter",
						"name", record.Descriptor().Name(),
						"error", err)
					return nil
				}

				metric, err := dtMetric.NewMetric(
					record.Descriptor().Name(),
					dtMetric.WithPrefix(e.opts.Prefix),
					dtMetric.WithDimensions(dtDimensions),
					dtMetric.WithFloatGaugeValue(lastValue.CoerceToFloat64(record.Descriptor().NumberKind())),
					dtMetric.WithTimestamp(ts),
				)

				if err != nil {
					e.logger.Sugar().Errorw("error creating gauge metric from last value",
						"name", record.Descriptor().Name(),
						"error", err)
				}

				line, err := metric.Serialize()
				if err != nil {
					e.logger.Sugar().Errorw("error serializing gauge metric",
						"name", record.Descriptor().Name(),
						"error", err)
				}
				if line != "" {
					lines = append(lines, line)
				}
			} else {
				e.logger.Sugar().Errorf("Unsupported aggregation",
					"aggregator", agg.Kind().String())
			}
			return nil
		})
	})

	limit := apiconstants.GetPayloadLinesLimit()
	for i := 0; i < len(lines); i += limit {
		batch := lines[i:min(i+limit, len(lines))]

		output := strings.Join(batch, "\n")
		if output != "" {
			err := e.send(output)
			if err != nil {
				return fmt.Errorf("error processing data:, %s", err.Error())
			}
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
