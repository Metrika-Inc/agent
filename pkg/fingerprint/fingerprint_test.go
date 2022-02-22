package fingerprint

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type MockHostname struct{}

func (m MockHostname) name() string {
	return "mock_hostname"
}

func (m MockHostname) bytes() ([]byte, error) {
	return []byte("foobar"), nil
}

type MockCPUInfo struct{}

func (m MockCPUInfo) name() string {
	return "mock_cpu_info"
}

func (m MockCPUInfo) bytes() ([]byte, error) {
	b, err := ioutil.ReadFile("./fixtures/mock_cpu_info.json")
	if err != nil {
		return nil, err
	}

	return b, nil
}

func overrideDefaultSources() func() {
	defaultSourcesWas := defaultSources
	defaultSources = []source{MockCPUInfo{}, MockHostname{}}
	return func() {
		defaultSources = defaultSourcesWas
	}
}

func TestNew(t *testing.T) {
	defer overrideDefaultSources()()

	gotFp, err := New(ioutil.Discard)
	require.Nil(t, err)

	gotHash := gotFp.Hash()
	expHash := "9da4115ce0f86d58f8cd1d66688ac6e644f72172e8a6ef4a026dca10d26e6617"

	require.Equal(t, expHash, gotHash)
}

func TestNew_Write(t *testing.T) {
	defer overrideDefaultSources()()

	tmpfile, err := ioutil.TempFile("", "host_fingerprint")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(tmpfile.Name())

	gotFp, err := New(tmpfile)
	require.Nil(t, err)

	err = gotFp.Write()
	require.Nil(t, err)

	gotHash := gotFp.Hash()
	expHash := "9da4115ce0f86d58f8cd1d66688ac6e644f72172e8a6ef4a026dca10d26e6617"
	require.Equal(t, expHash, gotHash)

	tmpfile.Seek(0, 0)
	gotHashOutBytes, err := ioutil.ReadAll(tmpfile)
	require.Nil(t, err)

	gotHashOut := string(gotHashOutBytes)
	require.Equal(t, expHash, gotHashOut)
}

func TestNewWithValidation_Bootstrap(t *testing.T) {
	defer overrideDefaultSources()()

	fpfile, err := ioutil.TempFile("", "host_fingerprint")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(fpfile.Name())

	expHash := "9da4115ce0f86d58f8cd1d66688ac6e644f72172e8a6ef4a026dca10d26e6617"
	_, err = fpfile.Write([]byte(expHash))
	require.Nil(t, err)

	fpr, err := os.Open(fpfile.Name())
	require.Nil(t, err)
	defer fpr.Close()

	fpv, err := NewWithValidation(ioutil.Discard, fpr)
	require.Nil(t, err)

	gotHash := fpv.Hash()
	require.Equal(t, expHash, gotHash)
}

func TestNewWithValidation_Restart(t *testing.T) {
	defer overrideDefaultSources()()

	fpfile, err := ioutil.TempFile("", "host_fingerprint")
	if err != nil {
		t.Error(err)
	}
	defer fpfile.Close()

	expHash := "9da4115ce0f86d58f8cd1d66688ac6e644f72172e8a6ef4a026dca10d26e6617"
	prevFpfile, err := os.OpenFile(fpfile.Name(), os.O_RDWR, 0644)
	require.Nil(t, err)
	defer prevFpfile.Close()

	_, err = prevFpfile.Write([]byte(expHash))
	require.Nil(t, err)
	err = prevFpfile.Close()
	require.Nil(t, err)

	fpr, err := os.Open(fpfile.Name())
	require.Nil(t, err)
	defer fpr.Close()

	fpw, err := os.Open(fpfile.Name())
	require.Nil(t, err)
	defer fpw.Close()

	errfpv, err := NewWithValidation(fpw, fpr)
	require.Nil(t, err)

	require.Equal(t, expHash, errfpv.Hash())
}

func TestNewWithValidation_Regression(t *testing.T) {
	defer overrideDefaultSources()()

	oldfpf, err := ioutil.TempFile("", "host_fingerprint")
	require.Nil(t, err)

	oldhash := []byte("foobar")
	_, err = oldfpf.Write(oldhash)
	require.Nil(t, err)

	t.Log(oldfpf.Name())
	oldfpf.Close()

	fpr, err := os.Open(oldfpf.Name())
	require.Nil(t, err)
	defer fpr.Close()

	errfpv, err := NewWithValidation(ioutil.Discard, fpr)
	require.NotNil(t, err)

	require.Equal(t, "", errfpv.Hash())
}
