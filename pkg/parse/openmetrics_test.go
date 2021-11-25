package parse

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOpenMetrics(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/openmetrics_valid.txt")
	require.NoError(t, err, "failed to read test file")
	// ParseOpenMetrics(data, &models.PEFFilter{ToMatch: []string{"http_request_duration_seconds_bucket"}})
	ParsePEF(data, nil)
}

func BenchmarkParseOpenMetrics(t *testing.B) {
	data, err := ioutil.ReadFile("testdata/openmetrics_valid.txt")
	require.NoError(t, err, "failed to read test file")
	// ParseOpenMetrics(data, &models.PEFFilter{ToMatch: []string{"http_request_duration_seconds_bucket"}})

	for i := 0; i < t.N; i++ {
		_, err := ParsePEF(data, nil)
		require.NoError(t, err)
	}
}
