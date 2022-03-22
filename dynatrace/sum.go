package dynatrace

import (
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"go.opentelemetry.io/otel/metric/number"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
)

func valueOptForSum(sum aggregation.Sum, monotonic bool, kind number.Kind) (metric.MetricOption, error) {
	value, err := sum.Sum()
	if err != nil {
		return nil, err
	}

	if monotonic {
		return metric.WithFloatCounterValueDelta(value.CoerceToFloat64(kind)), nil
	}

	return metric.WithFloatGaugeValue(value.CoerceToFloat64(kind)), nil
}
