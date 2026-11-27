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

package doclist

import (
	"reflect"
	"testing"
)

// TestCheck checks the package directories found in a fixture (no root, test-only, hidden or
// testdata directory) and how a doc lists them: "a/sub/" does not list "a/", and "a/ first" does.
func TestCheck(t *testing.T) {
	tests := []struct {
		name string
		docs []string
		want []string
	}{
		{name: "a missing", docs: []string{"doc.md"}, want: []string{"doc.md: missing a/"}},
		{name: "all listed", docs: []string{"full.md"}, want: nil},
		{name: "no docs", docs: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Check("testdata/repo", tt.docs...)
			if err != nil {
				t.Fatalf("Check(testdata/repo, %v) error = %v", tt.docs, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Check(testdata/repo, %v) = %q, want %q", tt.docs, got, tt.want)
			}
		})
	}
}

// TestCheck_errors checks a missing doc and a missing root are errors.
func TestCheck_errors(t *testing.T) {
	tests := []struct {
		name string
		root string
		docs []string
	}{
		{name: "missing doc", root: "testdata/repo", docs: []string{"nope.md"}},
		{name: "missing root", root: "testdata/nope", docs: []string{"doc.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Check(tt.root, tt.docs...); err == nil {
				t.Errorf("Check(%s, %v) error = nil, want an error", tt.root, tt.docs)
			}
		})
	}
}

// TestCheck_repository checks AGENTS.md and docs/ARCHITECTURE.md list every package of the module.
func TestCheck_repository(t *testing.T) {
	got, err := Check("../..", "AGENTS.md", "docs/ARCHITECTURE.md")
	if err != nil {
		t.Fatalf("Check(module) error = %v", err)
	}
	for _, v := range got {
		t.Errorf("package list (AGENTS.md, Standards and guardrails): %s", v)
	}
}
