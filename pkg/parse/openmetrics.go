package parse

import (
	"agent/pkg/models"
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// OpenMetrics is a specification built upon Prometheus expositon format
// Spec: https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md
// PEF spec: https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md

// This is an parser of the said implementation

var (
	errInvalid = errors.New("invalid syntax")
	errNoHash  = errors.New("expected # at line start")
)

func ParsePEF(data []byte, filter *models.PEFFilter) (*models.PEFResults, error) {
	if data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("%w: data must end in newline", errInvalid)
	}
	data = bytes.TrimRight(data, "\n")
	lines := bytes.Split(data, []byte{'\n'})
	family := &models.PEFFamily{}
	res := &models.PEFResults{}
	var inFamily bool
	var err error
	for i := 0; i < len(lines); i++ {
		// Ignore empty lines
		if len(lines[i]) == 0 {
			continue
		}
		// Lines starting with "#" indicate a comment, description, or type
		if bytes.HasPrefix(lines[i], []byte{'#'}) {
			t, err := hashLineType(lines[i])
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errInvalid, err)
			}

			// Disregard comments completely
			if t == models.Comment {
				continue
			}
			inFamily = true
			lineItems := bytes.SplitN(lines[i], []byte{' '}, 4)
			family.Name = string(lineItems[2])
			// Use other lines to fill up the metadata map
			if t == models.Help {
				family.Description = string(lineItems[3])
			} else if t == models.Type {
				family.Type = models.MetricMap[string(lineItems[3])]
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
			family = &models.PEFFamily{}
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

func parseFamily(family *models.PEFFamily, lines [][]byte) (*models.PEFFamily, int, error) {
	var i int
	var sumFound, countFound, bucketFound bool
	for ; i < len(lines); i++ {
		// Get metric name and labels, if any.
		if bytes.HasPrefix(lines[i], []byte(family.Name)) {
			metric, err := parsePEFLine(lines[i])
			if err != nil {
				return nil, 0, err
			}
			if family.Type == models.Histogram {
				if family.Name+"_bucket" == metric.Name {
					bucketFound = true
				} else if family.Name+"_count" == metric.Name {
					countFound = true
				} else if family.Name+"_sum" == metric.Name {
					sumFound = true
				} else {
					return nil, 0, fmt.Errorf("%w: in group %s unexpected metric name for histogram: '%s'", errInvalid, family.Name, metric.Name)
				}
			}
			if family.Type == models.Summary {
				if family.Name+"_count" == metric.Name {
					countFound = true
				} else if family.Name+"_sum" == metric.Name {
					sumFound = true
				} else if family.Name != metric.Name {
					return nil, 0, fmt.Errorf("%w: in group %s unexpected metric name for summary: '%s'", errInvalid, family.Name, metric.Name)
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
	if family.Type == models.Histogram && (!sumFound || !countFound || !bucketFound) {
		return nil, 0, fmt.Errorf("%w: missing required histogram fields for %s", errInvalid, family.Name)
	}
	if family.Type == models.Summary && (!sumFound || !countFound) {
		return nil, 0, fmt.Errorf("%w: missing required summary fields for %s", errInvalid, family.Name)
	}
	if i != 0 {
		i--
	}
	return family, i, nil
}

func parsePEFLine(line []byte) (*models.PEFMetric, error) {
	metric := &models.PEFMetric{}
	var err error
	if labelStart := bytes.IndexByte(line, '{'); labelStart > 0 {
		metric.Name = string(line[:labelStart])

		cutoff := line[labelStart+1:]
		labelEnd := bytes.IndexByte(cutoff, '}')
		if labelEnd < 0 {
			return nil, fmt.Errorf("%w: line '%s', found '{', but no '}'", errInvalid, line)
		}
		metric.Labels, err = parseLabels(cutoff[:labelEnd])
		if err != nil {
			return nil, fmt.Errorf("%w: failed parsing labels for '%s': %v", errInvalid, line, err)
		}
		line = cutoff[labelEnd+1:]
	} else {
		lineSplit := bytes.SplitN(line, []byte{' '}, 2)
		if len(lineSplit) != 2 {
			return nil, fmt.Errorf("%w: line '%s' expected a ' ', but did not find one", errInvalid, line)
		}
		metric.Name = string(lineSplit[0])

		line = lineSplit[1]
	}
	// assign value
	line = bytes.TrimSpace(line)
	values := bytes.Split(line, []byte{' '})
	metric.Value, err = strconv.ParseFloat(string(values[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("%w: metric %s, failed to parse value '%s' into float", errInvalid, metric.Name, values[0])
	}
	if len(values) == 2 {
		metric.Timestamp, err = strconv.ParseInt(string(values[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w, metric %s failed to convert '%s' into int/timestamp", errInvalid, metric.Name, values[1])
		}
	}
	return metric, nil
}

func parseLabels(data []byte) ([]models.PEFLabel, error) {
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
	var labels []models.PEFLabel
	var label models.PEFLabel
	for i := 0; i < len(items); i++ {

		if !inside {
			if len(items[i]) == 0 {
				continue
			}
			label = models.PEFLabel{}
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

func hashLineType(data []byte) (models.HashLineType, error) {
	if len(data) == 0 || data[0] != '#' {
		return 0, errNoHash
	}

	if bytes.HasPrefix(data, []byte("# HELP")) {
		if c := bytes.Count(data, []byte{' '}); c < 3 {
			return 0, fmt.Errorf("expected 3 or more whitespaces in HELP line, got: %d", c)
		}
		return models.Help, nil
	}

	if bytes.HasPrefix(data, []byte("# TYPE")) {
		if c := bytes.Count(data, []byte{' '}); c != 3 {
			return 0, fmt.Errorf("expected exactly 3 whitespaces in TYPE line, got: %d", c)
		}
		return models.Type, nil
	}

	return models.Comment, nil
}

func isComment(data []byte) bool {
	t, _ := hashLineType(data)
	return t == models.Comment
}
