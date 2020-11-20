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
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/label"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
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

	client := &http.Client{}

	return &Exporter{
		client: client,
		opts:   opts,
	}, nil
}

// Options contains options for configuring the exporter.
type Options struct {
	URL      string
	APIToken string
	Prefix   string
	Tags     []string

	MetricNameFormatter func(namespace, name string) string
}

// Exporter forwards metrics to a Dynatrace agent
type Exporter struct {
	opts   Options
	client *http.Client
}

var (
	_                     export.Exporter = &Exporter{}
	reNameAllowedCharList                 = regexp.MustCompile("[^A-Za-z0-9.-]+")
	maxDimKeyLen                          = 100
	maxMetricKeyLen                       = 250
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
	output := ""
	valueline := ""

	err := cs.ForEach(e, func(r export.Record) error {
		agg := r.Aggregation()

		name, _ := e.normalizeMetricName(r.Descriptor().InstrumentationName(), r.Descriptor().Name())

		itr := label.NewMergeIterator(r.Labels(), r.Resource().LabelSet())
		tags := append([]string{}, e.opts.Tags...)
		for itr.Next() {
			labels := itr.Label()
			dimName, _ := e.normalizeDimensionName(r.Descriptor().InstrumentationName(), string(labels.Key))
			tag := dimName + "=" + labels.Value.Emit()
			tags = append(tags, tag)
		}
		var tagline string

		for _, tag := range tags {
			if tagline != "" {
				tagline += ","
			}
			tagline += tag
		}
		fmt.Println(agg.Kind().String())

		switch agg := agg.(type) {
		case aggregation.MinMaxSumCount:
			minVal, err := agg.Min()
			if err != nil {
				return fmt.Errorf("error getting min for %s: %w", name, err)
			}
			minValue := strconv.FormatFloat(normalizeMetricValue(r.Descriptor().NumberKind(), minVal), 'f', 6, 64)

			maxVal, err := agg.Max()
			if err != nil {
				return fmt.Errorf("error getting max for %s: %w", name, err)
			}
			maxValue := strconv.FormatFloat(normalizeMetricValue(r.Descriptor().NumberKind(), maxVal), 'f', 6, 64)

			sumVal, err := agg.Sum()
			if err != nil {
				return fmt.Errorf("error getting sum for %s: %w", name, err)
			}
			sumValue := strconv.FormatFloat(normalizeMetricValue(r.Descriptor().NumberKind(), sumVal), 'f', 6, 64)

			countVal, err := agg.Count()
			if err != nil {
				return fmt.Errorf("error getting count for %s: %w", name, err)
			}

			countValue := strconv.FormatInt(countVal, 10)
			valueline = "gauge,min=" + minValue + ",max=" + maxValue + ",sum=" + sumValue + ",count=" + countValue

		case aggregation.Sum:
			val, err := agg.Sum()
			if err != nil {
				return fmt.Errorf("error getting LastValue for %s: %w", name, err)
			}

			value := strconv.FormatFloat(normalizeMetricValue(r.Descriptor().NumberKind(), val), 'f', 6, 64)

			// fmt.Println(r.Descriptor().MetricKind())
			switch r.Descriptor().MetricKind() {
			case metric.CounterKind, metric.SumObserverKind:
				valueline = "count,delta=" + value
			case metric.UpDownCounterKind, metric.UpDownSumObserverKind:
				valueline = value
			}
		}
		if tagline != "" {
			name = name + ","
		}

		if output != "" {
			output = output + "\n"
		}
		output += name + tagline + " " + valueline

		return nil
	})
	if err != nil {
		return fmt.Errorf("error generating metric lines: %s", err.Error())
	}
	if output != "" {
		fmt.Println(output)
		err = e.send(output)
		if err != nil {
			return fmt.Errorf("error processing data:, %s", err.Error())
		}
	}
	return nil
}

func (e *Exporter) send(message string) error {

	req, err := http.NewRequest("POST", e.opts.URL, bytes.NewBufferString(message))
	if err != nil {
		fmt.Printf("Dynatrace error: %s \n", err.Error())
		return fmt.Errorf("dynatrace error while creating HTTP request:, %s", err.Error())
	}

	req.Header.Add("Content-Type", "text/plain; charset=UTF-8")

	if len(e.opts.APIToken) != 0 {
		req.Header.Add("Authorization", "Api-Token "+e.opts.APIToken)
	}
	// add user-agent header to identify metric source
	req.Header.Add("User-Agent", "opentelemetry-go")

	resp, err := e.client.Do(req)
	if err != nil {
		fmt.Printf("Dynatrace error: %s \n", err.Error())
		return fmt.Errorf("dynatrace error while sending HTTP request:, %s", err.Error())
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Dynatrace error reading response")
	}
	bodyString := string(bodyBytes)

	fmt.Printf("Dynatrace returned: %s \n", bodyString)

	if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted) {
		return fmt.Errorf("dynatrace request failed with response code:, %d", resp.StatusCode)
	}

	return nil

}

// Close the exporter
func (e *Exporter) Close() error {
	e.client = nil
	return nil
}

// normalizeMetricName formats the custom namespace and view name to
// Metric naming Conventions
func (e *Exporter) normalizeMetricName(namespace, name string) (string, error) {
	// Append Prefix if there is one
	if e.opts.Prefix != "" {
		name = e.opts.Prefix + "." + name
	}
	return normalizeString(e.opts.MetricNameFormatter(namespace, name), maxMetricKeyLen)
}

func (e *Exporter) normalizeDimensionName(namespace, name string) (string, error) {
	return normalizeString(strings.ToLower(e.opts.MetricNameFormatter(namespace, name)), maxDimKeyLen)
}

// normalizeString replaces all non-alphanumerical characters to underscore
func normalizeString(str string, max int) (string, error) {
	str = reNameAllowedCharList.ReplaceAllString(str, "_")

	// Strip Digits and underscores if they are at the beginning of the string
	normalizedString := strings.TrimLeft(str, "_0123456789")

	for strings.HasPrefix(normalizedString, "_") {
		normalizedString = normalizedString[1:]
	}

	if len(normalizedString) > max {
		normalizedString = normalizedString[:max]
	}

	for strings.HasSuffix(normalizedString, "_") {
		normalizedString = normalizedString[:len(normalizedString)-1]
	}

	if len(normalizedString) == 0 {
		return "", fmt.Errorf("error normalizing the string: %s", str)
	}
	return normalizedString, nil
}

func normalizeMetricValue(kind metric.NumberKind, number metric.Number) float64 {
	switch kind {
	case metric.Float64NumberKind:
		return number.AsFloat64()
	case metric.Int64NumberKind:
		return float64(number.AsInt64())
	}
	return float64(number)
}
