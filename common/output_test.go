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

// Package common contains common functions and types.
//
// It has AWS related functions and types.
// It also has functions to print the results in different formats.
package common

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// TestValidOutputs tests the ValidOutputs function.
func TestValidOutputs(t *testing.T) {
	type args struct {
		o string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{
			name: "Valid output",
			args: args{
				o: "json",
			},
			want:  "json, json-pretty, table",
			want1: true,
		},
		{
			name: "Invalid output",
			args: args{
				o: "invalid",
			},
			want:  "json, json-pretty, table",
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ValidOutputs(tt.args.o)
			if got != tt.want {
				t.Errorf("ValidOutputs() got\n%#v\nwant\n%#v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ValidOutputs() got1\n%#v\nwant\n%#v", got1, tt.want1)
			}
		})
	}
}

// TestPrintResults is a test function for PrintResults.
func TestPrintResults(t *testing.T) {
	// save the original function map, defer the restore and mock the function map
	originalOutputs := outputs
	defer func() { outputs = originalOutputs }()
	outputs = map[string]func(Results, bool, bool) string{
		JSON:       func(r Results, b1, b2 bool) string { return "json" },
		JSONPretty: func(r Results, b1, b2 bool) string { return "json-pretty" },
		Table:      func(r Results, b1, b2 bool) string { return "table" },
	}

	type args struct {
		results   Results
		output    string
		showEmpty bool
		showTags  bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "JSON output",
			args: args{results: &tr, output: JSON, showEmpty: false, showTags: false},
			want: "json\n",
		},
		{
			name: "JSONPretty output",
			args: args{results: &tr, output: JSONPretty, showEmpty: false, showTags: false},
			want: "json-pretty\n",
		},
		{
			name: "Table output",
			args: args{results: &tr, output: Table, showEmpty: false, showTags: false},
			want: "table\n",
		},
		{
			name: "Invalid output",
			args: args{results: &tr, output: "invalid", showEmpty: false, showTags: false},
			want: "Invalid output format: invalid\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultsChan := make(chan Results, 1)
			done := make(chan bool)
			resultsChan <- tt.args.results
			close(resultsChan)
			go func() {
				<-done
				close(done)
			}()
			w := bytes.Buffer{}
			PrintResults(&w, resultsChan, done, tt.args.output, tt.args.showEmpty, tt.args.showTags)
			if got := w.String(); got != tt.want {
				t.Errorf("PrintResults()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_toBold is a test function for toBold.
func Test_toBold(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{s: ""},
			want: "",
		},
		{
			name: "string",
			args: args{s: "string"},
			want: "\033[1mstring\033[0m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toBold(tt.args.s); got != tt.want {
				t.Errorf("toBold()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_toJSON is a test function for toJSON.
//
//nolint:dupl
func Test_toJSON(t *testing.T) {
	type args struct {
		r         Results
		showEmpty bool
		showTags  bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty json showEmpty false",
			args: args{r: &trEmpty, showEmpty: false, showTags: false},
			want: "",
		},
		{
			name: "empty json",
			args: args{r: &trEmpty, showEmpty: true, showTags: false},
			want: jsonEmptyNoPretty,
		},
		{
			name: "json",
			args: args{r: &tr, showEmpty: false, showTags: false},
			want: jsonNoPretty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSON(tt.args.r, tt.args.showEmpty, tt.args.showTags)
			if got != tt.want {
				t.Errorf("toJSON()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_toJSONPretty is a test function for toJSONPretty.
//
//nolint:dupl
func Test_toJSONPretty(t *testing.T) {
	type args struct {
		r         Results
		showEmpty bool
		showTags  bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty json showEmpty false",
			args: args{r: &trEmpty, showEmpty: false, showTags: false},
			want: "",
		},
		{
			name: "empty json",
			args: args{r: &trEmpty, showEmpty: true, showTags: false},
			want: jsonEmptyPretty,
		},
		{
			name: "json",
			args: args{r: &tr, showEmpty: false, showTags: false},
			want: jsonPretty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSONPretty(tt.args.r, tt.args.showEmpty, tt.args.showTags)
			if got != tt.want {
				t.Errorf("toJSON()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_toTable is a test function for toTable.
func Test_toTable(t *testing.T) {
	// save the original function, defer the restore and mock the function
	oldBold := Bold
	defer func() { Bold = oldBold }()
	Bold = func(s string) string { return s }

	type args struct {
		r         Results
		showEmpty bool
		showTags  bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "table with no tags",
			args: args{r: &tr, showEmpty: false, showTags: false},
			want: tableNoTags,
		},
		{
			name: "table with tags",
			args: args{r: &tr, showEmpty: false, showTags: true},
			want: tableTags,
		},
		{
			name: "empty table with no tags",
			args: args{r: &trEmpty, showEmpty: true, showTags: false},
			want: tableEmptyNoTags,
		},
		{
			name: "empty table with tags",
			args: args{r: &trEmpty, showEmpty: true, showTags: true},
			want: tableEmptyTags,
		},
		{
			name: "empty table showEmpty false",
			args: args{r: &trEmpty, showEmpty: false, showTags: false},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toTable(tt.args.r, tt.args.showEmpty, tt.args.showTags)
			if got != tt.want {
				t.Errorf("toTable()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_rowFromStruct is a test function for rowFromStruct.
func Test_rowFromStruct(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name string
		args args
		want table.Row
	}{
		{
			name: "empty",
			args: args{i: struct{}{}},
			want: table.Row{},
		},
		{
			name: "test struct",
			args: args{i: tdr1},
			want: table.Row{
				fmt.Sprintf("%s: %s\n%s: %s",
					text.Bold.Sprint("Info String1"),
					"testInfo1String1",
					text.Bold.Sprint("Info String2"),
					"testInfo1String2"),
				fmt.Sprintf("%s: %s\n%s: %s",
					text.Bold.Sprint("key1"),
					"value1",
					text.Bold.Sprint("key2"),
					"value2"),
				"sliceValue1\nsliceValue2",
				"testString1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rowFromStruct(tt.args.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rowFromStruct()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_headerStructFieldsToString is a test function for headerStructFieldsToString.
func Test_headerStructFieldsToString(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{i: struct{}{}},
			want: "",
		},
		{
			name: "testInfo struct 1",
			args: args{i: ti1},
			want: fmt.Sprintf("%s: %s\n%s: %s",
				text.Bold.Sprint("Info String1"),
				"testInfo1String1",
				text.Bold.Sprint("Info String2"),
				"testInfo1String2"),
		},
		{
			name: "testInfo struct 2",
			args: args{i: ti2},
			want: fmt.Sprintf("%s: %s\n%s: %s",
				text.Bold.Sprint("Info String1"),
				"testInfo2String1",
				text.Bold.Sprint("Info String2"),
				"testInfo2String2"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headerStructFieldsToString(tt.args.i); got != tt.want {
				t.Errorf("headerStructFieldsToString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_sortedStringMapToString is a test function for sortedStringMapToString.
func Test_sortedStringMapToString(t *testing.T) {
	type args struct {
		m map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{m: map[string]string{}},
			want: "",
		},
		{
			name: "one",
			args: args{m: map[string]string{"one": "1"}},
			want: fmt.Sprintf("%s: 1", text.Bold.Sprint("one")),
		},
		{
			name: "two",
			args: args{m: map[string]string{"one": "1", "two": "2"}},
			want: fmt.Sprintf("%s: 1\n%s: 2", text.Bold.Sprint("one"), text.Bold.Sprint("two")),
		},
		{
			name: "three",
			args: args{m: map[string]string{"one": "1", "two": "2", "three": "3"}},
			want: fmt.Sprintf("%s: 1\n%s: 3\n%s: 2",
				text.Bold.Sprint("one"),
				text.Bold.Sprint("three"),
				text.Bold.Sprint("two")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedStringMapToString(tt.args.m); got != tt.want {
				t.Errorf("sortedStringMapToString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// Test_sortedStringSliceToString is a test function for sortedStringSliceToString.
func Test_sortedStringSliceToString(t *testing.T) {
	type args struct {
		s []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{s: []string{}},
			want: "",
		},
		{
			name: "one",
			args: args{s: []string{"one"}},
			want: "one",
		},
		{
			name: "two",
			args: args{s: []string{"one", "two"}},
			want: "one\ntwo",
		},
		{
			name: "three",
			args: args{s: []string{"one", "two", "three"}},
			want: "one\nthree\ntwo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedStringSliceToString(tt.args.s); got != tt.want {
				t.Errorf("sortedStringSliceToString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}
