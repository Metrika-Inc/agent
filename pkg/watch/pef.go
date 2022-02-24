package watch

import (
	"agent/api/v1/model"
	"agent/pkg/parse/openmetrics"
	"agent/pkg/timesync"
	"bytes"
	"time"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

type PefWatch struct {
	Watch
	PefWatchConf
	httpWatch Watcher

	httpDataCh chan interface{}
}

type PefWatchConf struct {
	Interval time.Duration
	Filter openmetrics.PEFFilter
}

func NewPefWatch(conf PefWatchConf, httpWatch Watcher) *PefWatch {
	p := &PefWatch{
		Watch:        NewWatch(),
		PefWatchConf: conf,
		httpWatch:    httpWatch,
		httpDataCh:   make(chan interface{}, 10),
	}

	return p
}

func (p *PefWatch) StartUnsafe() {
	p.Watch.StartUnsafe()

	p.httpWatch.Subscribe(p.httpDataCh)
	Start(p.httpWatch)

	go p.parseAndEmit()
}

func (p *PefWatch) parseAndEmit() {
	for {
		select {
		case r := <-p.httpDataCh:
			pefData, ok := r.([]byte)
			if !ok {
				p.Log.Error("type assertion failed")
			}

			pefReader := bytes.NewBuffer(pefData)
			mf, err := openmetrics.ParsePEF(pefReader, nil)
			if err != nil {
				p.Log.Errorw("failed to parse PEF metrics", zap.Error(err))
			}

			for _, family := range mf {
				data, err := proto.Marshal(family)
				if err != nil {
					p.Log.Errorw("failed to marshal MetricFamily", zap.Error(err))
				}
				msg := model.Message{
					Timestamp: timesync.Now().UnixMilli(),
					Type:      model.MessageType_metric,
					Name:      "pef.metric",
					Body:      data,
				}
				p.Emit(msg)
			}
		case <-p.StopKey:
			return
		}
	}
}

func (p *PefWatch) Stop() {
	p.httpWatch.Stop()
	p.Watch.Stop()
}
