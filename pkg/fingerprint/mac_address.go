package fingerprint

import (
	"fmt"
	"net"
)

const MACAddressSrc = "mac_address"

type MACAddress struct{}

func (m MACAddress) name() string {
	return MACAddressSrc
}

func (m MACAddress) bytes() ([]byte, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			return []byte(iface.HardwareAddr.String()), nil
		}
	}

	return nil, fmt.Errorf("no interface up (IFF_UP)")
}
