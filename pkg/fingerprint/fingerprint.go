package fingerprint

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
)

var (
	defaultSources = []source{CPUInfo{}, Hostname{}, MACAddress{}}
	zerofp         = Fingerprint{}
)

type ValidationError struct {
	err error
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("%v", v.err)
}

func validationError(err error) *ValidationError {
	return &ValidationError{err: err}
}

type source interface {
	name() string
	bytes() ([]byte, error)
}

type Fingerprint struct {
	out  io.Writer
	hash string
}

func (f Fingerprint) Hash() string {
	return f.hash
}

func (f Fingerprint) Write() error {
	n, err := f.out.Write([]byte(f.hash))
	if err != nil {
		return err
	}

	if n != len(f.hash) {
		err := fmt.Errorf("unexpected number of bytes written: %d/%d",
			n, len(f.hash))
		return err
	}

	return nil
}

func validate(newfp Fingerprint, oldfpr io.Reader) error {
	prevHashBytes, err := ioutil.ReadAll(oldfpr)
	if err != nil {
		return err
	}

	prevHash := string(prevHashBytes)
	if len(prevHash) > 1 && prevHash != newfp.hash {
		err := fmt.Errorf("hash mismatch detected, expected %s, got %s",
			prevHash, newfp.hash)

		return validationError(err)
	}

	return nil
}

// NewWithValidation creates a new fingerprint with writer next
// and validates it against previous fingerprint located under p.
func NewWithValidation(newfpw io.Writer, oldfpr io.Reader) (Fingerprint, error) {
	newfp, err := New(newfpw)
	if err != nil {
		return zerofp, err
	}

	if err := validate(newfp, oldfpr); err != nil {
		return zerofp, err
	}

	return newfp, nil
}

// New returns a Fingerprint that is initialized by computing a
// SHA256 hash from a list of default sources.
func New(out io.Writer) (Fingerprint, error) {
	h := sha256.New()
	for _, src := range defaultSources {
		srcBytes, err := src.bytes()
		if err != nil {
			return zerofp, err
		}

		n, err := h.Write(srcBytes)
		if err != nil {
			return zerofp, err
		}

		if n != len(srcBytes) {
			err := fmt.Errorf("unexpected number of bytes written: %d/%d",
				n, len(srcBytes))
			return zerofp, err
		}
	}

	hstr := fmt.Sprintf("%x", h.Sum(nil))
	if len(hstr) < sha256.BlockSize {
		err := fmt.Errorf("unexpected hash length, expected %d, got %d",
			sha256.BlockSize, len(hstr))
		return zerofp, err
	}

	return Fingerprint{out, hstr}, nil
}
