package models

type PromMetric struct {
	FamilyName  string
	Name        string
	Description string
	UnitType    string
	MetricType  MetricType
	Labels      []PromLabel
	Value       float64
	Timestamp   int64
}

type PromLabel struct {
	Key, Value string
}

type HashLineType int

const (
	Comment HashLineType = iota
	Help
	Type
)

type MetricType int

const (
	Unknown MetricType = iota
	Gauge
	Counter
	Histogram
	Summary
)

var MetricMap = map[string]MetricType{
	"counter":   Counter,
	"gauge":     Gauge,
	"histogram": Histogram,
	"summary":   Summary,
}

type PromFilter struct {
	Metrics []string
}

func (p *PromFilter) Match(metricName string) bool {
	for _, metric := range p.Metrics {
		if metric == metricName {
			return true
		}
	}
	return false
}
