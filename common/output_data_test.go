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
	"context"
	"reflect"
)

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

func (tr *testResults) Search(_ context.Context) {}
func (tr *testResults) Len() int                 { return len(tr.Data) }
func (tr *testResults) GetProfile() string       { return tr.Profile }
func (tr *testResults) GetRegion() string        { return tr.Region }
func (tr *testResults) GetErrors() []string      { return tr.Errors }
func (tr *testResults) AddError(msg string)      { tr.Errors = append(tr.Errors, msg) }
func (tr *testResults) GetSortField() string     { return "field" }
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
	StructField testInfo `json:"struct_field" header:"Struct Field"`
	// MapField header is `Tags`` because there is a test case for `--show-tags` on toTable().
	MapField    map[string]string `json:"map_field" header:"Tags"`
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

// mockResults returns a new result set with two rows and two errors.
func mockResults() *testResults {
	return &testResults{
		Profile: "testProfile",
		Region:  "testRegion",
		Errors:  []string{"testError1", "testError2"},
		Data: []testDataRow{
			mockDataRow1(),
			mockDataRow2(),
		},
	}
}

// mockResultsEmpty returns a new empty result set.
func mockResultsEmpty() *testResults {
	return &testResults{
		Profile: "testProfileEmpty",
		Region:  "testRegionEmpty",
		Errors:  []string{},
		Data:    []testDataRow{},
	}
}

// mockDataRow1 returns the first row: a struct, a map, a slice and a string field.
func mockDataRow1() testDataRow {
	return testDataRow{
		StructField: mockInfo1(),
		MapField: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		SliceField:  []string{"sliceValue1", "sliceValue2"},
		StringField: "testString1",
	}
}

// mockDataRow2 returns the second row.
func mockDataRow2() testDataRow {
	return testDataRow{
		StructField: mockInfo2(),
		MapField: map[string]string{
			"key3": "value3",
			"key4": "value4",
		},
		SliceField:  []string{"sliceValue3", "sliceValue4"},
		StringField: "testString2",
	}
}

// mockInfo1 returns the struct field of the first row.
func mockInfo1() testInfo {
	return testInfo{
		InfoString1: "testInfo1String1",
		InfoString2: "testInfo1String2",
	}
}

// mockInfo2 returns the struct field of the second row.
func mockInfo2() testInfo {
	return testInfo{
		InfoString1: "testInfo2String1",
		InfoString2: "testInfo2String2",
	}
}

// jsonEmptyNoPretty is a json string used for testing.
const jsonEmptyNoPretty = `{"profile":"testProfileEmpty","region":"testRegionEmpty","data":[]}`

// jsonNoPretty is a json string used for testing.
//
//nolint:lll // the expected compact JSON is one line by definition
const jsonNoPretty = `{"profile":"testProfile","region":"testRegion","errors":["testError1","testError2"],"data":[{"struct_field":{"info_string1":"testInfo1String1","info_string2":"testInfo1String2"},"map_field":{"key1":"value1","key2":"value2"},"slice_field":["sliceValue1","sliceValue2"],"string_field":"testString1"},{"struct_field":{"info_string1":"testInfo2String1","info_string2":"testInfo2String2"},"map_field":{"key3":"value3","key4":"value4"},"slice_field":["sliceValue3","sliceValue4"],"string_field":"testString2"}]}`

// jsonEmptyPretty is a json string used for testing.
const jsonEmptyPretty = `{
  "profile": "testProfileEmpty",
  "region": "testRegionEmpty",
  "data": []
}`

// jsonPretty is a json string used for testing.
const jsonPretty = `{
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

// tableNoTags is a test table output from tr.
const tableNoTags = `+-------------------------------------------------------------+
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
const tableTags = `+----------------------------------------------------------------------------+
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

// tableTagsFiltered is a test table output from tr with tags restricted to key1 and key3.
const tableTagsFiltered = `+----------------------------------------------------------------------------+
| [Profile] testProfile [Region] testRegion [Sort] field                     |
|                                                                            |
| testError1                                                                 |
| testError2                                                                 |
+--------------------------------+--------------+-------------+--------------+
| Struct Field                   | Tags         | Slice Field | String Field |
+--------------------------------+--------------+-------------+--------------+
| Info String1: testInfo1String1 | key1: value1 | sliceValue1 | testString1  |
| Info String2: testInfo1String2 |              | sliceValue2 |              |
+--------------------------------+--------------+-------------+--------------+
| Info String1: testInfo2String1 | key3: value3 | sliceValue3 | testString2  |
| Info String2: testInfo2String2 |              | sliceValue4 |              |
+--------------------------------+--------------+-------------+--------------+
`

// tableEmptyNoTags is a test table output from trEmpty.
const tableEmptyNoTags = `+-------------------------------------------+
| [Profile] testProfileEmpty [Region] testR |
| egionEmpty [Sort] field                   |
+--------------+-------------+--------------+
| Struct Field | Slice Field | String Field |
+--------------+-------------+--------------+
+--------------+-------------+--------------+
`

// tableEmptyTags is a test table output from trEmpty with tags.
const tableEmptyTags = `+--------------------------------------------------+
| [Profile] testProfileEmpty [Region] testRegionEm |
| pty [Sort] field                                 |
+--------------+------+-------------+--------------+
| Struct Field | Tags | Slice Field | String Field |
+--------------+------+-------------+--------------+
+--------------+------+-------------+--------------+
`
