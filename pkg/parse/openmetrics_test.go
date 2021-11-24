package parse

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseOpenMetrics(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/openmetrics_valid.txt")
	require.NoError(t, err, "failed to read test file")
	results, err := ParseOpenMetrics(data, nil)
	_ = results
	_ = err
	time.Sleep(1)
	for _, res := range results {
		fmt.Println(res.Labels)
	}
}
