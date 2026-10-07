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
	"strings"
	"testing"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// TestValidOutputs tests the ValidOutputs function.
func TestValidOutputs(t *testing.T) {
	tests := []struct {
		name  string
		o     string
		want  string
		want1 bool
	}{
		{
			name:  "Valid output",
			o:     "json",
			want:  "json, json-pretty, table",
			want1: true,
		},
		{
			name:  "Invalid output",
			o:     "invalid",
			want:  "json, json-pretty, table",
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ValidOutputs(tt.o)
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
	outputs = map[string]func(Results, bool, bool, []string) string{
		JSON:       func(_ Results, _, _ bool, _ []string) string { return "json" },
		JSONPretty: func(_ Results, _, _ bool, _ []string) string { return "json-pretty" },
		Table:      func(_ Results, _, _ bool, _ []string) string { return "table" },
	}

	tests := []struct {
		name    string
		results Results
		output  string
		want    string
	}{
		{
			name:    "JSON output",
			results: mockResults(),
			output:  JSON,
			want:    "json\n",
		},
		{
			name:    "JSONPretty output",
			results: mockResults(),
			output:  JSONPretty,
			want:    "json-pretty\n",
		},
		{
			name:    "Table output",
			results: mockResults(),
			output:  Table,
			want:    "table\n",
		},
		{
			name:    "Invalid output",
			results: mockResults(),
			output:  "invalid",
			want:    "Invalid output format: invalid\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultsChan := make(chan Results, 1)
			done := make(chan bool)
			resultsChan <- tt.results
			close(resultsChan)
			go func() {
				<-done
				close(done)
			}()
			w := bytes.Buffer{}
			PrintResults(&w, resultsChan, done, tt.output, false, false, nil)
			if got := w.String(); got != tt.want {
				t.Errorf("PrintResults()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestToBold is a test function for toBold.
func TestToBold(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "empty",
			s:    "",
			want: "",
		},
		{
			name: "string",
			s:    "string",
			want: "\033[1mstring\033[0m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toBold(tt.s); got != tt.want {
				t.Errorf("toBold()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestToJSON is a test function for toJSON.
//
//nolint:dupl // same table shape as TestToJSONPretty, for the other JSON printer
func TestToJSON(t *testing.T) {
	tests := []struct {
		name      string
		r         Results
		showEmpty bool
		showTags  bool
		want      string
	}{
		{
			name:      "empty json showEmpty false",
			r:         mockResultsEmpty(),
			showEmpty: false,
			showTags:  false,
			want:      "",
		},
		{
			name:      "empty json",
			r:         mockResultsEmpty(),
			showEmpty: true,
			showTags:  false,
			want:      jsonEmptyNoPretty,
		},
		{
			name:      "json",
			r:         mockResults(),
			showEmpty: false,
			showTags:  false,
			want:      jsonNoPretty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSON(tt.r, tt.showEmpty, tt.showTags, nil)
			if got != tt.want {
				t.Errorf("toJSON()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestToJSONPretty is a test function for toJSONPretty.
//
//nolint:dupl // same table shape as TestToJSON, for the other JSON printer
func TestToJSONPretty(t *testing.T) {
	tests := []struct {
		name      string
		r         Results
		showEmpty bool
		showTags  bool
		want      string
	}{
		{
			name:      "empty json showEmpty false",
			r:         mockResultsEmpty(),
			showEmpty: false,
			showTags:  false,
			want:      "",
		},
		{
			name:      "empty json",
			r:         mockResultsEmpty(),
			showEmpty: true,
			showTags:  false,
			want:      jsonEmptyPretty,
		},
		{
			name:      "json",
			r:         mockResults(),
			showEmpty: false,
			showTags:  false,
			want:      jsonPretty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSONPretty(tt.r, tt.showEmpty, tt.showTags, nil)
			if got != tt.want {
				t.Errorf("toJSON()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestToTable is a test function for toTable.
func TestToTable(t *testing.T) {
	// save the original function, defer the restore and mock the function
	oldBold := Bold
	defer func() { Bold = oldBold }()
	Bold = func(s string) string { return s }

	tests := []struct {
		name      string
		r         Results
		showEmpty bool
		showTags  bool
		tagsKeys  []string
		want      string
	}{
		{
			name: "table with no tags",
			r:    mockResults(),
			want: tableNoTags,
		},
		{
			name:     "table with tags",
			r:        mockResults(),
			showTags: true,
			want:     tableTags,
		},
		{
			name:     "table with tags filtered by key",
			r:        mockResults(),
			showTags: true,
			tagsKeys: []string{"key1", "key3"},
			want:     tableTagsFiltered,
		},
		{
			name:      "empty table with no tags",
			r:         mockResultsEmpty(),
			showEmpty: true,
			want:      tableEmptyNoTags,
		},
		{
			name:      "empty table with tags",
			r:         mockResultsEmpty(),
			showEmpty: true,
			showTags:  true,
			want:      tableEmptyTags,
		},
		{
			name: "empty table showEmpty false",
			r:    mockResultsEmpty(),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toTable(tt.r, tt.showEmpty, tt.showTags, tt.tagsKeys)
			if got != tt.want {
				t.Errorf("toTable()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestRowFromStruct is a test function for rowFromStruct.
func TestRowFromStruct(t *testing.T) {
	tests := []struct {
		name     string
		i        interface{}
		tagsKeys []string
		want     table.Row
	}{
		{
			name: "empty",
			i:    struct{}{},
			want: table.Row{},
		},
		{
			name: "test struct",
			i:    mockDataRow1(),
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
		{
			name:     "test struct with tagsKeys filter",
			i:        mockDataRow1(),
			tagsKeys: []string{"key1"},
			want: table.Row{
				fmt.Sprintf("%s: %s\n%s: %s",
					text.Bold.Sprint("Info String1"),
					"testInfo1String1",
					text.Bold.Sprint("Info String2"),
					"testInfo1String2"),
				fmt.Sprintf("%s: %s", text.Bold.Sprint("key1"), "value1"),
				"sliceValue1\nsliceValue2",
				"testString1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rowFromStruct(tt.i, tt.tagsKeys); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rowFromStruct()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestHeaderStructFieldsToString is a test function for headerStructFieldsToString.
func TestHeaderStructFieldsToString(t *testing.T) {
	tests := []struct {
		name string
		i    interface{}
		want string
	}{
		{
			name: "empty",
			i:    struct{}{},
			want: "",
		},
		{
			name: "testInfo struct 1",
			i:    mockInfo1(),
			want: fmt.Sprintf("%s: %s\n%s: %s",
				text.Bold.Sprint("Info String1"),
				"testInfo1String1",
				text.Bold.Sprint("Info String2"),
				"testInfo1String2"),
		},
		{
			name: "testInfo struct 2",
			i:    mockInfo2(),
			want: fmt.Sprintf("%s: %s\n%s: %s",
				text.Bold.Sprint("Info String1"),
				"testInfo2String1",
				text.Bold.Sprint("Info String2"),
				"testInfo2String2"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headerStructFieldsToString(tt.i); got != tt.want {
				t.Errorf("headerStructFieldsToString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestSortedStringMapToString is a test function for sortedStringMapToString.
func TestSortedStringMapToString(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		keys []string
		want string
	}{
		{
			name: "empty",
			m:    map[string]string{},
			want: "",
		},
		{
			name: "one",
			m:    map[string]string{"one": "1"},
			want: fmt.Sprintf("%s: 1", text.Bold.Sprint("one")),
		},
		{
			name: "two",
			m:    map[string]string{"one": "1", "two": "2"},
			want: fmt.Sprintf("%s: 1\n%s: 2", text.Bold.Sprint("one"), text.Bold.Sprint("two")),
		},
		{
			name: "three",
			m:    map[string]string{"one": "1", "two": "2", "three": "3"},
			want: fmt.Sprintf("%s: 1\n%s: 3\n%s: 2",
				text.Bold.Sprint("one"),
				text.Bold.Sprint("three"),
				text.Bold.Sprint("two")),
		},
		{
			name: "filtered by keys",
			m:    map[string]string{"one": "1", "two": "2", "three": "3"},
			keys: []string{"one", "two"},
			want: fmt.Sprintf("%s: 1\n%s: 2", text.Bold.Sprint("one"), text.Bold.Sprint("two")),
		},
		{
			name: "filtered by keys, no match",
			m:    map[string]string{"one": "1"},
			keys: []string{"missing"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedStringMapToString(tt.m, tt.keys); got != tt.want {
				t.Errorf("sortedStringMapToString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestSortedStringSliceToString is a test function for sortedStringSliceToString.
func TestSortedStringSliceToString(t *testing.T) {
	tests := []struct {
		name string
		s    []string
		want string
	}{
		{
			name: "empty",
			s:    []string{},
			want: "",
		},
		{
			name: "one",
			s:    []string{"one"},
			want: "one",
		},
		{
			name: "two",
			s:    []string{"one", "two"},
			want: "one\ntwo",
		},
		{
			name: "three",
			s:    []string{"one", "two", "three"},
			want: "one\nthree\ntwo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedStringSliceToString(tt.s); got != tt.want {
				t.Errorf("sortedStringSliceToString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestOutputs_failedEmptyResults checks that every registered output prints an empty result set
// that carries errors, even when showEmpty is false, so failures stay visible.
func TestOutputs_failedEmptyResults(t *testing.T) {
	failed := &testResults{
		Profile: "bad",
		Region:  "r1",
		Errors:  []string{"api error InvalidClientTokenId"},
	}
	for name, printer := range outputs {
		t.Run(name, func(t *testing.T) {
			got := printer(failed, false, false, nil)
			for _, want := range []string{"bad", "r1", "InvalidClientTokenId"} {
				if !strings.Contains(got, want) {
					t.Errorf("%s output = %q, want it to contain %q", name, got, want)
				}
			}
		})
	}
}
