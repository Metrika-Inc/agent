package parse

import (
	"agent/pkg/models"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOpenMetrics(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/openmetrics_valid.txt")
	require.NoError(t, err, "failed to read test file")
	ParseOpenMetrics(data, &models.PEFFilter{ToMatch: []string{"http_request_duration_seconds_bucket"}})
}
