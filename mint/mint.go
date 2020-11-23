package mint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Dimension struct {
	key   string
	value string
}

// NewDimension constructs a Dimension from a key-value pair
func NewDimension(key, value string) Dimension {
	return Dimension{
		key:   key,
		value: value,
	}
}

func (d *Dimension) toString() string {
	return fmt.Sprintf("%s=\"%s\"", d.key, d.value)
}

// SerializeDescriptor serializes a descriptor in MINT format
func SerializeDescriptor(name, prefix string, dimensions []Dimension, tags []string) string {
	prefixedName := normalizeMetricName(joinPrefix(name, prefix))

	if len(dimensions) > 0 {
		if len(tags) > 0 {
			return fmt.Sprintf("%s,%s,%s", prefixedName, strings.Join(tags, ","), serializeDimensions(dimensions))
		}
		return fmt.Sprintf("%s,%s", prefixedName, serializeDimensions(dimensions))
	}

	return prefixedName
}

func normalizeMetricName(name string) string {
	if len(name) > 250 {
		name = name[0:250]
	}

	sections := strings.Split(name, ".")
	normalizedSections := []string{}
	for i, section := range sections {
		normalized := ""
		if i == 0 {
			normalized = normalizeFirstMetricNameSection(section)
			if normalized == "" {
				return ""
			}
		} else {
			normalized = normalizeMetricNameSection(section)
		}
		if normalized != "" {
			normalizedSections = append(normalizedSections, normalized)
		}
	}

	return strings.Join(normalizedSections, ".")
}

var (
	reTrimMetricNameStart                   = regexp.MustCompile("[a-zA-Z].*")
	reMetricNameSectionAllowedStartCharList = regexp.MustCompile("^[^a-zA-Z0-9]+")
	reMetricNameSectionAllowedCharList      = regexp.MustCompile("[^a-zA-Z0-9_-]+")
)

func normalizeFirstMetricNameSection(section string) string {
	return normalizeMetricNameSection(reTrimMetricNameStart.FindString(section))
}

func normalizeMetricNameSection(section string) string {
	return reMetricNameSectionAllowedCharList.ReplaceAllString(reMetricNameSectionAllowedStartCharList.ReplaceAllString(section, ""), "_")
}

func serializeDimensions(dimensions []Dimension) string {
	tags := []string{}
	for _, dim := range dimensions {
		tags = append(tags, dim.toString())
	}
	return strings.Join(tags, ",")
}

func joinPrefix(name, prefix string) string {
	if prefix != "" {
		return fmt.Sprintf("%s.%s", prefix, name)
	}
	return name
}

// SerializeRecord returns a string suitable for MINT ingest
func SerializeRecord(descriptor, valueLine string) string {
	return fmt.Sprintf("%s %s", descriptor, valueLine)
}

// SerializeIntSummaryValue returns a MINT gauge value line
func SerializeIntSummaryValue(min, max, sum, count int64) string {
	return fmt.Sprintf("gauge,min=%d,max=%d,sum=%d,count=%d", min, max, sum, count)
}

// SerializeDoubleSummaryValue returns a MINT gauge value line
func SerializeDoubleSummaryValue(min, max, sum float64, count int64) string {
	return fmt.Sprintf("gauge,min=%s,max=%s,sum=%s,count=%d", serializeFloat64(min), serializeFloat64(max), serializeFloat64(sum), count)
}

// SerializeIntCountValue returns a MINT count value line
func SerializeIntCountValue(value int64) string {
	return fmt.Sprintf("count,%d", value)
}

// SerializeDoubleCountValue returns a MINT count value line
func SerializeDoubleCountValue(value float64) string {
	return fmt.Sprintf("count,%s", serializeFloat64(value))
}

func serializeFloat64(n float64) string {
	str := strings.TrimRight(strconv.FormatFloat(n, 'f', 6, 64), "0.")
	if str == "" {
		// if everything was trimmed away, number was 0.000000
		return "0"
	}
	return str
}
