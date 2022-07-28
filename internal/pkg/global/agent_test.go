package global

import (
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentPrepareStartup(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	t.Log(tmpdir)

	err := AgentPrepareStartup()
	require.Nil(t, err)

	expCacheDir := ".cache"
	files, err := ioutil.ReadDir(tmpdir)
	require.Nil(t, err)

	gotFiles := []string{}
	for _, file := range files {
		gotFiles = append(gotFiles, file.Name())
	}
	require.Lenf(t, files, 1, "expected only %s dir but got %v", expCacheDir, gotFiles)
	gotFile := files[0]
	require.True(t, gotFile.IsDir())

	require.Equal(t, expCacheDir, gotFiles[0])

	// now check .cache/ contents
	files, err = ioutil.ReadDir(filepath.Join(tmpdir, gotFile.Name()))
	require.Nil(t, err)

	gotFiles = []string{}
	for _, file := range files {
		gotFiles = append(gotFiles, file.Name())
	}

	require.Len(t, files, 1)
	require.Equal(t, "ma_fingerprint", files[0].Name())
}

func TestAgentPrepareStartup_FingerpintMismatch(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	t.Log(tmpdir)

	err := AgentPrepareStartup()
	require.Nil(t, err)

	// now rewrite cached fingerpint to play out the mismatch scenario
	fakeFingerpint := []byte("fingerprint_mismatch")
	fingerprintPath := filepath.Join(tmpdir, ".cache/ma_fingerprint")
	err = ioutil.WriteFile(fingerprintPath, fakeFingerpint, fs.ModePerm)
	require.Nil(t, err)

	err = AgentPrepareStartup()
	require.NotNil(t, err)
}
