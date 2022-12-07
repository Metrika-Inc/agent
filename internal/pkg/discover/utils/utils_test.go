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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	dt "github.com/docker/docker/api/types"
	"github.com/stretchr/testify/require"
)

func TestGetEnvFromFile(t *testing.T) {
	path := "testcases/correctEnvFile"
	envs, err := GetEnvFromFile("testcases/correctEnvFile")
	require.NoError(t, err)
	require.Len(t, envs, 4)

	out, err := ioutil.ReadFile(path)
	require.NoError(t, err)

	for k, v := range envs {
		r, err := regexp.Compile(fmt.Sprintf("%v=%v", k, v))
		require.NoError(t, err, "test issue, invalid regex formed")
		if ok := r.Match(out); !ok {
			t.Fatalf("Could not find the env var %v = %v", k, v)
		}
	}
}

func TestGetLogLine(t *testing.T) {
	t.Run("GetLogLine happy case", func(t *testing.T) {
		logFile := "testcases/logs/correctLog.json"
		expectedFile := "testcases/logs/firstLogLine.json"
		f, err := os.Open(logFile)
		require.NoError(t, err)
		defer f.Close()

		scan := bufio.NewScanner(f)
		line, err := GetLogLine(scan)
		require.NoError(t, err)

		expected, err := ioutil.ReadFile(expectedFile)
		require.NoError(t, err)

		require.Equal(t, expected, line)
	})
	t.Run("GetLogLine empty file", func(t *testing.T) {
		b := make([]byte, 0, 100)
		buf := bytes.NewBuffer(b)
		scan := bufio.NewScanner(buf)
		line, err := GetLogLine(scan)
		require.ErrorIs(t, err, ErrEmptyLogFile)
		require.Nil(t, line)
	})

	t.Run("GetLogLine thorough check", func(t *testing.T) {
		logFile := "testcases/logs/correctLog.json"
		out, err := os.ReadFile(logFile)
		require.NoError(t, err)
		lines := strings.Split(strings.Trim(string(out), "\n"), "\n")
		reader := bytes.NewBuffer(out)
		scan := bufio.NewScanner(reader)
		for i, line := range lines {
			logLine, err := GetLogLine(scan)
			require.NoError(t, err)
			require.Equal(t, line, string(logLine), "line %d", i)
		}
	})
}

func TestMatchContainer(t *testing.T) {
	out, err := ioutil.ReadFile("testcases/containers.json")
	require.NoError(t, err)
	var containers []dt.Container
	err = json.Unmarshal(out, &containers)
	require.NoError(t, err)

	testCases := []struct {
		matchers       []string
		expectedResult int // index; -1 == not found
	}{
		{[]string{"ctlptl"}, 0},
		{[]string{"fake", "ctlptl"}, 0},
		{[]string{"registry:2"}, 0},
		{[]string{"service"}, 1},
		{[]string{"fake", "service"}, 1},
		{[]string{"service:latest"}, 1},
		{[]string{"fake"}, -1},
	}

	for _, tc := range testCases {
		container, err := MatchContainer(containers, tc.matchers)
		if tc.expectedResult < 0 {
			require.ErrorIs(t, err, ErrContainerNotFound)
		} else {
			require.Equal(t, containers[tc.expectedResult], container)
		}
	}
}

func TestGetRunningContainers(t *testing.T) {
	out, err := ioutil.ReadFile("testcases/containers.json")
	require.NoError(t, err)

	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(out)
	})

	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	os.Setenv("DOCKER_HOST", ts.URL)
	defer os.Unsetenv("DOCKER_HOST")

	containers, err := GetRunningContainers()
	require.Nil(t, err)

	var expcontainers []dt.Container
	err = json.Unmarshal(out, &expcontainers)
	require.NoError(t, err)

	require.Equal(t, expcontainers, containers)
}
