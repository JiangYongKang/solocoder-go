package metrics

import (
	"fmt"
	"math"
	"strings"
)

func (r *registry) PrometheusFormat() []byte {
	snapshot := r.Snapshot()
	var builder strings.Builder

	grouped := make(map[string][]MetricValue)
	for _, mv := range snapshot {
		grouped[mv.Name] = append(grouped[mv.Name], mv)
	}

	for name, values := range grouped {
		if len(values) == 0 {
			continue
		}
		metricType := values[0].Type

		switch metricType {
		case CounterType:
			builder.WriteString(fmt.Sprintf("# HELP %s_total \n", name))
			builder.WriteString(fmt.Sprintf("# TYPE %s_total counter\n", name))
			for _, v := range values {
				labels := formatLabels(v.Labels)
				builder.WriteString(fmt.Sprintf("%s_total%s %v\n", name, labels, v.Value))
			}

		case GaugeType:
			builder.WriteString(fmt.Sprintf("# HELP %s \n", name))
			builder.WriteString(fmt.Sprintf("# TYPE %s gauge\n", name))
			for _, v := range values {
				labels := formatLabels(v.Labels)
				builder.WriteString(fmt.Sprintf("%s%s %v\n", name, labels, v.Value))
			}

		case HistogramType:
			builder.WriteString(fmt.Sprintf("# HELP %s \n", name))
			builder.WriteString(fmt.Sprintf("# TYPE %s histogram\n", name))
			for _, v := range values {
				for _, bucket := range v.Buckets {
					labels := mergeLabels(v.Labels, Label{Name: "le", Value: formatBucketBound(bucket.UpperBound)})
					builder.WriteString(fmt.Sprintf("%s_bucket%s %d\n", name, formatLabels(labels), bucket.Count))
				}
				labels := formatLabels(v.Labels)
				builder.WriteString(fmt.Sprintf("%s_sum%s %v\n", name, labels, v.Sum))
				builder.WriteString(fmt.Sprintf("%s_count%s %d\n", name, labels, v.Count))
			}

		case SummaryType:
			builder.WriteString(fmt.Sprintf("# HELP %s \n", name))
			builder.WriteString(fmt.Sprintf("# TYPE %s summary\n", name))
			for _, v := range values {
				for _, q := range v.Quantiles {
					labels := mergeLabels(v.Labels, Label{Name: "quantile", Value: fmt.Sprintf("%g", q.Quantile)})
					builder.WriteString(fmt.Sprintf("%s%s %v\n", name, formatLabels(labels), q.Value))
				}
				labels := formatLabels(v.Labels)
				builder.WriteString(fmt.Sprintf("%s_sum%s %v\n", name, labels, v.Sum))
				builder.WriteString(fmt.Sprintf("%s_count%s %d\n", name, labels, v.Count))
			}
		}
	}

	return []byte(builder.String())
}

func formatBucketBound(bound float64) string {
	if math.IsInf(bound, 1) {
		return "+Inf"
	}
	return fmt.Sprintf("%g", bound)
}

func mergeLabels(labels Labels, extra Label) Labels {
	result := make(Labels, len(labels), len(labels)+1)
	copy(result, labels)
	result = append(result, extra)
	return result
}

func PrometheusFormat() []byte {
	return DefaultRegistry.PrometheusFormat()
}

func Snapshot() []MetricValue {
	return DefaultRegistry.Snapshot()
}

func RegisterCounter(name string, labels Labels) CounterMetric {
	return DefaultRegistry.RegisterCounter(name, labels)
}

func RegisterGauge(name string, labels Labels) GaugeMetric {
	return DefaultRegistry.RegisterGauge(name, labels)
}

func RegisterHistogram(name string, labels Labels, buckets []float64) HistogramMetric {
	return DefaultRegistry.RegisterHistogram(name, labels, buckets)
}

func RegisterSummary(name string, labels Labels, quantiles []float64) SummaryMetric {
	return DefaultRegistry.RegisterSummary(name, labels, quantiles)
}

func GetCounter(name string, labels Labels) (CounterMetric, bool) {
	return DefaultRegistry.GetCounter(name, labels)
}

func GetGauge(name string, labels Labels) (GaugeMetric, bool) {
	return DefaultRegistry.GetGauge(name, labels)
}

func GetHistogram(name string, labels Labels) (HistogramMetric, bool) {
	return DefaultRegistry.GetHistogram(name, labels)
}

func GetSummary(name string, labels Labels) (SummaryMetric, bool) {
	return DefaultRegistry.GetSummary(name, labels)
}

func Unregister(name string, labels Labels) bool {
	return DefaultRegistry.Unregister(name, labels)
}
