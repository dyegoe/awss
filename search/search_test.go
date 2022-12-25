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
)

// // TestExecute tests the Execute function.
// func TestExecute(t *testing.T) {
// 	type args struct {
// 		cmd       string
// 		profiles  []string
// 		regions   []string
// 		filters   map[string][]string
// 		sortField string
// 		output    string
// 		showEmpty bool
// 		showTags  bool
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			err := Execute(
// 				tt.args.cmd,
// 				tt.args.profiles,
// 				tt.args.regions,
// 				tt.args.filters,
// 				tt.args.sortField,
// 				tt.args.output,
// 				tt.args.showEmpty,
// 				tt.args.showTags,
// 			)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// TestCheckSortField tests the checkSortField function.
func TestCheckSortField(t *testing.T) {
	// save the original function, defer the restore and mock the function
	oldGetSortFieldsCMDList := getSortFieldsCMDList
	defer func() { getSortFieldsCMDList = oldGetSortFieldsCMDList }()
	getSortFieldsCMDList = map[string]func(string) (map[string]string, error){
		"test": func(f string) (map[string]string, error) {
			fields := map[string]string{"field1": "value1"}
			if _, ok := fields[f]; !ok {
				return nil, fmt.Errorf("field %s not found", f)
			}
			return fields, nil
		},
	}

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
