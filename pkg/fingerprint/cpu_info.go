package fingerprint

import (
	"encoding/json"
	"fmt"

	"github.com/shirou/gopsutil/cpu"
)

const CPUInfoSrc = "cpu_info"

type CPUInfo struct{}

func (c CPUInfo) name() string {
	return CPUInfoSrc
}

func (c CPUInfo) bytes() ([]byte, error) {
	infoStats, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	if len(infoStats) < 1 {
		return nil, fmt.Errorf("fingerprint error, empty info stats")
	}

	infoStatsBytes, err := json.Marshal(infoStats[0])
	if err != nil {
		return nil, err
	}

	return infoStatsBytes, nil
}
