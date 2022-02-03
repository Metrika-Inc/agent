package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"testing"

	dt "github.com/docker/docker/api/types"
	"github.com/stretchr/testify/require"
)

func TestPidOf(t *testing.T) {
	pid := getRealPID(t)
	require.NotEqual(t, 0, pid)

	result, err := PidOf("404noPID")
	require.Equal(t, 0, result)
	expectedError := &exec.ExitError{}
	require.ErrorAs(t, err, &expectedError)
}

func TestPidArgs(t *testing.T) {
	t.Run("PidArgs happy case", func(t *testing.T) {
		pid := getRealPID(t)
		args, err := PidArgs(pid)
		require.NoError(t, err)
		require.GreaterOrEqual(t, 1, len(args))
	})
	t.Run("PidArgs pid not found", func(t *testing.T) {
		pid := -123
		args, err := PidArgs(pid)
		require.Error(t, err)
		require.Nil(t, args)
	})

}

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

func getRealPID(t *testing.T) int {
	pid, err := PidOf("init")
	if err != nil {
		pid, err = PidOf("systemd")
	}
	require.NoError(t, err)
	return pid
}

func TestGetLogLine(t *testing.T) {
	t.Run("GetLogLine happy case", func(t *testing.T) {
		logFile := "testcases/logs/correctLog.json"
		expectedFile := "testcases/logs/firstLogLine.json"
		f, err := os.Open(logFile)
		require.NoError(t, err)
		defer f.Close()

		line, err := GetLogLine(f)
		require.NoError(t, err)

		expected, err := ioutil.ReadFile(expectedFile)
		require.NoError(t, err)

		require.Equal(t, expected, line)
	})
	t.Run("GetLogLine empty file", func(t *testing.T) {
		b := make([]byte, 0, 100)
		buf := bytes.NewBuffer(b)
		line, err := GetLogLine(buf)
		require.ErrorIs(t, err, ErrEmptyLogFile)
		require.Nil(t, line)
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
