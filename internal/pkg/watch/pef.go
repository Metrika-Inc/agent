package watch

import (
	"bytes"
	"strings"

	"agent/api/v1/model"
	"agent/pkg/parse/openmetrics"
	"agent/pkg/timesync"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PEFWatch struct {
	Watch
	PEFWatchConf
	httpWatch Watcher

	httpDataCh chan interface{}
}

type PEFWatchConf struct {
	Filter *openmetrics.PEFFilter
}

func NewPEFWatch(conf PEFWatchConf, httpWatch Watcher) *PEFWatch {
	p := &PEFWatch{
		Watch:        NewWatch(),
		PEFWatchConf: conf,
		httpWatch:    httpWatch,
		httpDataCh:   make(chan interface{}, 10),
	}

	return p
}

func (p *PEFWatch) StartUnsafe() {
	p.Watch.StartUnsafe()

	p.httpWatch.Subscribe(p.httpDataCh)
	Start(p.httpWatch)

	p.wg.Add(1)
	go p.parseAndEmit()
}

func (p *PEFWatch) parseAndEmit() {
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
				openMetricFam, err := dtoToOpenMetrics(family)
				if err != nil {
					p.Log.Errorw("failed to convert to openmetrics", zap.Error(err))

					continue
				}

				t := timesync.Default.Now()
				for _, m := range openMetricFam.GetMetrics() {
					for _, mp := range m.GetMetricPoints() {
						if mp != nil {
							mp.Timestamp = timestamppb.New(t)
						}
					}
				}

				msg := &model.Message{
					Name:  "pef." + strings.ToLower(*family.Name),
					Value: &model.Message_MetricFamily{MetricFamily: openMetricFam},
				}
				p.Emit(msg)
			}
		case <-p.StopKey:
			return
		}
	}
}

func (p *PEFWatch) Stop() {
	p.httpWatch.Stop()
	p.Watch.Stop()
}
