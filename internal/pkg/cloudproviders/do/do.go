package do

import (
	"fmt"
	"net/http"

	"github.com/digitalocean/go-metadata"
)

var client *metadata.Client

func init() {
	client = metadata.NewClient(metadata.WithHTTPClient(&http.Client{Transport: http.DefaultTransport}))
}

func IsRunningOn() bool {
	mt, err := client.Metadata()
	if err != nil {
		return false
	}

	if mt.DropletID == 0 {
		return false
	}

	return true
}

func Hostname() (string, error) {
	mt, err := client.Metadata()
	if err != nil {
		return "", err
	}

	if len(mt.Hostname) == 0 {
		return "", fmt.Errorf("empty hostname")
	}

	return mt.Hostname, nil
}
