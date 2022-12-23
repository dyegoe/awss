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
				t.Errorf("ValidOutputs() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ValidOutputs() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

// testResults is a struct used for testing.
//
// It will implement Results interface.
type testResults struct {
	Profile string        `json:"profile"`
	Region  string        `json:"region"`
	Errors  []string      `json:"errors,omitempty"`
	Data    []testDataRow `json:"data"`
}

// Results interface is implemented by testResults.
// Using the functions below, testResults will be able to be printed in different formats.

func (tr *testResults) Search()              {}
func (tr *testResults) Len() int             { return len(tr.Data) }
func (tr *testResults) GetProfile() string   { return tr.Profile }
func (tr *testResults) GetRegion() string    { return tr.Region }
func (tr *testResults) GetErrors() []string  { return tr.Errors }
func (tr *testResults) GetSortField() string { return "field" }
func (tr *testResults) GetHeaders() []interface{} {
	headers := []interface{}{}

	v := reflect.ValueOf(testDataRow{})
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)

		if header, ok := field.Tag.Lookup("header"); ok {
			headers = append(headers, header)
		}
	}

	return headers
}
func (tr *testResults) GetRows() []interface{} {
	rows := []interface{}{}

	for _, row := range tr.Data {
		rows = append(rows, row)
	}
	return rows
}

// testDataRow is a struct used for testing.
//
// It represents a row of the testResults.
type testDataRow struct {
	StructField testInfo          `json:"struct_field" header:"Struct Field"`
	MapField    map[string]string `json:"map_field" header:"Tags"` // header is `Tags`` because there is a test case for `--show-tags` on toTable().
	SliceField  []string          `json:"slice_field" header:"Slice Field"`
	StringField string            `json:"string_field" header:"String Field"`
}

// testInfo is a struct used for testing.
//
// It represents a field of the testDataRow.
type testInfo struct {
	InfoString1 string `json:"info_string1" header:"Info String1"`
	InfoString2 string `json:"info_string2" header:"Info String2"`
}

// tr is a testResults used for testing.
//
//	json:"profile" = testProfile
//	json:"region"  = testRegion
//	json:"errors"  = []string{"testError1", "testError2"}
//	json:"data"    = []testDataRow{tdr1, tdr2}
var tr = testResults{
	Profile: "testProfile",
	Region:  "testRegion",
	Errors:  []string{"testError1", "testError2"},
	Data: []testDataRow{
		tdr1,
		tdr2,
	},
}

// trEmpty is a testResults used for testing.
//
//	json:"profile" = testProfileEmpty
//	json:"region"  = testRegionEmpty
//	json:"errors"  = []string{}
//	json:"data"    = []testDataRow{}
var trEmpty = testResults{
	Profile: "testProfileEmpty",
	Region:  "testRegionEmpty",
	Errors:  []string{},
	Data:    []testDataRow{},
}

// tdr1 is a testDataRow used for testing.
//
//	json:"struct_field" header:"Struct Field"
//	  json:"info_string1" header:"Info String1" = testInfo1String1
//	  json:"info_string2" header:"Info String2" = testInfo1String2
//	json:"map_field"    header:"Map Field" = map[string]string{"key1": "value1", "key2": "value2"}
//	json:"slice_field"  header:"Slice Field" = []string{"sliceValue1", "sliceValue2"}
//	json:"string_field" header:"String Field" = testString1
var tdr1 = testDataRow{
	StructField: ti1,
	MapField: map[string]string{
		"key1": "value1",
		"key2": "value2",
	},
	SliceField:  []string{"sliceValue1", "sliceValue2"},
	StringField: "testString1",
}

// tdr2 is a testDataRow used for testing.
//
//	json:"struct_field" header:"Struct Field"
//	  json:"info_string1" header:"Info String1" = testInfo2String1
//	  json:"info_string2" header:"Info String2" = testInfo2String2
//	json:"map_field" header:"Map Field" = map[string]string{"key3": "value3", "key4": "value4"}
//	json:"slice_field" header:"Slice Field" = []string{"sliceValue3", "sliceValue4"}
//	json:"string_field" header:"String Field" = testString2
var tdr2 = testDataRow{
	StructField: ti2,
	MapField: map[string]string{
		"key3": "value3",
		"key4": "value4",
	},
	SliceField:  []string{"sliceValue3", "sliceValue4"},
	StringField: "testString2",
}

// ti1
//
//	json:"info_string1" header:"Info String1" = testInfo1String1
//	json:"info_string2" header:"Info String2" = testInfo1String2
var ti1 = testInfo{
	InfoString1: "testInfo1String1",
	InfoString2: "testInfo1String2",
}

// ti2
//
//	json:"info_string1" header:"Info String1" = testInfo2String1
//	json:"info_string2" header:"Info String2" = testInfo2String2
var ti2 = testInfo{
	InfoString1: "testInfo2String1",
	InfoString2: "testInfo2String2",
}

// TestPrintResults is a test function for PrintResults.
func TestPrintResults(t *testing.T) {
	// save the original Bold function
	originalOutputs := outputs
	// restore the original Bold function
	defer func() { outputs = originalOutputs }()
	// set Bold to a function that returns the input string
	outputs = map[string]func(Results, bool, bool) (string, error){
		JSON:       func(r Results, b1, b2 bool) (string, error) { return "json", nil },
		JSONPretty: func(r Results, b1, b2 bool) (string, error) { return "json-pretty", nil },
		Table:      func(r Results, b1, b2 bool) (string, error) { return "table", nil },
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
			args: args{
				results:   &tr,
				output:    JSON,
				showEmpty: false,
				showTags:  false,
			},
			want: "json\n",
		},
		{
			name: "JSONPretty output",
			args: args{
				results:   &tr,
				output:    JSONPretty,
				showEmpty: false,
				showTags:  false,
			},
			want: "json-pretty\n",
		},
		{
			name: "Table output",
			args: args{
				results:   &tr,
				output:    Table,
				showEmpty: false,
				showTags:  false,
			},
			want: "table\n",
		},
		{
			name: "Invalid output",
			args: args{
				results:   &tr,
				output:    "invalid",
				showEmpty: false,
				showTags:  false,
			},
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

			var w bytes.Buffer

			PrintResults(&w, resultsChan, done, tt.args.output, tt.args.showEmpty, tt.args.showTags)
			if got := w.String(); got != tt.want {
				t.Errorf("PrintResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

// jsonEmptyNoPretty is a json string used for testing.
var jsonEmptyNoPretty = `{"profile":"testProfileEmpty","region":"testRegionEmpty","data":[]}`

// jsonNoPretty is a json string used for testing.
var jsonNoPretty = `{"profile":"testProfile","region":"testRegion","errors":["testError1","testError2"],"data":[{"struct_field":{"info_string1":"testInfo1String1","info_string2":"testInfo1String2"},"map_field":{"key1":"value1","key2":"value2"},"slice_field":["sliceValue1","sliceValue2"],"string_field":"testString1"},{"struct_field":{"info_string1":"testInfo2String1","info_string2":"testInfo2String2"},"map_field":{"key3":"value3","key4":"value4"},"slice_field":["sliceValue3","sliceValue4"],"string_field":"testString2"}]}`

// Test_toJSON is a test function for toJSON.
func Test_toJSON(t *testing.T) {
	type args struct {
		r         Results
		showEmpty bool
		showTags  bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "empty json showEmpty false",
			args:    args{r: &trEmpty, showEmpty: false, showTags: false},
			want:    "",
			wantErr: false,
		},
		{
			name:    "empty json",
			args:    args{r: &trEmpty, showEmpty: true, showTags: false},
			want:    jsonEmptyNoPretty,
			wantErr: false,
		},
		{
			name:    "json",
			args:    args{r: &tr, showEmpty: false, showTags: false},
			want:    jsonNoPretty,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toJSON(tt.args.r, tt.args.showEmpty, tt.args.showTags)
			if (err != nil) != tt.wantErr {
				t.Errorf("toJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("toJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

// jsonEmptyPretty is a json string used for testing.
var jsonEmptyPretty = `{
  "profile": "testProfileEmpty",
  "region": "testRegionEmpty",
  "data": []
}`

// jsonPretty is a json string used for testing.
var jsonPretty = `{
  "profile": "testProfile",
  "region": "testRegion",
  "errors": [
    "testError1",
    "testError2"
  ],
  "data": [
    {
      "struct_field": {
        "info_string1": "testInfo1String1",
        "info_string2": "testInfo1String2"
      },
      "map_field": {
        "key1": "value1",
        "key2": "value2"
      },
      "slice_field": [
        "sliceValue1",
        "sliceValue2"
      ],
      "string_field": "testString1"
    },
    {
      "struct_field": {
        "info_string1": "testInfo2String1",
        "info_string2": "testInfo2String2"
      },
      "map_field": {
        "key3": "value3",
        "key4": "value4"
      },
      "slice_field": [
        "sliceValue3",
        "sliceValue4"
      ],
      "string_field": "testString2"
    }
  ]
}`

// Test_toJSON is a test function for toJSON.
func Test_toJSONPretty(t *testing.T) {
	type args struct {
		r         Results
		showEmpty bool
		showTags  bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "empty json showEmpty false",
			args:    args{r: &trEmpty, showEmpty: false, showTags: false},
			want:    "",
			wantErr: false,
		},
		{
			name:    "empty json",
			args:    args{r: &trEmpty, showEmpty: true, showTags: false},
			want:    jsonEmptyPretty,
			wantErr: false,
		},
		{
			name:    "json",
			args:    args{r: &tr, showEmpty: false, showTags: false},
			want:    jsonPretty,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toJSONPretty(tt.args.r, tt.args.showEmpty, tt.args.showTags)
			if (err != nil) != tt.wantErr {
				t.Errorf("toJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("toJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

// tableNoTags is a test table output from tr.
var tableNoTags = `+-------------------------------------------------------------+
| [Profile] testProfile [Region] testRegion [Sort] field      |
|                                                             |
| testError1                                                  |
| testError2                                                  |
+--------------------------------+-------------+--------------+
| Struct Field                   | Slice Field | String Field |
+--------------------------------+-------------+--------------+
| Info String1: testInfo1String1 | sliceValue1 | testString1  |
| Info String2: testInfo1String2 | sliceValue2 |              |
+--------------------------------+-------------+--------------+
| Info String1: testInfo2String1 | sliceValue3 | testString2  |
| Info String2: testInfo2String2 | sliceValue4 |              |
+--------------------------------+-------------+--------------+
`

// tableTags is a test table output from tr with tags.
var tableTags = `+----------------------------------------------------------------------------+
| [Profile] testProfile [Region] testRegion [Sort] field                     |
|                                                                            |
| testError1                                                                 |
| testError2                                                                 |
+--------------------------------+--------------+-------------+--------------+
| Struct Field                   | Tags         | Slice Field | String Field |
+--------------------------------+--------------+-------------+--------------+
| Info String1: testInfo1String1 | key1: value1 | sliceValue1 | testString1  |
| Info String2: testInfo1String2 | key2: value2 | sliceValue2 |              |
+--------------------------------+--------------+-------------+--------------+
| Info String1: testInfo2String1 | key3: value3 | sliceValue3 | testString2  |
| Info String2: testInfo2String2 | key4: value4 | sliceValue4 |              |
+--------------------------------+--------------+-------------+--------------+
`

// tableEmptyNoTags is a test table output from trEmpty.
var tableEmptyNoTags = `+-------------------------------------------+
| [Profile] testProfileEmpty [Region] testR |
| egionEmpty [Sort] field                   |
+--------------+-------------+--------------+
| Struct Field | Slice Field | String Field |
+--------------+-------------+--------------+
+--------------+-------------+--------------+
`

// tableEmptyTags is a test table output from trEmpty with tags.
var tableEmptyTags = `+--------------------------------------------------+
| [Profile] testProfileEmpty [Region] testRegionEm |
| pty [Sort] field                                 |
+--------------+------+-------------+--------------+
| Struct Field | Tags | Slice Field | String Field |
+--------------+------+-------------+--------------+
+--------------+------+-------------+--------------+
`

// Test_toTable is a test function for toTable.
func Test_toTable(t *testing.T) {
	// save the original Bold function
	oldBold := Bold
	// restore the original Bold function
	defer func() { Bold = oldBold }()
	// set Bold to a function that returns the input string
	Bold = func(s string) string { return s }

	type args struct {
		r         Results
		showEmpty bool
		showTags  bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "table with no tags",
			args:    args{r: &tr, showEmpty: false, showTags: false},
			want:    tableNoTags,
			wantErr: false,
		},
		{
			name:    "table with tags",
			args:    args{r: &tr, showEmpty: false, showTags: true},
			want:    tableTags,
			wantErr: false,
		},
		{
			name:    "empty table with no tags",
			args:    args{r: &trEmpty, showEmpty: true, showTags: false},
			want:    tableEmptyNoTags,
			wantErr: false,
		},
		{
			name:    "empty table with tags",
			args:    args{r: &trEmpty, showEmpty: true, showTags: true},
			want:    tableEmptyTags,
			wantErr: false,
		},
		{
			name:    "empty table showEmpty false",
			args:    args{r: &trEmpty, showEmpty: false, showTags: false},
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toTable(tt.args.r, tt.args.showEmpty, tt.args.showTags)
			if (err != nil) != tt.wantErr {
				t.Errorf("toTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("toTable() = %v, want %v", got, tt.want)
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
				fmt.Sprintf("%s: %s\n%s: %s", text.Bold.Sprint("Info String1"), "testInfo1String1", text.Bold.Sprint("Info String2"), "testInfo1String2"),
				fmt.Sprintf("%s: %s\n%s: %s", text.Bold.Sprint("key1"), "value1", text.Bold.Sprint("key2"), "value2"),
				"sliceValue1\nsliceValue2",
				"testString1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rowFromStruct(tt.args.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rowFromStruct() = %v, want %v", got, tt.want)
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
			want: fmt.Sprintf("%s: %s\n%s: %s", text.Bold.Sprint("Info String1"), "testInfo1String1", text.Bold.Sprint("Info String2"), "testInfo1String2"),
		},
		{
			name: "testInfo struct 2",
			args: args{i: ti2},
			want: fmt.Sprintf("%s: %s\n%s: %s", text.Bold.Sprint("Info String1"), "testInfo2String1", text.Bold.Sprint("Info String2"), "testInfo2String2"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headerStructFieldsToString(tt.args.i); got != tt.want {
				t.Errorf("headerStructFieldsToString() = %v, want %v", got, tt.want)
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
			want: fmt.Sprintf("%s: 1\n%s: 3\n%s: 2", text.Bold.Sprint("one"), text.Bold.Sprint("three"), text.Bold.Sprint("two")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedStringMapToString(tt.args.m); got != tt.want {
				t.Errorf("sortedStringMapToString() = %v, want %v", got, tt.want)
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
				t.Errorf("sortedStringSliceToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
				t.Errorf("toBold() = %v, want %v", got, tt.want)
			}
		})
	}
}
