package metrics

import "time"

type MetricType string

const (
	CounterType   MetricType = "counter"
	GaugeType     MetricType = "gauge"
	HistogramType MetricType = "histogram"
	SummaryType   MetricType = "summary"
)

type Label struct {
	Name  string
	Value string
}

type Labels []Label

func (l Labels) Hash() string {
	if len(l) == 0 {
		return ""
	}
	var h string
	for i, label := range l {
		if i > 0 {
			h += ","
		}
		h += label.Name + "=" + label.Value
	}
	return h
}

type Metric interface {
	Name() string
	Type() MetricType
	Labels() Labels
	Snapshot() MetricValue
}

type MetricValue struct {
	Name      string
	Type      MetricType
	Labels    Labels
	Timestamp time.Time
	Value     float64
	Buckets   []BucketValue
	Quantiles []QuantileValue
	Count     uint64
	Sum       float64
}

type BucketValue struct {
	UpperBound float64
	Count      uint64
}

type QuantileValue struct {
	Quantile float64
	Value    float64
}

type CounterMetric interface {
	Metric
	Inc()
	Add(delta float64)
	Value() float64
	Reset()
}

type GaugeMetric interface {
	Metric
	Set(value float64)
	Inc()
	Dec()
	Add(delta float64)
	Sub(delta float64)
	Value() float64
}

type HistogramMetric interface {
	Metric
	Observe(value float64)
	Buckets() []BucketValue
	Count() uint64
	Sum() float64
}

type SummaryMetric interface {
	Metric
	Observe(value float64)
	Quantiles() []QuantileValue
	Count() uint64
	Sum() float64
}

type Registry interface {
	RegisterCounter(name string, labels Labels) CounterMetric
	RegisterGauge(name string, labels Labels) GaugeMetric
	RegisterHistogram(name string, labels Labels, buckets []float64) HistogramMetric
	RegisterSummary(name string, labels Labels, quantiles []float64) SummaryMetric
	GetCounter(name string, labels Labels) (CounterMetric, bool)
	GetGauge(name string, labels Labels) (GaugeMetric, bool)
	GetHistogram(name string, labels Labels) (HistogramMetric, bool)
	GetSummary(name string, labels Labels) (SummaryMetric, bool)
	Snapshot() []MetricValue
	PrometheusFormat() []byte
	Unregister(name string, labels Labels) bool
}
