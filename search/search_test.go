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

// Package search provides the entry point for the search command.
//
// It implements a search command that searches for resources in AWS.
// The searches are done in parallel and the results are printed in the
// specified format.
package search

import (
	"fmt"
	"testing"

	"github.com/dyegoe/awss/common"
)

// mockEngines swaps the registry for a single "test" engine and restores it after the test.
func mockEngines(t *testing.T, newFn constructor) {
	t.Helper()
	old := engines
	t.Cleanup(func() { engines = old })
	engines = map[string]engine{
		"test": {
			new: newFn,
			sortFields: func(f string) (map[string]string, error) {
				fields := map[string]string{"field1": "value1"}
				if _, ok := fields[f]; !ok {
					return nil, fmt.Errorf("field %s not found", f)
				}
				return fields, nil
			},
		},
	}
}

// TestCheckSortField tests the CheckSortField function.
func TestCheckSortField(t *testing.T) {
	mockEngines(t, nil)

	type args struct {
		cmd string
		f   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Command found and field found",
			args:    args{cmd: "test", f: "field1"},
			wantErr: false,
		},
		{
			name:    "Command not found",
			args:    args{cmd: "test2", f: "field1"},
			wantErr: true,
		},
		{
			name:    "Command found but field not found",
			args:    args{cmd: "test", f: "field2"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckSortField(tt.args.cmd, tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("CheckSortField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestExecute_unknownCommand checks that an unknown command fails before any profile or region is touched.
func TestExecute_unknownCommand(t *testing.T) {
	called := false
	mockEngines(t, func(_, _ string, _ map[string][]string, _ Options) common.Results {
		called = true
		return nil
	})

	err := Execute("nope", []string{"default"}, []string{"us-east-1"}, map[string][]string{}, Options{Output: common.JSON})
	if err == nil {
		t.Fatal("Execute() error = nil, want command not found")
	}
	if called {
		t.Error("Execute() built results for an unknown command")
	}
}

// TestEngines_registeredCommands checks every built-in command has both a constructor and sort fields.
func TestEngines_registeredCommands(t *testing.T) {
	for _, cmd := range []string{"ec2", "eni", "ebs"} {
		eng, ok := engines[cmd]
		if !ok {
			t.Errorf("engines[%q] missing", cmd)
			continue
		}
		if eng.new == nil || eng.sortFields == nil {
			t.Errorf("engines[%q] must define both new and sortFields", cmd)
		}
		r := eng.new("default", "us-east-1", map[string][]string{}, Options{SortField: "id", NoInstanceName: true})
		if r == nil {
			t.Errorf("engines[%q].new returned nil", cmd)
			continue
		}
		if got := r.GetSortField(); got != "id" {
			t.Errorf("engines[%q].new did not pass SortField through, got %q", cmd, got)
		}
	}
}
