/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testconv

import (
	"reflect"
	"testing"
)

// TestCheck checks the checker on fixture packages: one that follows every convention and one
// that breaks each of them once.
func TestCheck(t *testing.T) {
	tests := []struct {
		name string
		root string
		want []string
	}{
		{name: "every convention followed", root: "testdata/good", want: nil},
		{
			name: "every convention broken", root: "testdata/bad",
			want: []string{
				"bad_test.go:12: TestResults_Collect: the scenario Collect starts lower-case " +
					"(or it is not a method of Results)",
				"bad_test.go:15: TestResults_collect_too_long: at most Test<Subject>_<Method>_<scenario>",
				"bad_test.go:18: TestResults_nope_badSortField: nope is not a method of Results",
				"bad_test.go:21: TestMain: TestMain is reserved for func TestMain(m *testing.M), add a _<scenario>",
				"bad_test.go:23: TestParseRow has no doc comment",
				`bad_test.go:24: "args" struct: use flat fields in the test table`,
				"bad_test.go:27: commented-out test: delete it",
				"bad_test.go:30: package-level var in a test file: return the fixture from a function",
				"bad_test.go:6: Test_parseRow: no Test_ prefix, use Test<Subject>",
				"bad_test.go:9: TestNothing: Nothing names no identifier of the package",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Check(tt.root)
			if err != nil {
				t.Fatalf("Check(%q) error = %v", tt.root, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Check(%q) =\n%q\nwant\n%q", tt.root, got, tt.want)
			}
		})
	}
}

// TestCheck_repository enforces the test conventions on the whole module.
func TestCheck_repository(t *testing.T) {
	got, err := Check("../..")
	if err != nil {
		t.Fatalf("Check(module) error = %v", err)
	}
	for _, v := range got {
		t.Errorf("test convention (AGENTS.md, Testing conventions): %s", v)
	}
}
