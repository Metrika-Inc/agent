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

//go:build !nofilesystem && (linux || freebsd || openbsd || darwin || dragonfly)
// +build !nofilesystem
// +build linux freebsd openbsd darwin dragonfly

package collector

import (
	"errors"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Arch-dependent implementation must define:
// * defMountPointsExcluded
// * defFSTypesExcluded
// * filesystemLabelNames
// * filesystemCollector.GetStats

var (
	mountPointsExcludeSet = true

	// mountPointsExclude Regexp of mount points to exclude for filesystem collector
	// collector.filesystem.mount-points-exclude
	mountPointsExclude = defMountPointsExcluded

	// oldMountPointsExcluded Regexp of mount points to ignore for filesystem collector
	// "collector.filesystem.ignored-mount-points",
	oldMountPointsExcluded = ""

	fsTypesExcludeSet = true

	// fsTypesExclude Regexp of filesystem types to exclude for filesystem collector
	// "collector.filesystem.fs-types-exclude",
	fsTypesExclude = defFSTypesExcluded

	// oldFSTypesExcluded
	// Regexp of filesystem types to ignore for filesystem collector
	// collector.filesystem.ignored-fs-types
	oldFSTypesExcluded = ""

	filesystemLabelNames = []string{"device", "mountpoint", "fstype"}
)

type filesystemCollector struct {
	excludedMountPointsPattern    *regexp.Regexp
	excludedFSTypesPattern        *regexp.Regexp
	sizeDesc, freeDesc, availDesc *prometheus.Desc
	filesDesc, filesFreeDesc      *prometheus.Desc
	roDesc, deviceErrorDesc       *prometheus.Desc
}

type filesystemLabels struct {
	device, mountPoint, fsType, options string
}

type filesystemStats struct {
	labels            filesystemLabels
	size, free, avail float64
	files, filesFree  float64
	ro, deviceError   float64
}

// NewFilesystemCollector returns a new Collector exposing filesystems stats.
func NewFilesystemCollector() (prometheus.Collector, error) {
	if oldMountPointsExcluded != "" {
		if !mountPointsExcludeSet {
			mountPointsExclude = oldMountPointsExcluded
		} else {
			return nil, errors.New("--collector.filesystem.ignored-mount-points and --collector.filesystem.mount-points-exclude are mutually exclusive")
		}
	}

	if oldFSTypesExcluded != "" {
		if !fsTypesExcludeSet {
			fsTypesExclude = oldFSTypesExcluded
		} else {
			return nil, errors.New("--collector.filesystem.ignored-fs-types and --collector.filesystem.fs-types-exclude are mutually exclusive")
		}
	}

	subsystem := "filesystem"
	mountPointPattern := regexp.MustCompile(mountPointsExclude)
	filesystemsTypesPattern := regexp.MustCompile(fsTypesExclude)

	sizeDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "size_bytes"),
		"Filesystem size in bytes.",
		filesystemLabelNames, nil,
	)

	freeDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "free_bytes"),
		"Filesystem free space in bytes.",
		filesystemLabelNames, nil,
	)

	availDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "avail_bytes"),
		"Filesystem space available to non-root users in bytes.",
		filesystemLabelNames, nil,
	)

	filesDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "files"),
		"Filesystem total file nodes.",
		filesystemLabelNames, nil,
	)

	filesFreeDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "files_free"),
		"Filesystem total free file nodes.",
		filesystemLabelNames, nil,
	)

	roDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "readonly"),
		"Filesystem read-only status.",
		filesystemLabelNames, nil,
	)

	deviceErrorDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "device_error"),
		"Whether an error occurred while getting statistics for the given device.",
		filesystemLabelNames, nil,
	)

	return &filesystemCollector{
		excludedMountPointsPattern: mountPointPattern,
		excludedFSTypesPattern:     filesystemsTypesPattern,
		sizeDesc:                   sizeDesc,
		freeDesc:                   freeDesc,
		availDesc:                  availDesc,
		filesDesc:                  filesDesc,
		filesFreeDesc:              filesFreeDesc,
		roDesc:                     roDesc,
		deviceErrorDesc:            deviceErrorDesc,
	}, nil
}

func (c *filesystemCollector) Collect(ch chan<- prometheus.Metric) {
	stats, err := c.GetStats()
	if err != nil {
		zap.S().Error(err)
	}
	// Make sure we expose a metric once, even if there are multiple mounts
	seen := map[filesystemLabels]bool{}
	for _, s := range stats {
		if seen[s.labels] {
			continue
		}
		seen[s.labels] = true

		ch <- prometheus.MustNewConstMetric(
			c.deviceErrorDesc, prometheus.GaugeValue,
			s.deviceError, s.labels.device, s.labels.mountPoint, s.labels.fsType,
		)
		if s.deviceError > 0 {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.sizeDesc, prometheus.GaugeValue,
			s.size, s.labels.device, s.labels.mountPoint, s.labels.fsType,
		)
		ch <- prometheus.MustNewConstMetric(
			c.freeDesc, prometheus.GaugeValue,
			s.free, s.labels.device, s.labels.mountPoint, s.labels.fsType,
		)
		ch <- prometheus.MustNewConstMetric(
			c.availDesc, prometheus.GaugeValue,
			s.avail, s.labels.device, s.labels.mountPoint, s.labels.fsType,
		)
		ch <- prometheus.MustNewConstMetric(
			c.filesDesc, prometheus.GaugeValue,
			s.files, s.labels.device, s.labels.mountPoint, s.labels.fsType,
		)
		ch <- prometheus.MustNewConstMetric(
			c.filesFreeDesc, prometheus.GaugeValue,
			s.filesFree, s.labels.device, s.labels.mountPoint, s.labels.fsType,
		)
		ch <- prometheus.MustNewConstMetric(
			c.roDesc, prometheus.GaugeValue,
			s.ro, s.labels.device, s.labels.mountPoint, s.labels.fsType,
		)
	}
}

func (c *filesystemCollector) Describe(ch chan<- *prometheus.Desc) {
	stats, err := c.GetStats()
	if err != nil {
		zap.S().Error(err)
	}
	// Make sure we expose a metric once, even if there are multiple mounts
	seen := map[filesystemLabels]bool{}
	for _, s := range stats {
		if seen[s.labels] {
			continue
		}
		seen[s.labels] = true

		ch <- c.deviceErrorDesc

		if s.deviceError > 0 {
			continue
		}

		ch <- c.sizeDesc
		ch <- c.freeDesc
		ch <- c.availDesc
		ch <- c.filesDesc
		ch <- c.filesFreeDesc
		ch <- c.roDesc
	}
}
