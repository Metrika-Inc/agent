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
