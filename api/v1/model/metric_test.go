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

func TestMessageBytes(t *testing.T) {
	baseSize := 104
	tests := []struct {
		name     string
		metric   Message
		expBytes int
	}{
		{
			name: "basic",
			metric: Message{
				Timestamp:  1,
				Type:       1,
				NodeState:  1,
				AgentState: 1,
				Name:       newString(0),
				Body:       []byte(newString(32))},
			expBytes: baseSize + len(newString(0)) + len([]byte(newString(32))),
		},
		{
			name: "basic with padding",
			metric: Message{
				Timestamp: 1,
				Type:      1,
				Name:      newString(6),
				Body:      []byte(newString(7)),
			},
			expBytes: baseSize + len(newString(8)) + len([]byte(newString(8))),
		},
		{
			name: "basic with padding again",
			metric: Message{
				Timestamp: 1,
				Type:      1,
				Name:      newString(34),
				Body:      []byte(newString(34)),
			},
			expBytes: baseSize + len(newString(40)) + len([]byte(newString(40))),
		},
		{
			name: "body nil",
			metric: Message{
				Timestamp: 1,
				Type:      1,
				Name:      newString(0),
				Body:      []byte(nil),
			},
			expBytes: baseSize + len(newString(0)) + len([]byte(nil)),
		},
		{
			name: "body empty string",
			metric: Message{
				Timestamp: 1,
				Type:      1,
				Name:      newString(8),
				Body:      []byte(newString(0)),
			},
			expBytes: baseSize + len(newString(8)) + len([]byte(newString(0))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metric.Bytes()
			assert.Equal(t, tt.expBytes, int(got))
		})
	}
}
