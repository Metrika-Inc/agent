package collector

import (
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Namespace defines the common namespace to be used by all metrics.
const namespace = "node"

var (
	// ErrNoData indicates the collector found no data to collect, but had no other error.
	ErrNoData = errors.New("collector returned no data")
)

func IsNoDataError(err error) bool {
	return err == ErrNoData
}

type NodeExporterCollectErr struct {
	Err error
}

func (n *NodeExporterCollectErr) Error() string {
	return fmt.Sprint("error collecting node exporter metrics: ", n.Err)
}

type typedDesc struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
}

func (d *typedDesc) mustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.desc, d.valueType, value, labels...)
}

type CollectorName string

var (
	PrometheusNetNetstat CollectorName = "prometheus.proc.net.netstat_linux"
	PrometheusNetARP     CollectorName = "prometheus.proc.net.arp_linux"
	PrometheusStat       CollectorName = "prometheus.proc.stat_linux"
	PrometheusConntrack  CollectorName = "prometheus.proc.conntrack_linux"
	PrometheusCPU        CollectorName = "prometheus.proc.cpu"
	PrometheusDiskStats  CollectorName = "prometheus.proc.diskstats"
	PrometheusEntropy    CollectorName = "prometheus.proc.entropy"
	PrometheusFileFD     CollectorName = "prometheus.proc.filefd"
	PrometheusFilesystem CollectorName = "prometheus.proc.filesystem"
	PrometheusLoadAvg    CollectorName = "prometheus.proc.loadavg"
	PrometheusMemInfo    CollectorName = "prometheus.proc.meminfo"
	PrometheusNetClass   CollectorName = "prometheus.proc.netclass"
	PrometheusNetDev     CollectorName = "prometheus.proc.netdev"
	PrometheusSockStat   CollectorName = "prometheus.proc.sockstat"
	PrometheusTextfile   CollectorName = "prometheus.proc.textfile"
	PrometheusTime       CollectorName = "prometheus.time"
	PrometheusUname      CollectorName = "prometheus.uname"
	PrometheusVMStat     CollectorName = "prometheus.vmstat"

	CollectorsFactory = map[string]func() (prometheus.Collector, error){
		string(PrometheusNetNetstat): NewNetStatCollector,
		string(PrometheusNetARP):     NewARPCollector,
		string(PrometheusStat):       NewStatCollector,
		string(PrometheusConntrack):  NewConntrackCollector,
		string(PrometheusCPU):        NewCPUCollector,
		string(PrometheusDiskStats):  NewDiskstatsCollector,
		string(PrometheusEntropy):    NewEntropyCollector,
		string(PrometheusFileFD):     NewFileFDStatCollector,
		string(PrometheusFilesystem): NewFilesystemCollector,
		string(PrometheusLoadAvg):    NewLoadavgCollector,
		string(PrometheusMemInfo):    NewMeminfoCollector,
		string(PrometheusNetClass):   NewNetClassCollector,
		string(PrometheusNetDev):     NewNetDevCollector,
		string(PrometheusSockStat):   NewSockStatCollector,
		string(PrometheusTextfile):   NewTextFileCollector,
		string(PrometheusTime):       NewTimeCollector,
		string(PrometheusUname):      NewUnameCollector,
		string(PrometheusVMStat):     NewvmStatCollector,
	}
)
