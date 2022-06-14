package gce

import (
	"net/http"

	"cloud.google.com/go/compute/metadata"
)

var client *metadata.Client

func init() {
	client = metadata.NewClient(&http.Client{Transport: http.DefaultTransport})
}

func IsRunningOn() bool {
	return metadata.OnGCE()
}

func Hostname() (string, error) {
	return client.Hostname()
}
