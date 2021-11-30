package openmetrics

import (
	"agent/api/v1/model"
	"agent/pkg/parse"
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// OpenMetrics is a specification built upon Prometheus expositon format
// Spec: https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md
// PEF spec: https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md

// Currently the parser if for PEF implementation only.
// It does not enforce the format fully, but it is capable to detect a considerable amount of invalid syntax
// As well as an incomplete data.

var (
	errInvalid = errors.New("invalid syntax")
	errNoHash  = errors.New("expected # at line start")
)

// ParsePEF accepts raw data in Prometheus Exposition Format
// 'filter' is used to match only a select subset of metrics.
func ParsePEF(data []byte, filter parse.KeyMatcher) (*model.PEFResults, error) {
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("%w: data must end in newline", errInvalid)
	}
	data = bytes.TrimRight(data, "\n")
	lines := bytes.Split(data, []byte{'\n'})
	family := model.PEFFamily{Metric: make([]model.PEFMetric, 0, 4)}
	res := &model.PEFResults{Family: make([]model.PEFFamily, 0, 8), Uncategorized: make([]model.PEFMetric, 0, 8)}
	var inFamily bool
	var err error
	for i := 0; i < len(lines); i++ {
		// Ignore empty lines
		if len(lines[i]) == 0 {
			continue
		}
		// Lines starting with "#" indicate a comment, description, or type
		if lines[i][0] == '#' {
			t, err := hashLineType(lines[i])
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errInvalid, err)
			}

			// Disregard comments completely
			if t == model.Comment {
				continue
			}
			inFamily = true
			lineItems := bytes.SplitN(lines[i], []byte{' '}, 4)
			family.Name = string(lineItems[2])
			// Use other lines to fill up the metadata map
			switch t {
			case model.Help:
				family.Description = string(lineItems[3])
			case model.Type:
				family.Type = model.MetricMap[string(lineItems[3])]
			}
			continue
		} else if inFamily {
			// check if filter matches the metric
			if filter == nil || filter.Match(family.Name) {
				var j int
				family, j, err = parseFamily(family, lines[i:])
				if err != nil {
					return nil, err
				}
				res.Family = append(res.Family, family)
				i += j
			}
			family = model.PEFFamily{Metric: make([]model.PEFMetric, 0, 4)}
			inFamily = false
		} else {
			metric, err := parsePEFLine(lines[i])
			if err != nil {
				return nil, err
			}
			if filter == nil || filter.Match(metric.Name) {
				res.Uncategorized = append(res.Uncategorized, metric)
			}
		}
	}
	return res, nil
}

func parseFamily(family model.PEFFamily, lines [][]byte) (model.PEFFamily, int, error) {
	var i int
	var sumFound, countFound, bucketFound bool

	var familyNameBucket = family.Name + "_bucket"
	var familyNameCount = family.Name + "_count"
	var familyNameSum = family.Name + "_sum"

	for ; i < len(lines); i++ {
		// Get metric name and labels, if any.
		if bytes.HasPrefix(lines[i], []byte(family.Name)) {
			metric, err := parsePEFLine(lines[i])
			if err != nil {
				return model.PEFFamily{}, 0, err
			}
			if family.Type == model.Histogram {
				if familyNameBucket == metric.Name {
					bucketFound = true
				} else if familyNameCount == metric.Name {
					countFound = true
				} else if familyNameSum == metric.Name {
					sumFound = true
				} else {
					return model.PEFFamily{}, 0, fmt.Errorf("%w: in group %s unexpected metric name for histogram: '%s'", errInvalid, family.Name, metric.Name)
				}
			}
			if family.Type == model.Summary {
				if familyNameCount == metric.Name {
					countFound = true
				} else if familyNameSum == metric.Name {
					sumFound = true
				} else if family.Name != metric.Name {
					return model.PEFFamily{}, 0, fmt.Errorf("%w: in group %s unexpected metric name for summary: '%s'", errInvalid, family.Name, metric.Name)
				}
			}
			family.Metric = append(family.Metric, metric)
		} else if len(lines[i]) == 0 {
			continue
		} else if isComment(lines[i]) {
			continue
		} else {
			break
		}
	}
	if family.Type == model.Histogram && (!sumFound || !countFound || !bucketFound) {
		return model.PEFFamily{}, 0, fmt.Errorf("%w: missing required histogram fields for %s", errInvalid, family.Name)
	}
	if family.Type == model.Summary && (!sumFound || !countFound) {
		return model.PEFFamily{}, 0, fmt.Errorf("%w: missing required summary fields for %s", errInvalid, family.Name)
	}
	if i != 0 {
		i--
	}
	return family, i, nil
}

func parsePEFLine(line []byte) (model.PEFMetric, error) {
	metric := model.PEFMetric{}
	var err error
	if labelStart := bytes.IndexByte(line, '{'); labelStart > 0 {
		metric.Name = string(line[:labelStart])

		cutoff := line[labelStart+1:]
		labelEnd := bytes.IndexByte(cutoff, '}')
		if labelEnd < 0 {
			return model.PEFMetric{}, fmt.Errorf("%w: line '%s', found '{', but no '}'", errInvalid, line)
		}
		metric.Labels, err = parseLabels(cutoff[:labelEnd])
		if err != nil {
			return model.PEFMetric{}, fmt.Errorf("%w: failed parsing labels for '%s': %v", errInvalid, line, err)
		}
		line = cutoff[labelEnd+1:]
	} else {
		lineSplit := bytes.SplitN(line, []byte{' '}, 2)
		if len(lineSplit) != 2 {
			return model.PEFMetric{}, fmt.Errorf("%w: line '%s' expected a ' ', but did not find one", errInvalid, line)
		}
		metric.Name = string(lineSplit[0])

		line = lineSplit[1]
	}
	// assign value
	line = bytes.TrimSpace(line)
	values := bytes.Split(line, []byte{' '})
	metric.Value, err = strconv.ParseFloat(string(values[0]), 64)
	if err != nil {
		return model.PEFMetric{}, fmt.Errorf("%w: metric %s, failed to parse value '%s' into float", errInvalid, metric.Name, values[0])
	}
	if len(values) == 2 {
		metric.Timestamp, err = strconv.ParseInt(string(values[1]), 10, 64)
		if err != nil {
			return model.PEFMetric{}, fmt.Errorf("%w, metric %s failed to convert '%s' into int/timestamp", errInvalid, metric.Name, values[1])
		}
	}
	return metric, nil
}

func parseLabels(data []byte) ([]model.PEFLabel, error) {
	if len(data) == 0 {
		return nil, nil
	}
	items := bytes.Split(data, []byte{'"'})
	// since label data should end in a quote, remove last element
	if len(items[len(items)-1]) != 0 {
		return nil, errors.New("expected '\"' before closing bracket '}' of labels")
	}
	items = items[:len(items)-1]
	var inside bool
	var labels = make([]model.PEFLabel, 0, 2)
	var label model.PEFLabel
	for i := 0; i < len(items); i++ {

		if !inside {
			if len(items[i]) == 0 {
				continue
			}
			label = model.PEFLabel{}
			items[i] = bytes.TrimLeft(items[i], ",")
			if items[i][len(items[i])-1] != '=' {
				return nil, errors.New("expected '=' to precede '\"'")
			}
			label.Key = string(items[i][:len(items[i])-1])
			inside = true
		} else {
			label.Value += string(items[i])
			if len(items[i]) > 0 && items[i][len(items[i])-1] == '\\' {
				continue
			}
			labels = append(labels, label)
			inside = false
		}
	}
	return labels, nil
}

func hashLineType(data []byte) (model.HashLineType, error) {
	if len(data) == 0 || data[0] != '#' {
		return 0, errNoHash
	}

	if bytes.HasPrefix(data, []byte("# HELP")) {
		if c := bytes.Count(data, []byte{' '}); c < 3 {
			return 0, fmt.Errorf("expected 3 or more whitespaces in HELP line, got: %d", c)
		}
		return model.Help, nil
	}

	if bytes.HasPrefix(data, []byte("# TYPE")) {
		if c := bytes.Count(data, []byte{' '}); c != 3 {
			return 0, fmt.Errorf("expected exactly 3 whitespaces in TYPE line, got: %d", c)
		}
		return model.Type, nil
	}

	return model.Comment, nil
}

func isComment(data []byte) bool {
	t, _ := hashLineType(data)
	return t == model.Comment
}
