// Copyright 2015 The Prometheus Authors
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

package collector

// func TestTextfileCollector(t *testing.T) {
// 	tests := []struct {
// 		path string
// 		out  string
// 	}{
// 		// {
// 		// 	path: "fixtures/textfile/no_metric_files",
// 		// 	out:  "fixtures/textfile/no_metric_files.out",
// 		// },
// 		{
// 			path: "fixtures/textfile/two_metric_files",
// 			out:  "fixtures/textfile/two_metric_files.out",
// 		},
// 		{
// 			path: "fixtures/textfile/nonexistent_path",
// 			out:  "fixtures/textfile/nonexistent_path.out",
// 		},
// 		{
// 			path: "fixtures/textfile/client_side_timestamp",
// 			out:  "fixtures/textfile/client_side_timestamp.out",
// 		},
// 		{
// 			path: "fixtures/textfile/different_metric_types",
// 			out:  "fixtures/textfile/different_metric_types.out",
// 		},
// 		{
// 			path: "fixtures/textfile/inconsistent_metrics",
// 			out:  "fixtures/textfile/inconsistent_metrics.out",
// 		},
// 		{
// 			path: "fixtures/textfile/histogram",
// 			out:  "fixtures/textfile/histogram.out",
// 		},
// 		{
// 			path: "fixtures/textfile/histogram_extra_dimension",
// 			out:  "fixtures/textfile/histogram_extra_dimension.out",
// 		},
// 		{
// 			path: "fixtures/textfile/summary",
// 			out:  "fixtures/textfile/summary.out",
// 		},
// 		{
// 			path: "fixtures/textfile/summary_extra_dimension",
// 			out:  "fixtures/textfile/summary_extra_dimension.out",
// 		},
// 		{
// 			path: "fixtures/textfile/*_extra_dimension",
// 			out:  "fixtures/textfile/glob_extra_dimension.out",
// 		},
// 	}

// 	for i, test := range tests {
// 		mtime := 1.0
// 		c := &textFileCollector{
// 			path:  "pkg/collector/" + test.path,
// 			mtime: &mtime,
// 		}
// 		test.out = "pkg/collector/" + test.out

// 		registry := prometheus.NewRegistry()
// 		registry.MustRegister(c)

// 		rw := httptest.NewRecorder()
// 		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(rw, &http.Request{})
// 		got := string(rw.Body.String())

// 		want, err := ioutil.ReadFile(test.out)
// 		if err != nil {
// 			t.Fatalf("%d. error reading fixture file %s: %s", i, test.out, err)
// 		}

// 		if string(want) != got {
// 			t.Fatalf("%d.%q want:\n\n%s\n\ngot:\n\n%s", i, test.path, string(want), got)
// 		}
// 	}
// }
