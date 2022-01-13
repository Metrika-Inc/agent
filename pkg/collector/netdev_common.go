// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !nonetdev && (linux || freebsd || openbsd || dragonfly || darwin)
// +build !nonetdev
// +build linux freebsd openbsd dragonfly darwin

package collector

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	// netdevDeviceInclude Regexp of net devices to include
	// (mutually exclusive to device-exclude)
	// collector.netdev.device-include
	netdevDeviceInclude = ""

	// oldNetdevDeviceInclude DEPRECATED: Use collector.netdev.device-include
	// collector.netdev.device-whitelist
	oldNetdevDeviceInclude = ""

	// netdevDeviceExclude Regexp of net devices to exclude
	// (mutually exclusive to device-include).
	// collector.netdev.device-exclude
	netdevDeviceExclude = ""

	// oldNetdevDeviceExclude
	// "DEPRECATED: Use collector.netdev.device-exclude")
	// collector.netdev.device-blacklist
	oldNetdevDeviceExclude = ""

	// netdevAddressInfo Collect address-info for every device
	// "collector.netdev.address-info"
	netdevAddressInfo = true
)

type netDevCollector struct {
	subsystem    string
	deviceFilter netDevFilter
	metricDescs  map[string]*prometheus.Desc
}

type netDevStats map[string]map[string]uint64

// NewNetDevCollector returns a new Collector exposing network device stats.
func NewNetDevCollector() (prometheus.Collector, error) {
	if oldNetdevDeviceInclude != "" {
		if netdevDeviceInclude == "" {
			log.Trace("msg", "--collector.netdev.device-whitelist is DEPRECATED and will be removed in 2.0.0, use --collector.netdev.device-include")
			netdevDeviceInclude = oldNetdevDeviceInclude
		} else {
			return nil, errors.New("--collector.netdev.device-whitelist and --collector.netdev.device-include are mutually exclusive")
		}
	}

	if oldNetdevDeviceExclude != "" {
		if netdevDeviceExclude == "" {
			log.Trace("msg", "--collector.netdev.device-blacklist is DEPRECATED and will be removed in 2.0.0, use --collector.netdev.device-exclude")
			netdevDeviceExclude = oldNetdevDeviceExclude
		} else {
			return nil, errors.New("--collector.netdev.device-blacklist and --collector.netdev.device-exclude are mutually exclusive")
		}
	}

	if netdevDeviceExclude != "" && netdevDeviceInclude != "" {
		return nil, errors.New("device-exclude & device-include are mutually exclusive")
	}

	if netdevDeviceExclude != "" {
		log.Debug("msg", "Parsed flag --collector.netdev.device-exclude", "flag", netdevDeviceExclude)
	}

	if netdevDeviceInclude != "" {
		log.Debug("msg", "Parsed Flag --collector.netdev.device-include", "flag", netdevDeviceInclude)
	}

	return &netDevCollector{
		subsystem:    "network",
		deviceFilter: newNetDevFilter(netdevDeviceExclude, netdevDeviceInclude),
		metricDescs:  map[string]*prometheus.Desc{},
	}, nil
}

func (c *netDevCollector) Collect(ch chan<- prometheus.Metric) {
	netDev, err := getNetDevStats(&c.deviceFilter)
	if err != nil {
		err = fmt.Errorf("couldn't get netstats: %w", err)
		log.Error(err)

		return
	}
	for dev, devStats := range netDev {
		for key, value := range devStats {
			desc, ok := c.metricDescs[key]
			if !ok {
				desc = prometheus.NewDesc(
					prometheus.BuildFQName(namespace, c.subsystem, key+"_total"),
					fmt.Sprintf("Network device statistic %s.", key),
					[]string{"device"},
					nil,
				)
				c.metricDescs[key] = desc
			}
			ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(value), dev)
		}
	}
	if netdevAddressInfo {
		interfaces, err := net.Interfaces()
		if err != nil {
			err = fmt.Errorf("could not get network interfaces: %w", err)
			log.Error(err)

			return
		}

		desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, "network_address",
			"info"), "node network address by device",
			[]string{"device", "address", "netmask", "scope"}, nil)

		for _, addr := range getAddrsInfo(interfaces) {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1,
				addr.device, addr.addr, addr.netmask, addr.scope)
		}
	}
}

type addrInfo struct {
	device  string
	addr    string
	scope   string
	netmask string
}

func scope(ip net.IP) string {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return "link-local"
	}

	if ip.IsInterfaceLocalMulticast() {
		return "interface-local"
	}

	if ip.IsGlobalUnicast() {
		return "global"
	}

	return ""
}

// getAddrsInfo returns interface name, address, scope and netmask for all interfaces.
func getAddrsInfo(interfaces []net.Interface) []addrInfo {
	var res []addrInfo

	for _, ifs := range interfaces {
		addrs, _ := ifs.Addrs()
		for _, addr := range addrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			size, _ := ipNet.Mask.Size()

			res = append(res, addrInfo{
				device:  ifs.Name,
				addr:    ip.String(),
				scope:   scope(ip),
				netmask: strconv.Itoa(size),
			})
		}
	}

	return res
}

func (c *netDevCollector) Describe(ch chan<- *prometheus.Desc) {
	netDev, err := getNetDevStats(&c.deviceFilter)
	if err != nil {
		err = fmt.Errorf("couldn't get netstats: %w", err)
		log.Error(err)

		return
	}
	for _, devStats := range netDev {
		for key := range devStats {
			desc, ok := c.metricDescs[key]
			if !ok {
				desc = prometheus.NewDesc(
					prometheus.BuildFQName(namespace, c.subsystem, key+"_total"),
					fmt.Sprintf("Network device statistic %s.", key),
					[]string{"device"},
					nil,
				)
			}
			ch <- desc
		}
	}
	if netdevAddressInfo {
		desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, "network_address",
			"info"), "node network address by device",
			[]string{"device", "address", "netmask", "scope"}, nil)

		ch <- desc
	}
}
