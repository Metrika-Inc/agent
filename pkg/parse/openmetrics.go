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

func ParseOpenMetrics(data []byte, filter *models.PEFFilter) ([]models.PEFMetric, error) {
	data = bytes.TrimRight(data, "\n")
	var metadata = make(map[string]models.PEFMetric)
	lines := bytes.Split(data, []byte{'\n'})
	metrics := []models.PEFMetric{}
	var metric models.PEFMetric
	var lastFamilyName string
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

			lineItems := bytes.SplitN(lines[i], []byte{' '}, 4)
			lastFamilyName = string(lineItems[2])
			m := metadata[lastFamilyName]
			// Use other lines to fill up the metadata map
			if t == models.Help {
				m.FamilyName = lastFamilyName
				m.Description = string(lineItems[3])
			} else if t == models.Type {
				m.FamilyName = lastFamilyName
				m.MetricType = models.MetricMap[string(lineItems[3])]
			}
			metadata[lastFamilyName] = m
			continue
		}

		var err error
		// Lines not starting with "#" are metric entries
		// Find if there's any metadata for it
		if bytes.HasPrefix(lines[i], []byte(lastFamilyName)) {
			metric = metadata[lastFamilyName]
		} else {
			metric = models.PEFMetric{}
		}
		// Get metric name and labels, if any.
		if labelStart := bytes.IndexByte(lines[i], '{'); labelStart > 0 {
			metric.Name = string(lines[i][:labelStart])
			if filter != nil && !filter.Match(metric.Name) {
				continue
			}
			cutoff := lines[i][labelStart+1:]
			labelEnd := bytes.IndexByte(cutoff, '}')
			if labelEnd < 0 {
				return nil, fmt.Errorf("%w: line: %d, col: %d, found '{', but no '}'", errInvalid, i+1, labelStart)
			}
			metric.Labels, err = parseLabels(cutoff[:labelEnd])
			if err != nil {
				return nil, fmt.Errorf("%w: line: %d %v", errInvalid, i+1, err)
			}
			lines[i] = cutoff[labelEnd+1:]
		} else {
			lineSplit := bytes.SplitN(lines[i], []byte{' '}, 2)
			if len(lineSplit) != 2 {
				return nil, fmt.Errorf("%w: line: %d, expected a ' ', but did not find one", errInvalid, i+1)
			}
			metric.Name = string(lineSplit[0])
			if filter != nil && !filter.Match(metric.Name) {
				continue
			}
			lines[i] = lineSplit[1]
		}

		// assign value
		lines[i] = bytes.TrimSpace(lines[i])
		values := bytes.Split(lines[i], []byte{' '})
		metric.Value, err = strconv.ParseFloat(string(values[0]), 64)
		if err != nil {
			return nil, fmt.Errorf("%w: line: %d, failed to parse value '%s' into float", errInvalid, i+1, values[0])
		}
		if len(values) == 2 {
			metric.Timestamp, err = strconv.ParseInt(string(values[1]), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w, line: %d, failed to convert timestamp", errInvalid, i)
			}
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
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
	var labels = make([]models.PEFLabel, 0)
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
