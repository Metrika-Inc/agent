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

package otc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

var mockResp = []byte(`02aab8a4-74ef-476e-8182-f6d2ba4166a6`)

func TestHostname(t *testing.T) {
	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(mockResp)
	})

	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	otc := NewSearch()
	otc.request, _ = http.NewRequest("GET", ts.URL, nil)
	got, err := otc.Hostname()
	require.Nil(t, err)

	require.Equal(t, "02aab8a4-74ef-476e-8182-f6d2ba4166a6", got)
}
