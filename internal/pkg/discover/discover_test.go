package discover

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPidOf(t *testing.T) {
	pid := getRealPID(t)
	fmt.Println(pid)
	require.NotEqual(t, 0, pid)

	result, err := pidOf("404noPID")
	require.Equal(t, 0, result)
	fmt.Println(err)
	expectedError := &exec.ExitError{}
	require.ErrorAs(t, err, &expectedError)
}

func TestPidArgv(t *testing.T) {
	pid := getRealPID(t)
	args, err := pidArgs(pid)
	require.NoError(t, err)
	require.GreaterOrEqual(t, 1, len(args))
	fmt.Println(args)
}

func getRealPID(t *testing.T) int {
	pid, err := pidOf("init")
	if err != nil {
		pid, err = pidOf("systemd")
	}
	require.NoError(t, err)
	return pid
}
