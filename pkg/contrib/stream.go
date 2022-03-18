package contrib

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

var (
	DefaultStreamRegisterer = new(StreamRegisterer)
	ReadStreamRegistry      *StreamRegisterer
	OutPath                 = filepath.Join(global.AgentCacheDir, "agent_stream.log")
)

type StreamRegisterer struct {
	streams []Stream
}

func (r *StreamRegisterer) Register(w ...Stream) error {
	r.streams = append(r.streams, w...)

	return nil
}

func (r *StreamRegisterer) Start(ctx context.Context, wg *sync.WaitGroup, ch chan interface{}) error {
	for _, s := range r.streams {
		s.Start(ctx, wg, ch)
	}

	return nil
}

type Stream interface {
	Start(ctx context.Context, wg *sync.WaitGroup, ch chan interface{})
}

type FileStream struct {
	ch chan interface{}
	f  *os.File
}

type LogLine struct {
	*model.Message

	AgentUUID string              `json:"agent_uuid"`
	Mf        *model.MetricFamily `json:"mf,omitempty"`
	Ev        *model.Event        `json:"ev,omitempty"`
}

func (m *FileStream) handle(msgi interface{}) error {
	log := zap.S()

	msg, ok := msgi.(model.Message)
	if !ok {
		log.Warnf("type assertion failed: %T", msg)
	}

	bytes, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	if _, err := m.f.Write(append(bytes, '\n')); err != nil {
		return err
	}

	return nil
}

func (m *FileStream) Start(ctx context.Context, wg *sync.WaitGroup, ch chan interface{}) {
	log := zap.S()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}

				if err := m.handle(msg); err != nil {
					log.Fatal(err)
				}
			case <-ctx.Done():
				if err := m.f.Close(); err != nil {
					log.Warnf("close error: %v", err)
				}

				return
			}
		}
	}()
}

func NewFileStream() (*FileStream, error) {
	f, err := os.OpenFile(OutPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o755)
	if err != nil {
		return nil, err
	}

	return &FileStream{f: f}, nil
}
