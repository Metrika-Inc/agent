package watch

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ory/dockertest"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func setupInfluxContainer(ctx context.Context, wg *sync.WaitGroup, image, version string, port int32) (string, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", errors.Wrapf(err, "could not connect to Docker")
	}

	resource, err := pool.Run(image, version, []string{})
	if err != nil {
		return "", errors.Wrapf(err, "could not start resource")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		pool.Purge(resource)
	}()

	influxURL := "http://" + resource.GetHostPort(fmt.Sprintf("%d/tcp", port))
	if err := pool.Retry(func() error {
		res, err := http.Get(influxURL)
		if err != nil {
			return err
		}
		res.Body.Close()
		return nil
	}); err != nil {
		return "", errors.Wrapf(err, "could not connect to InfluxDB")
	}

	return influxURL, nil
}

func getRandomPort() int {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	ln.Close()

	return ln.Addr().(*net.TCPAddr).Port
}

func TestNewInfluxExporterWatch_WriteSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	defer cancel()

	reqURL, err := setupInfluxContainer(ctx, wg, "influxdb", "1.8", 8086)
	upstreamURL, err := url.Parse(reqURL)
	require.Nil(t, err)

	conf := InfluxExporterWatchConf{
		UpstreamURL:       upstreamURL,
		ListenAddr:        fmt.Sprintf("127.0.0.1:%d", getRandomPort()),
		ExporterActivated: true,
	}

	w := NewInfluxExporterWatch(conf)
	require.NotNil(t, w)
	w.StartUnsafe()
	defer w.Stop()

	// create a database
	createDb := []byte("q=CREATE DATABASE test")
	req, err := http.NewRequest("POST", reqURL+"/query", bytes.NewBuffer(createDb))
	require.Nil(t, err)

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// send a write request
	ts := time.Now().UTC()
	testPoint := []byte(fmt.Sprintf("testmetric field1=42.0 %d\n", ts.UnixNano()))
	req, err = http.NewRequest("POST", "http://"+conf.ListenAddr+"/write?db=test&precision=n", bytes.NewBuffer(testPoint))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err = (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// check upstream influx got the previous write
	req, err = http.NewRequest("POST", reqURL+"/query?db=test&q=SELECT%20*%20FROM%20testmetric", nil)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err = (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	gotBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	resp.Body.Close()

	expRes := []byte(`{"results":[{"statement_id":0,"series":[{"name":"testmetric","columns":["time","field1"],"values":[["<ts>",42]]}]}]}`)

	tsFmt := "2006-01-02T15:04:05.000000000Z"
	// ugly hack to avoid failing test due to
	// seconds fractional part being truncated by influx.
	// expected: "2023-02-21T18:21:39.857440970Z"
	// actual  : "2023-02-21T18:21:39.85744097Z"
	if strings.HasSuffix(ts.Format(tsFmt), "0Z") {
		tsFmt = "2006-01-02T15:04:05.00000000Z"
	}
	expResStr := strings.Replace(string(expRes), "<ts>", ts.Format(tsFmt), 1)
	gotBody = gotBody[:len(gotBody)-1] // remove new-line char coming from the query api

	require.Equal(t, expResStr, string(gotBody))

	pefOut, _ := ioutil.TempFile(os.TempDir(), "pef")
	pefOut.Close()
	defer os.Remove(pefOut.Name())

	prometheus.WriteToTextfile(pefOut.Name(), w.registry)

	gotPef, _ := ioutil.ReadFile(pefOut.Name())
	expContains := []byte(fmt.Sprintf("# HELP testmetric_field1 Solana InfluxDB metric\n# TYPE testmetric_field1 untyped\ntestmetric_field1 42 %d\n", ts.UnixMilli()))
	require.Contains(t, string(gotPef), string(expContains))
}

func TestNewInfluxExporterWatch_WriteError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	defer cancel()

	reqURL, err := setupInfluxContainer(ctx, wg, "influxdb", "1.8", 8086)
	upstreamURL, err := url.Parse(reqURL)
	require.Nil(t, err)

	conf := InfluxExporterWatchConf{
		UpstreamURL:       upstreamURL,
		ListenAddr:        fmt.Sprintf("127.0.0.1:%d", getRandomPort()),
		ExporterActivated: true,
	}

	w := NewInfluxExporterWatch(conf)
	require.NotNil(t, w)
	w.StartUnsafe()
	defer w.Stop()

	// create a database
	createDb := []byte("q=CREATE DATABASE test")
	req, err := http.NewRequest("POST", reqURL+"/query", bytes.NewBuffer(createDb))
	require.Nil(t, err)

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// send a write with a invalid Line format
	testPoint := []byte("invalidmetric field142.0 1675040877000000001\n")
	req, err = http.NewRequest("POST", "http://"+conf.ListenAddr+"/write?db=test&precision=n", bytes.NewBuffer(testPoint))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err = (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// send a write with an invalid timestamp
	testPoint = []byte("invalidmetric field1=42.0 1675040877000000001000\n")
	req, err = http.NewRequest("POST", "http://"+conf.ListenAddr+"/write?db=test&precision=n", bytes.NewBuffer(testPoint))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err = (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// send a valid write request
	ts := time.Now().UTC()
	testPoint = []byte(fmt.Sprintf("testmetric field1=42.0 %d\n", ts.UnixNano()))
	req, err = http.NewRequest("POST", "http://"+conf.ListenAddr+"/write?db=test&precision=n", bytes.NewBuffer(testPoint))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err = (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// check upstream influx got the previous write
	req, err = http.NewRequest("POST", reqURL+"/query?db=test&q=SELECT%20*%20FROM%20testmetric", nil)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err = (&http.Client{}).Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	gotBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	resp.Body.Close()

	expRes := []byte(`{"results":[{"statement_id":0,"series":[{"name":"testmetric","columns":["time","field1"],"values":[["<ts>",42]]}]}]}`)

	tsFmt := "2006-01-02T15:04:05.000000000Z"
	// ugly hack to avoid failing test due to
	// seconds fractional part being truncated by influx.
	// expected: "2023-02-21T18:21:39.857440970Z"
	// actual  : "2023-02-21T18:21:39.85744097Z"
	if strings.HasSuffix(ts.Format(tsFmt), "0Z") {
		tsFmt = "2006-01-02T15:04:05.00000000Z"
	}
	expResStr := strings.Replace(string(expRes), "<ts>", ts.Format(tsFmt), 1)

	gotBody = gotBody[:len(gotBody)-1] // remove new-line char coming from the query api
	require.Equal(t, expResStr, string(gotBody))

	pefOut, _ := ioutil.TempFile(os.TempDir(), "pef")
	pefOut.Close()
	defer os.Remove(pefOut.Name())

	prometheus.WriteToTextfile(pefOut.Name(), w.registry)

	gotPef, _ := ioutil.ReadFile(pefOut.Name())
	expContains := []byte(fmt.Sprintf("# HELP testmetric_field1 Solana InfluxDB metric\n# TYPE testmetric_field1 untyped\ntestmetric_field1 42 %d\n", ts.UnixMilli()))
	require.Contains(t, string(gotPef), string(expContains))
}

func TestNewInfluxExporterWatch_UpstreamDisabled(t *testing.T) {
	conf := InfluxExporterWatchConf{
		ListenAddr:        fmt.Sprintf("127.0.0.1:%d", getRandomPort()),
		ExporterActivated: true,
	}

	w := NewInfluxExporterWatch(conf)
	require.NotNil(t, w)
	w.StartUnsafe()
	defer w.Stop()

	// send a valid write request and expect a 204 even if upstream is disabled
	ts := time.Now().UTC()
	testPoint := []byte(fmt.Sprintf("testmetric field1=42.0 %d\n", ts.UnixNano()))
	req, err := http.NewRequest("POST", "http://"+conf.ListenAddr+"/write?db=test&precision=n", bytes.NewBuffer(testPoint))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	httpts := time.Now()
	var resp *http.Response
	time.Sleep(time.Millisecond)
	for resp, err = (&http.Client{}).Do(req); err != nil; {
		time.Sleep(time.Second)
		if time.Now().Sub(httpts) > 10*time.Second {
			require.Nilf(t, err, "timeout waiting for http server")
		}
	}
	resp.Body.Close()

	require.Nil(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	pefOut, _ := ioutil.TempFile(os.TempDir(), "pef")
	pefOut.Close()
	defer os.Remove(pefOut.Name())

	prometheus.WriteToTextfile(pefOut.Name(), w.registry)

	gotPef, _ := ioutil.ReadFile(pefOut.Name())
	expContains := []byte(fmt.Sprintf("# HELP testmetric_field1 Solana InfluxDB metric\n# TYPE testmetric_field1 untyped\ntestmetric_field1 42 %d\n", ts.UnixMilli()))
	require.Contains(t, string(gotPef), string(expContains))
}
