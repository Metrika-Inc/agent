package model

type PEFResults struct {
	Family        []*PEFFamily
	Uncategorized []*PEFMetric
}

// PEFFamily is a group of metrics, preceded by"# HELP" and "# TYPE" lines
type PEFFamily struct {
	Name        string
	Description string
	Type        MetricType
	Metric      []*PEFMetric
}

// PEFMetric represents a single entry from PEF data.
// Includes all relevant metadata, if PEF has it correctly specified.
type PEFMetric struct {
	Name      string
	Labels    []PEFLabel
	Value     float64
	Timestamp int64
}

// PEFLabel represent key-value pairs, specified in curly brackets of a PEF entry.
type PEFLabel struct {
	Key, Value string
}

// HashLineType is used for categorizing any lines in PEF data starting with "#"
type HashLineType int

const (
	Invalid HashLineType = iota
	Comment
	Help
	Type
	// Unit
)

// MetricType is used for recognizing the metric type.
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

// PEFFilter is used as a matcher when only a subset of PEF entries are wanted.
type PEFFilter struct {
	ToMatch []string
}

// Match checks the metricName against specified set of names
func (p *PEFFilter) Match(metricName string) bool {
	for _, metric := range p.ToMatch {
		if metric == metricName {
			return true
		}
	}
	return false
}
