package dynatrace

import (
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"go.opentelemetry.io/otel/metric/number"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
)

func summaryFromHistogram(hist aggregation.Histogram, kind number.Kind) (metric.MetricOption, error) {
	// export histogram
	sum, err := hist.Sum()
	if err != nil {
		return nil, err
	}

	count, err := hist.Count()
	if err != nil {
		return nil, err
	}

	buckets, err := hist.Histogram()
	if err != nil {
		return nil, err
	}

	min, max := estimateHistMinMax(buckets.Boundaries, buckets.Counts)

	return metric.WithFloatSummaryValue(min, max, sum.CoerceToFloat64(kind), int64(count)), nil
}

// estimateHistMinMax returns the estimated minimum and maximum value in the histogram by using the min and max non-empty buckets.
func estimateHistMinMax(bounds []float64, counts []uint64) (float64, float64) {
	// Because we do not know the actual min and max, we estimate them based on the min and max non-empty bucket
	minIdx, maxIdx := -1, -1
	for y := 0; y < len(counts); y++ {
		if counts[y] > 0 {
			if minIdx == -1 {
				minIdx = y
			}
			maxIdx = y
		}
	}

	if minIdx == -1 || maxIdx == -1 {
		return 0, 0
	}

	var min, max float64

	// Use lower bound for min unless it is the first bucket which has no lower bound, then use upper
	if minIdx == 0 {
		min = bounds[minIdx]
	} else {
		min = bounds[minIdx-1]
	}

	// Use upper bound for max unless it is the last bucket which has no upper bound, then use lower
	if maxIdx == len(counts)-1 {
		max = bounds[maxIdx-1]
	} else {
		max = bounds[maxIdx]
	}

	return min, max
}
