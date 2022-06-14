package global

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateLogFolders(t *testing.T) {
	testCases := []struct {
		paths []string
	}{
		{[]string{"/tmp/metrikad/randomfile", "relativeFolder/randomfile"}},
	}

	for _, tc := range testCases {
		AgentConf.Runtime.Log.Outputs = tc.paths
		err := createLogFolders()
		require.NoError(t, err)
		for _, path := range AgentConf.Runtime.Log.Outputs {
			_, err := os.Create(path)
			require.NoError(t, err)
			defer func() {
				pathSplit := strings.Split(path, "/")
				if len(pathSplit) == 1 {
					os.Remove(path)
				} else {
					os.RemoveAll(strings.Join(pathSplit[:len(pathSplit)-1], "/"))
				}
			}()
			_, err = os.Stat(path)
			require.NoError(t, err)
		}
	}
}
