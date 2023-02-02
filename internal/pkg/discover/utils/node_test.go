// Copyright 2022 Metrika Inc.
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

package utils

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"agent/internal/pkg/global"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	l, _ := zap.NewProduction()
	zap.ReplaceGlobals(l)
	m.Run()
}

func TestNodeDiscovererNew(t *testing.T) {
	conf := NodeDiscovererConfig{UnitGlob: []string{"foobar"}}
	dsc, err := NewNodeDiscoverer(conf)

	require.Nil(t, err)
	require.NotNil(t, dsc)
}

func TestNodeDiscovererNewError(t *testing.T) {
	conf := NodeDiscovererConfig{}
	dsc, err := NewNodeDiscoverer(conf)

	require.NotNil(t, err)
	require.Nil(t, dsc)
}

func TestNodeDiscovererDetectSystemdService(t *testing.T) {
	defer DefaultDockerAdapter.Close()

	conf := NodeDiscovererConfig{UnitGlob: []string{"dbus.service"}}
	dsc, err := NewNodeDiscoverer(conf)

	require.Nil(t, err)
	require.NotNil(t, dsc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	unit, err := dsc.DetectSystemdService(ctx)
	require.Nil(t, err)

	require.NotNil(t, unit)
	require.Equal(t, "dbus.service", unit.Name)
}

func TestNodeDiscovererDetectDockerContainer(t *testing.T) {
	defer DefaultDockerAdapter.Close()

	out, err := ioutil.ReadFile("testcases/containers.json")
	require.NoError(t, err)

	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(out)
	})

	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	os.Setenv("DOCKER_HOST", ts.URL)
	defer os.Unsetenv("DOCKER_HOST")

	conf := NodeDiscovererConfig{ContainerRegex: []string{"ctlptl-registry"}}
	dsc, err := NewNodeDiscoverer(conf)

	require.Nil(t, err)
	require.NotNil(t, dsc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	container, err := dsc.DetectDockerContainer(ctx)
	require.Nil(t, err)

	require.NotNil(t, container)
	require.Equal(t, "/ctlptl-registry", container.Names[0])
}

func TestNodeDiscovererDetectSchemeOnlyDocker(t *testing.T) {
	defer DefaultDockerAdapter.Close()

	out, err := ioutil.ReadFile("testcases/containers.json")
	require.NoError(t, err)

	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(out)
	})

	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	os.Setenv("DOCKER_HOST", ts.URL)
	defer os.Unsetenv("DOCKER_HOST")

	conf := NodeDiscovererConfig{ContainerRegex: []string{"ctlptl-registry"}}
	dsc, err := NewNodeDiscoverer(conf)

	require.Nil(t, err)
	require.NotNil(t, dsc)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resCh := make(chan global.NodeRunScheme)
	go func() {
		scheme := dsc.DetectScheme(ctx)
		resCh <- scheme
	}()

	select {
	case got := <-resCh:
		require.Equal(t, global.NodeDocker, got)
	case <-time.After(10 * time.Second):
		t.Error("timeout waiting for DetectScheme goroutine")
	}
}

func TestNodeDiscovererDetectSchemeOnlySystemd(t *testing.T) {
	defer DefaultDockerAdapter.Close()

	conf := NodeDiscovererConfig{UnitGlob: []string{"dbus.service"}}
	dsc, err := NewNodeDiscoverer(conf)

	require.Nil(t, err)
	require.NotNil(t, dsc)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resCh := make(chan global.NodeRunScheme)
	go func() {
		scheme := dsc.DetectScheme(ctx)
		resCh <- scheme
	}()

	select {
	case got := <-resCh:
		require.Equal(t, global.NodeSystemd, got)
	case <-time.After(10 * time.Second):
		t.Error("timeout waiting for DetectScheme goroutine")
	}
}

func TestNodeDiscovererDetectScheme(t *testing.T) {
	defer DefaultDockerAdapter.Close()

	out, err := ioutil.ReadFile("testcases/containers.json")
	require.NoError(t, err)

	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(out)
	})

	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	os.Setenv("DOCKER_HOST", ts.URL)
	defer os.Unsetenv("DOCKER_HOST")

	tests := []struct {
		name      string
		dscConf   NodeDiscovererConfig
		expScheme global.NodeRunScheme
	}{
		{
			name: "docker",
			dscConf: NodeDiscovererConfig{
				ContainerRegex: []string{"ctlptl-registry"},
			},
			expScheme: global.NodeDocker,
		},
		{
			name: "systemd",
			dscConf: NodeDiscovererConfig{
				UnitGlob: []string{"dbus.service"},
			},
			expScheme: global.NodeSystemd,
		},
		{
			name: "docker+systemd",
			dscConf: NodeDiscovererConfig{
				UnitGlob:       []string{"dbus.service"},
				ContainerRegex: []string{"ctlptl-registry"},
			},
			expScheme: global.NodeDocker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsc, err := NewNodeDiscoverer(tt.dscConf)

			require.Nil(t, err)
			require.NotNil(t, dsc)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			scheme := dsc.DetectScheme(ctx)
			cancel()

			require.Nil(t, err)
			require.Equal(t, tt.expScheme, scheme)
		})
	}
}
