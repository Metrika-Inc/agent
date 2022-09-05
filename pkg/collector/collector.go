// Package collector includes all individual collectors to gather and export system metrics.
package collector

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Namespace defines the common namespace to be used by all metrics.
const namespace = "node"

var (
	// ErrNoData indicates the collector found no data to collect, but had no other error.
	ErrNoData = errors.New("collector returned no data")

	log *zap.SugaredLogger
)

type typedDesc struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
}

func (d *typedDesc) mustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.desc, d.valueType, value, labels...)
}

// Name key type for accessing CollectorsFactory
type Name string

var (
	prometheusNetNetstat Name = "prometheus.proc.net.netstat_linux"
	prometheusNetARP     Name = "prometheus.proc.net.arp_linux"
	prometheusStat       Name = "prometheus.proc.stat_linux"
	prometheusConntrack  Name = "prometheus.proc.conntrack_linux"
	prometheusCPU        Name = "prometheus.proc.cpu"
	prometheusDiskStats  Name = "prometheus.proc.diskstats"
	prometheusEntropy    Name = "prometheus.proc.entropy"
	prometheusFileFD     Name = "prometheus.proc.filefd"
	prometheusFilesystem Name = "prometheus.proc.filesystem"
	prometheusLoadAvg    Name = "prometheus.proc.loadavg"
	prometheusMemInfo    Name = "prometheus.proc.meminfo"
	prometheusNetClass   Name = "prometheus.proc.netclass"
	prometheusNetDev     Name = "prometheus.proc.netdev"
	prometheusSockStat   Name = "prometheus.proc.sockstat"
	prometheusTextfile   Name = "prometheus.proc.textfile"
	prometheusTime       Name = "prometheus.time"
	prometheusUname      Name = "prometheus.uname"
	prometheusVMStat     Name = "prometheus.vmstat"

	// CollectorsFactory map of contrustors per node exporter collector
	CollectorsFactory = map[Name]func() (prometheus.Collector, error){
		prometheusNetNetstat: NewNetStatCollector,
		prometheusNetARP:     NewARPCollector,
		prometheusStat:       NewStatCollector,
		prometheusConntrack:  NewConntrackCollector,
		prometheusCPU:        NewCPUCollector,
		prometheusDiskStats:  NewDiskstatsCollector,
		prometheusEntropy:    NewEntropyCollector,
		prometheusFileFD:     NewFileFDStatCollector,
		prometheusFilesystem: NewFilesystemCollector,
		prometheusLoadAvg:    NewLoadavgCollector,
		prometheusMemInfo:    NewMeminfoCollector,
		prometheusNetClass:   NewNetClassCollector,
		prometheusNetDev:     NewNetDevCollector,
		prometheusSockStat:   NewSockStatCollector,
		prometheusTextfile:   NewTextFileCollector,
		prometheusTime:       NewTimeCollector,
		prometheusUname:      NewUnameCollector,
		prometheusVMStat:     NewvmStatCollector,
	}
)
