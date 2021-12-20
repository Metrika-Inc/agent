package openmetrics

import (
	"io/ioutil"
	"math"
	"testing"

	"agent/api/v1/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ValidTests = "testdata/happy/"
	FailTests  = "testdata/failure/"
)

func TestParsePEF_HappyCase(t *testing.T) {
	expected := &model.PEFResults{
		Family: []model.PEFFamily{
			{
				Name:        "http_requests_total",
				Description: "The total number of HTTP requests.",
				Type:        model.Counter,
				Metric: []model.PEFMetric{
					{
						Name: "http_requests_total",
						Labels: []model.PEFLabel{
							{Key: "method", Value: "post"}, {Key: "code", Value: "200"},
						},
						Value:     1027,
						Timestamp: 1395066363000,
					},
					{
						Name: "http_requests_total",
						Labels: []model.PEFLabel{
							{Key: "method", Value: "post"}, {Key: "code", Value: "400"},
						},
						Value:     3,
						Timestamp: 1395066363000,
					},
				},
			},
			{
				Name:        "http_request_duration_seconds",
				Description: "A histogram of the request duration.",
				Type:        model.Histogram,
				Metric: []model.PEFMetric{
					{
						Name: "http_request_duration_seconds_bucket",
						Labels: []model.PEFLabel{
							{Key: "le", Value: "0.05"},
						},
						Value: 24054,
					},
					{
						Name: "http_request_duration_seconds_bucket",
						Labels: []model.PEFLabel{
							{Key: "le", Value: "0.1"},
						},
						Value: 33444,
					},
					{
						Name: "http_request_duration_seconds_bucket",
						Labels: []model.PEFLabel{
							{Key: "le", Value: "0.2"},
						},
						Value: 100392,
					},
					{
						Name: "http_request_duration_seconds_bucket",
						Labels: []model.PEFLabel{
							{Key: "le", Value: "0.5"},
						},
						Value: 129389,
					},
					{
						Name: "http_request_duration_seconds_bucket",
						Labels: []model.PEFLabel{
							{Key: "le", Value: "1"},
						},
						Value: 133988,
					},
					{
						Name: "http_request_duration_seconds_bucket",
						Labels: []model.PEFLabel{
							{Key: "le", Value: "+Inf"},
						},
						Value: 144320,
					},
					{
						Name:  "http_request_duration_seconds_sum",
						Value: 53423,
					},
					{
						Name:  "http_request_duration_seconds_count",
						Value: 144320,
					},
				},
			},
			{
				Name:        "rpc_duration_seconds",
				Description: "A summary of the RPC duration in seconds.",
				Metric: []model.PEFMetric{
					{
						Name: "rpc_duration_seconds",
						Labels: []model.PEFLabel{
							{Key: "quantile", Value: "0.01"},
						},
						Value: 3102,
					},
					{
						Name: "rpc_duration_seconds",
						Labels: []model.PEFLabel{
							{Key: "quantile", Value: "0.05"},
						},
						Value: 3272,
					},
					{
						Name: "rpc_duration_seconds",
						Labels: []model.PEFLabel{
							{Key: "quantile", Value: "0.5"},
						},
						Value: 4773,
					},
					{
						Name: "rpc_duration_seconds",
						Labels: []model.PEFLabel{
							{Key: "quantile", Value: "0.9"},
						},
						Value: 9001,
					},
					{
						Name: "rpc_duration_seconds",
						Labels: []model.PEFLabel{
							{Key: "quantile", Value: "0.99"},
						},
						Value: 76656,
					},
					{
						Name:  "rpc_duration_seconds_sum",
						Value: 1.7560473e+07,
					},
					{
						Name:  "rpc_duration_seconds_count",
						Value: 2693,
					},
				},
			},
		},
		Uncategorized: []model.PEFMetric{
			{
				Name: "msdos_file_access_time_seconds",
				Labels: []model.PEFLabel{
					{Key: "path", Value: `C:\\DIR\\FILE.TXT`}, {Key: "error", Value: "Cannot find file:\\n\\FILE.TXT\\"},
				},
				Value: 1458255915,
			},
			{
				Name:  "metric_without_timestamp_and_labels",
				Value: 12.47,
			},
			{
				Name: "something_weird",
				Labels: []model.PEFLabel{
					{Key: "problem", Value: "division by zero"},
				},
				Value:     math.Inf(0),
				Timestamp: -3982045,
			},
		},
	}
	data, err := ioutil.ReadFile(ValidTests + "pef_full")
	require.NoError(t, err, "failed to read test file")
	result, err := ParsePEF(data, nil)
	require.NoError(t, err, "PEF parsing failed unexpectedly")
	assertParsePEF(t, expected, result)
}

func TestParsePEF_Filter(t *testing.T) {
	expected := &model.PEFResults{
		Family: []model.PEFFamily{
			{
				Name:        "http_requests_total",
				Description: "The total number of HTTP requests.",
				Type:        model.Counter,
				Metric: []model.PEFMetric{
					{
						Name: "http_requests_total",
						Labels: []model.PEFLabel{
							{Key: "method", Value: "post"}, {Key: "code", Value: "200"},
						},
						Value:     1027,
						Timestamp: 1395066363000,
					},
					{
						Name: "http_requests_total",
						Labels: []model.PEFLabel{
							{Key: "method", Value: "post"}, {Key: "code", Value: "400"},
						},
						Value:     3,
						Timestamp: 1395066363000,
					},
				},
			},
		},
		Uncategorized: []model.PEFMetric{
			{
				Name: "something_weird",
				Labels: []model.PEFLabel{
					{Key: "problem", Value: "division by zero"},
				},
				Value:     math.Inf(0),
				Timestamp: -3982045,
			},
		},
	}
	filter := &model.PEFFilter{
		ToMatch: []string{"http_requests_total", "something_weird"},
	}
	data, err := ioutil.ReadFile(ValidTests + "pef_full")
	require.NoError(t, err, "failed to read test file")
	result, err := ParsePEF(data, filter)
	require.NoError(t, err, "PEF parsing failed unexpectedly")
	assertParsePEF(t, expected, result)
}

func TestParsePEF_Empty(t *testing.T) {
	result, err := ParsePEF([]byte{'\n'}, nil)
	require.NoError(t, err, "PEF parsing failed unexpectedly")
	assert.Len(t, result.Family, 0)
	assert.Len(t, result.Uncategorized, 0)
}

func assertParsePEF(t *testing.T, expected, actual *model.PEFResults) {
	require.Len(t, actual.Family, len(expected.Family))
	require.Len(t, actual.Uncategorized, len(expected.Uncategorized))
	for i := 0; i < len(actual.Family); i++ {
		require.Len(t, actual.Family[i].Metric, len(expected.Family[i].Metric))
		require.Equal(t, expected.Family[i].Name,
			actual.Family[i].Name, "metric family name mismatch")
		require.Equal(t, expected.Family[i].Description,
			actual.Family[i].Description, "metric family description mismatch")

		for j := 0; j < len(actual.Family[i].Metric); j++ {
			require.Equal(t, expected.Family[i].Metric[j],
				actual.Family[i].Metric[j], "metric value mismatch")
		}
	}
	for i := 0; i < len(actual.Uncategorized); i++ {
		require.Equal(t, actual.Uncategorized[i], expected.Uncategorized[i],
			"metric value mismatch")
	}
}

func TestParsePEF_Failures(t *testing.T) {
	cases := []struct {
		Description string
		Data        []byte
	}{
		{"Incomplete file", readFileOrFail(t, FailTests+"pef_incomplete")},
		{"Missing histogram params", readFileOrFail(t, FailTests+"pef_histogram1")},
		{"Missing summary params", readFileOrFail(t, FailTests+"pef_summary1")},
		{"Invalid histogram params", readFileOrFail(t, FailTests+"pef_histogram2")},
		{"Invalid summary params", readFileOrFail(t, FailTests+"pef_summary2")},
		{"Missing label closure",
			[]byte("http_requests_total{method=\"post\",code=\"200\" 1027 1395066363000\n")},
		{"Missing label value closure",
			[]byte("http_requests_total{method=\"post\",code=\"200} 1027 1395066363000\n")},
		{"Unescaped label sequence", []byte("http_requests_total{method=\"p\"ost\",code=\"200\"} 1027 1395066363000\n")},

		{"Invalid value type", []byte("metric_without_timestamp_and_labels asdf\n")},
		{"Invalid timestamp type", []byte("http_requests_total 1027 139506.12\n")},
		{"Invalid help line", []byte("# HELP metric_name\n")},
		{"Invalid type line", []byte("# TYPE metric_name type1 and some extra info\n")},
	}

	for _, testcase := range cases {
		result, err := ParsePEF(testcase.Data, nil)
		assert.Nil(t, result)
		require.Error(t, err, "Expected error when running TestParsePEF['%s']",
			testcase.Description)
	}
}

func readFileOrFail(t *testing.T, filepath string) []byte {
	res, err := ioutil.ReadFile(filepath)
	require.NoError(t, err, "failed to read file")
	return res
}

func BenchmarkParsePEF(t *testing.B) {
	data, err := ioutil.ReadFile(ValidTests + "pef_full")
	require.NoError(t, err, "failed to read test file")

	for i := 0; i < t.N; i++ {
		_, err := ParsePEF(data, nil)
		require.NoError(t, err)
	}
}
