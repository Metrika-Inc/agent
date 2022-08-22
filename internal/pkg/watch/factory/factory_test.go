package factory

import (
	"context"
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/global"
	"agent/internal/pkg/watch"

	"github.com/stretchr/testify/require"
)

type mockEmitter struct {
	ch chan interface{}
}

func newMockEmitter(ch chan interface{}) *mockEmitter {
	return &mockEmitter{ch: ch}
}

func (m *mockEmitter) Emit(message interface{}) {
	m.ch <- message
}

var tests = []string{
	"prometheus.proc.net.netstat_linux",
	"prometheus.proc.net.arp_linux",
	"prometheus.proc.stat_linux",
	"prometheus.proc.conntrack_linux",
	"prometheus.proc.cpu",
	"prometheus.proc.diskstats",
	"prometheus.proc.entropy",
	"prometheus.proc.filefd",
	"prometheus.proc.filesystem",
	"prometheus.proc.loadavg",
	"prometheus.proc.meminfo",
	"prometheus.proc.netclass",
	"prometheus.proc.netdev",
	"prometheus.proc.sockstat",
	"prometheus.proc.textfile",
	"prometheus.time",
	"prometheus.uname",
	"prometheus.vmstat",
}

func TestNewWatcherByType(t *testing.T) {
	emitch := make(chan interface{})
	testch := make(chan interface{})
	mockEmit := newMockEmitter(emitch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case msg, _ := <-testch:
				mockEmit.Emit(msg)
			case <-ctx.Done():
				return
			}
		}
	}()

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			conf := global.WatchConfig{
				Type:             tt,
				SamplingInterval: 50 * time.Millisecond,
			}

			w := NewWatcherByType(conf)
			require.Implements(t, new(watch.Watcher), w)

			w.Subscribe(testch)

			watch.Start(w)

			select {
			case msg, _ := <-testch:
				require.IsType(t, &model.Message{}, msg)
			case <-time.After(2 * time.Second):
				t.Error("timeout waiting for prometheus.cpu metrics")
			}

			w.Stop()
		})
	}
}
