package watch

import (
	"agent/api/v1/model"
	"agent/pkg/parse/openmetrics"
	"agent/pkg/timesync"
	"bytes"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

type PefWatch struct {
	Watch
	PefWatchConf
	httpWatch Watcher

	httpDataCh chan interface{}
	wg         *sync.WaitGroup
}

type PefWatchConf struct {
	Interval time.Duration
	Filter   *openmetrics.PEFFilter
}

func NewPefWatch(conf PefWatchConf, httpWatch Watcher) *PefWatch {
	p := &PefWatch{
		Watch:        NewWatch(),
		PefWatchConf: conf,
		httpWatch:    httpWatch,
		httpDataCh:   make(chan interface{}, 10),
		wg:           new(sync.WaitGroup),
	}

	return p
}

func (p *PefWatch) StartUnsafe() {
	p.Watch.StartUnsafe()

	p.httpWatch.Subscribe(p.httpDataCh)
	Start(p.httpWatch)

	p.wg.Add(1)
	go p.parseAndEmit()
}

func (p *PefWatch) parseAndEmit() {
	defer p.wg.Done()
	for {
		select {
		case r := <-p.httpDataCh:
			pefData, ok := r.([]byte)
			if !ok {
				p.Log.Error("type assertion failed")
				continue
			}

			pefReader := bytes.NewBuffer(pefData)
			mf, err := openmetrics.ParsePEF(pefReader, p.Filter)
			if err != nil {
				p.Log.Errorw("failed to parse PEF metrics", zap.Error(err))
				continue
			}

			for _, family := range mf {
				data, err := proto.Marshal(family)
				if err != nil {
					p.Log.Errorw("failed to marshal MetricFamily", zap.Error(err))
					continue
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
	p.wg.Wait()
}
