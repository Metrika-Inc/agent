package ec2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/require"
)

var (
	mockRespInstanceID = []byte(`02aab8a4-74ef-476e-8182-f6d2ba4166a6`)
	mockRespHostname   = []byte(`mock-hostname`)
)

func TestHostname(t *testing.T) {
	tests := []struct {
		name        string
		handleFunc  http.HandlerFunc
		expHostname string
	}{
		{
			name: "both available",
			handleFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "instance-id") {
					w.Write(mockRespInstanceID)
				} else {
					w.Write(mockRespHostname)
				}
			}),
			expHostname: "02aab8a4-74ef-476e-8182-f6d2ba4166a6",
		},
		{
			name: "instance-id 404",
			handleFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "instance-id") {
					w.WriteHeader(http.StatusNotFound)
				} else {
					w.Write(mockRespHostname)
				}
			}),
			expHostname: "mock-hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handleFunc)
			defer ts.Close()

			defaultOptionsFuncWas := defaultOptionsFunc
			newOptionsFunc := func() imds.Options {
				return imds.Options{Endpoint: ts.URL}
			}
			defaultOptionsFunc = newOptionsFunc
			defer func() { defaultOptionsFunc = defaultOptionsFuncWas }()

			c := NewSearch()
			got, err := c.Hostname()
			require.Nil(t, err)

			require.Equal(t, tt.expHostname, got)
		})
	}
}
