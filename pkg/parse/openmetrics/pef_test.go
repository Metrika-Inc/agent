package openmetrics

import (
	"agent/api/v1/model"
	"agent/pkg/timesync"
	"bytes"
	"io"
	"io/ioutil"
	"testing"
	"unsafe"

	"github.com/golang/protobuf/proto"
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

func TestPefDecoder(t *testing.T) {
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

func TestSizes(t *testing.T) {
	b := readFileOrFail(t, ValidTests+"pef_algorand")

	t.Run("TestParsePEF no filters", func(t *testing.T) {
		buf := bytes.NewBuffer(b)
		mf, err := ParsePEF(buf, nil)
		require.NoError(t, err)
		// require.Len(t, mf, 6)
		t.Logf("Size of Message model: %d", unsafe.Sizeof(model.Message{}))
		batch := &model.PlatformMessage{Data: make([]*model.Message, 0)}
		var totalSize, totalMarsalled uint
		for _, metricFamily := range mf {
			// 1. We marshal the metric, (bytes metric = 6;)
			// 2. We don't marshal the metric (io.prometheus.client.MetricFamily metric = 6;)
			// a) use proto.Size()
			// b) use custom implementation of Message.Size()

			// TODO:
			// Compare the outputs and performance of 2a 2b; (Done)
			// Have another discussion with Dimosthenis (Thursday)

			message := &model.Message{
				Timestamp:  timesync.Now().Unix(),
				Type:       model.MessageType_metric,
				Name:       "test",
				NodeState:  model.NodeState_down,
				AgentState: model.AgentState_healthy,
				Metric:     metricFamily,
			}
			batch.Data = append(batch.Data, message)

			protosize := proto.Size(metricFamily)
			bytessize := message.Bytes()
			t.Logf("proto.Size: %v, message.Bytes: %v", protosize, bytessize)
			totalSize += bytessize
			totalMarsalled += uint(protosize)
		}
		t.Logf("Total in memory (B): %v\nTotal marshalled (one by one): %v\nTotal marshalled (batch): %v",
			totalSize, totalMarsalled, proto.Size(batch))
		t.Logf("Marshalled + struct metadata: %v", totalMarsalled+uint(100*len(batch.Data)))
	})
}

func TestOffsets(t *testing.T) {
	m := dto.MetricFamily{}
	t.Log(unsafe.Offsetof(m.Help))
	t.Log(unsafe.Sizeof(m))
}

func BenchmarkMarshalOnce(b *testing.B) {
	out, err := ioutil.ReadFile(ValidTests + "pef_algorand")
	require.NoError(b, err)
	buf := bytes.NewBuffer(out)
	mf, err := ParsePEF(buf, nil)
	require.NoError(b, err)
	require.Len(b, mf, 54)

	batch := model.PlatformMessage{Data: make([]*model.Message, 0)}

	for i := 0; i < b.N; i++ {
		for _, m := range mf {
			batch.Data = append(batch.Data, &model.Message{
				Timestamp:  timesync.Now().Unix(),
				Type:       model.MessageType_metric,
				Name:       "test",
				NodeState:  model.NodeState_down,
				AgentState: model.AgentState_healthy,
				Metric:     m,
			})
		}
		_, _ = proto.Marshal(&batch)
		batch.Data = make([]*model.Message, 0)
	}
}

func BenchmarkMarshalTwice(b *testing.B) {
	out, err := ioutil.ReadFile(ValidTests + "pef_algorand")
	require.NoError(b, err)
	buf := bytes.NewBuffer(out)
	mf, err := ParsePEF(buf, nil)
	require.NoError(b, err)
	require.Len(b, mf, 54)

	batch := &model.PlatformMessage{Data: make([]*model.Message, 0)}

	for i := 0; i < b.N; i++ {
		for _, m := range mf {
			out, _ := proto.Marshal(m)
			batch.Data = append(batch.Data, &model.Message{
				Timestamp:  timesync.Now().Unix(),
				Type:       model.MessageType_metric,
				Name:       "test",
				NodeState:  model.NodeState_down,
				AgentState: model.AgentState_healthy,
				AltMetric:  out,
			})
		}
		_, _ = proto.Marshal(batch)

		batch.Data = make([]*model.Message, 0)
	}
}

func BenchmarkMarshalMetric(b *testing.B) {
	out, err := ioutil.ReadFile(ValidTests + "pef_algorand")
	require.NoError(b, err)
	buf := bytes.NewBuffer(out)
	mf, err := ParsePEF(buf, nil)
	require.NoError(b, err)
	require.Len(b, mf, 54)

	for i := 0; i < b.N; i++ {
		// for _, m := range mf {
		// 	_, _ = proto.Marshal(m)
		// }
			_, _ = proto.Marshal(mf[53-7])
	}
}

func BenchmarkBytes(b *testing.B) {
	out, err := ioutil.ReadFile(ValidTests + "pef_algorand")
	require.NoError(b, err)
	buf := bytes.NewBuffer(out)
	mf, err := ParsePEF(buf, nil)
	require.NoError(b, err)
	// require.Len(b, mf, 6)

	for i := 0; i < b.N; i++ {
		message := &model.Message{
			Timestamp:  timesync.Now().Unix(),
			Type:       model.MessageType_metric,
			Name:       "test",
			NodeState:  model.NodeState_down,
			AgentState: model.AgentState_healthy,
			Metric:     mf[0],
		}
		_ = message.Bytes()
	}
}

func BenchmarkBuiltInSize(b *testing.B) {
	out, err := ioutil.ReadFile(ValidTests + "pef_algorand")
	require.NoError(b, err)
	buf := bytes.NewBuffer(out)
	mf, err := ParsePEF(buf, nil)
	require.NoError(b, err)
	// require.Len(b, mf, 6)

	for i := 0; i < b.N; i++ {
		message := &model.Message{
			Timestamp:  timesync.Now().Unix(),
			Type:       model.MessageType_metric,
			Name:       "test",
			NodeState:  model.NodeState_down,
			AgentState: model.AgentState_healthy,
			Metric:     mf[0],
		}
		_ = proto.Size(message)
	}
}
