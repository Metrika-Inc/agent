package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newString(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += "s"
	}

	return s
}

func TestMetricPlatformBytes(t *testing.T) {
	tests := []struct {
		name     string
		metric   MetricPlatform
		expBytes int
	}{
		{
			name:     "basic",
			metric:   MetricPlatform{1, 1, newString(0), []byte(newString(32))},
			expBytes: 56 + len(newString(0)) + len([]byte(newString(32))),
		},
		{
			name:     "basic with padding",
			metric:   MetricPlatform{1, 1, newString(6), []byte(newString(6))},
			expBytes: 56 + len(newString(8)) + len([]byte(newString(8))),
		},
		{
			name:     "basic with padding again",
			metric:   MetricPlatform{1, 1, newString(34), []byte(newString(34))},
			expBytes: 56 + len(newString(40)) + len([]byte(newString(40))),
		},
		{
			name:     "body nil",
			metric:   MetricPlatform{1, 1, newString(0), []byte(nil)},
			expBytes: 56 + len(newString(0)) + len([]byte(nil)),
		},
		{
			name:     "body empty string",
			metric:   MetricPlatform{1, 1, newString(8), []byte(newString(0))},
			expBytes: 56 + len(newString(8)) + len([]byte(newString(0))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metric.Bytes()
			assert.Equal(t, int(tt.expBytes), int(got))
		})
	}
}
