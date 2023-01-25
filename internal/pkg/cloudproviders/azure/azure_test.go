package azure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

var mockResp = []byte(`02aab8a4-74ef-476e-8182-f6d2ba4166a6`)

func TestHostname(t *testing.T) {
	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(mockResp)
	})

	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	requestWas := newRequest
	newRequest = func() *http.Request {
		req, _ := http.NewRequest("GET", ts.URL, nil)
		return req
	}
	defer func() { newRequest = requestWas }()

	got, err := Hostname()
	require.Nil(t, err)

	require.Equal(t, "02aab8a4-74ef-476e-8182-f6d2ba4166a6", got)
}
