package fingerprint

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	val := []byte("foobar")
	expHash := fmt.Sprintf("%x", sha256.Sum256(val))

	gotFp, err := New(ioutil.Discard, val)
	require.Nil(t, err)

	gotHash := gotFp.Hash()

	require.Equal(t, expHash, gotHash)
}

func TestNew_Write(t *testing.T) {
	val := []byte("foobar")
	expHash := fmt.Sprintf("%x", sha256.Sum256(val))

	tmpfile, err := ioutil.TempFile("", "host_fingerprint")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(tmpfile.Name())

	gotFp, err := New(tmpfile, val)
	require.Nil(t, err)

	err = gotFp.Write()
	require.Nil(t, err)

	gotHash := gotFp.Hash()
	require.Equal(t, expHash, gotHash)

	tmpfile.Seek(0, 0)
	gotHashOutBytes, err := ioutil.ReadAll(tmpfile)
	require.Nil(t, err)

	gotHashOut := string(gotHashOutBytes)
	require.Equal(t, expHash, gotHashOut)
}

func TestNewWithValidation_Bootstrap(t *testing.T) {
	val := []byte("foobar")
	expHash := fmt.Sprintf("%x", sha256.Sum256(val))
	fpfile, err := ioutil.TempFile("", "host_fingerprint")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(fpfile.Name())

	_, err = fpfile.Write([]byte(expHash))
	require.Nil(t, err)

	fpr, err := os.Open(fpfile.Name())
	require.Nil(t, err)
	defer fpr.Close()

	fpv, err := NewWithValidation(val, ioutil.Discard, fpr)
	require.Nil(t, err)

	gotHash := fpv.Hash()
	require.Equal(t, expHash, gotHash)
}

func TestNewWithValidation_Restart(t *testing.T) {
	val := []byte("foobar")
	expHash := fmt.Sprintf("%x", sha256.Sum256(val))

	fpfile, err := ioutil.TempFile("", "host_fingerprint")
	if err != nil {
		t.Error(err)
	}
	defer fpfile.Close()

	prevFpfile, err := os.OpenFile(fpfile.Name(), os.O_RDWR, 0o644)
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

	errfpv, err := NewWithValidation(val, fpw, fpr)
	require.Nil(t, err)

	require.Equal(t, expHash, errfpv.Hash())
}

func TestNewWithValidation_Regression(t *testing.T) {
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

	errfpv, err := NewWithValidation([]byte(""), ioutil.Discard, fpr)
	require.NotNil(t, err)

	require.Equal(t, "", errfpv.Hash())
}
