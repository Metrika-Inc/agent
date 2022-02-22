package fingerprint

import "os"

const HostnameSrc = "hostname"

type Hostname struct{}

func (h Hostname) name() string {
	return HostnameSrc
}

func (h Hostname) bytes() ([]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return []byte(hostname), nil
}
