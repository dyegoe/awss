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

package nesting

import (
	"reflect"
	"testing"
)

// TestCheck checks the depth counting on a fixture: the limit, one too deep, else blocks, and
// function literals.
func TestCheck(t *testing.T) {
	const fix = "use guard clauses or extract a function"
	want := []string{
		"deep/deep.go:19: four nests 4 levels deep, the maximum is 3: " + fix,
		"deep/deep.go:34: elseBlock nests 4 levels deep, the maximum is 3: " + fix,
		"deep/deep.go:55: function literal nests 4 levels deep, the maximum is 3: " + fix,
	}
	got, err := Check("testdata")
	if err != nil {
		t.Fatalf("Check(testdata) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Check(testdata) =\n%q\nwant\n%q", got, want)
	}
}

// TestCheck_repository enforces the nesting limit on the whole module.
func TestCheck_repository(t *testing.T) {
	got, err := Check("../..")
	if err != nil {
		t.Fatalf("Check(module) error = %v", err)
	}
	for _, v := range got {
		t.Errorf("nesting (AGENTS.md, Standards and guardrails): %s", v)
	}
}
