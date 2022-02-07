package utils

import "strings"

// AutoConfigError implements an error interface and lists
// all the issues encountered during automatic discovery and validation.
type AutoConfigError struct {
	errors []error
}

// Append adds an additional error to the error list.
func (a *AutoConfigError) Append(e error) {
	a.errors = append(a.errors, e)
}

// ErrIfAny returns an error if at least a single error is appended to the type.
func (a *AutoConfigError) ErrIfAny() error {
	if len(a.errors) > 0 {
		return a
	}
	return nil
}

func (a *AutoConfigError) Error() string {
	errText := "automatic discovery failed because: "
	for _, err := range a.errors {
		errText += err.Error() + "; "
	}
	return strings.TrimRight(errText, " ")
}
