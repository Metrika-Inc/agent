package openmetrics

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/require"
)

const (
	ValidTests = "testdata/happy/"
	FailTests  = "testdata/failure/"
)

func readFileOrFail(t *testing.T, filepath string) []byte {
	res, err := ioutil.ReadFile(filepath)
	require.NoError(t, err, "failed to read file")
	return res
}

func BenchmarkPefDecoder(bech *testing.B) {
	data, err := ioutil.ReadFile(ValidTests + "pef_full")
	require.NoError(bech, err, "failed to read test file")

	for i := 0; i < bech.N; i++ {
		buf := bytes.NewBuffer(data)
		dec := expfmt.NewDecoder(buf, expfmt.FmtText)
		for {
			mf := dto.MetricFamily{}
			if err := dec.Decode(&mf); err != nil && err == io.EOF {
				break
			} else if err != nil {
				bech.Fatalf("received error while decoding PEF: %v", err)
			}
		}
	}
}

func BenchmarkExpfmtText(bech *testing.B) {
	data, err := ioutil.ReadFile(ValidTests + "pef_full")
	require.NoError(bech, err, "failed to read test file")
	var parser expfmt.TextParser
	for i := 0; i < bech.N; i++ {
		buf := bytes.NewBuffer(data)
		parser.TextToMetricFamilies(buf)
	}
}

func TestPEFDecoder(t *testing.T) {
	b := readFileOrFail(t, ValidTests+"pef_full")

	buf := bytes.NewBuffer(b)
	dec := expfmt.NewDecoder(buf, expfmt.FmtText)
	for {
		mf := dto.MetricFamily{}
		if err := dec.Decode(&mf); err != nil && err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("received error while decoding PEF: %v", err)
		}
	}
}
func TestParsePEF_Failures(t *testing.T) {
	cases := []struct {
		Description string
		Data        []byte
	}{
		{"Missing label closure",
			[]byte("http_requests_total{method=\"post\",code=\"200\" 1027 1395066363000\n")},
		{"Missing label value closure",
			[]byte("http_requests_total{method=\"post\",code=\"200} 1027 1395066363000\n")},
		{"Unescaped label sequence", []byte("http_requests_total{method=\"p\"ost\",code=\"200\"} 1027 1395066363000\n")},

		{"Invalid value type", []byte("metric_without_timestamp_and_labels asdf\n")},
		{"Invalid timestamp type", []byte("http_requests_total 1027 139506.12\n")},
		{"Invalid type line", []byte("# TYPE metric_name type1 and some extra info\n")},
	}

	for _, testcase := range cases {
		buf := bytes.NewBuffer(testcase.Data)
		_, err := ParsePEF(buf, nil)
		require.Error(t, err, "Expected error when running TestParsePEF['%s']", testcase.Description)
	}
}

func TestParsePEF_Happy(t *testing.T) {
	b := readFileOrFail(t, ValidTests+"pef_full")

	t.Run("TestParsePEF no filters", func(t *testing.T) {
		buf := bytes.NewBuffer(b)
		mf, err := ParsePEF(buf, nil)
		require.NoError(t, err)
		require.Len(t, mf, 6)
	})
	t.Run("TestParsePEF filters", func(t *testing.T) {
		testcases := []struct {
			filter  []string
			matches int
		}{
			{[]string{"http_request_duration_seconds", "something_weird"}, 2},
			{[]string{}, 0},
			{[]string{"non-existant-metric"}, 0},
			{[]string{"non-existant-metric", "http_requests_total"}, 1},
		}

		for _, tc := range testcases {
			filter := &PEFFilter{ToMatch: tc.filter}
			buf := bytes.NewBuffer(b)
			mf, err := ParsePEF(buf, filter)
			require.NoError(t, err)
			require.Len(t, mf, tc.matches)
		}
	})
}
