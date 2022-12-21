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
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

const (
	// JSON is the JSON output format.
	JSON = "json"
	// JSONPretty is the pretty JSON output format.
	JSONPretty = "json-pretty"
	// TABLE is the table output format.
	Table = "table"
)

// ValidOutputs is the list of available output formats.
var ValidOutputs = []string{JSON, JSONPretty, Table}

// PrintResults prints the results in the given format.
//
// The results are read from the resultsChan channel.
// The done channel is used to signal that the results were printed.
// The output is the format of the output.
// The showEmpty flag indicates if empty results should be shown.
func PrintResults(resultsChan <-chan Results, done chan<- bool, output string, showEmpty, showTags bool) {
	s := ""
	err := error(nil)

	for results := range resultsChan {
		switch output {
		case JSON:
			s, err = toJSON(results, showEmpty, false)
		case JSONPretty:
			s, err = toJSON(results, showEmpty, true)
		case Table:
			s, err = toTable(results, showEmpty, showTags)
		default:
			err = fmt.Errorf("output format %s not found", output)
		}
		if err != nil {
			fmt.Println(err)
			continue
		}
		if s != "" {
			fmt.Println(s)
		}
	}
	done <- true
}

// Bold is the function used to bold text.
//
// We use this var to allow tests to mock the function.
var Bold = toBold

// toJSON returns the results in JSON format.
//
// If pretty is true, the JSON is formatted.
func toJSON(r Results, showEmpty, pretty bool) (string, error) {
	var b []byte
	var err error
	if r.Len() > 0 || showEmpty {
		if pretty {
			b, err = json.MarshalIndent(r, "", "  ")
		} else {
			b, err = json.Marshal(r)
		}
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", nil
}

// toTable returns the results in table format.
//
// If showEmpty is true, the table is shown even if there are no results.
func toTable(r Results, showEmpty, showTags bool) (string, error) {
	if r.Len() == 0 && !showEmpty {
		return "", nil
	}

	tableStyle := table.StyleDefault
	tableStyle.Format.Header = text.FormatDefault
	tableStyle.Title.Align = text.AlignLeft

	t := table.NewWriter()
	t.SetStyle(tableStyle)
	t.SetAllowedRowLength(TermWidth)
	t.Style().Options.SeparateRows = true

	errors := r.GetErrors()
	showErrors := ""
	if len(errors) > 0 {
		showErrors = fmt.Sprintf("\n\n%s", StringSliceToString(errors, "\n"))
	}

	showSort := ""
	if sort := r.GetSortField(); sort != "" {
		showSort = fmt.Sprintf("%s %s", Bold("[Sort]"), sort)
	}

	t.SetTitle(fmt.Sprintf("%s %s %s %s %s %s", Bold("[Profile]"), r.GetProfile(), Bold("[Region]"), r.GetRegion(), showSort, showErrors))

	t.AppendHeader(r.GetHeaders())

	for _, d := range r.GetRows() {
		t.AppendRow(rowFromStruct(d))
	}

	t.SetColumnConfigs(
		[]table.ColumnConfig{
			{Name: "Tags", Hidden: !showTags},
		},
	)

	return fmt.Sprintf("%s\n", t.Render()), nil
}

// RowsFromStruct returns a table.Row from a struct.
//
// If the field is a map, it calls mapToTable.
// If the field is a slice, it joins the elements with a new line.
// If the field is a struct, it calls structToTable.
func rowFromStruct(i interface{}) table.Row {
	row := table.Row{}

	v := reflect.ValueOf(i)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		if _, ok := v.Type().Field(i).Tag.Lookup("header"); ok {
			switch field.Kind() {
			case reflect.Struct:
				row = append(row, headerStructFieldsToString(field.Interface()))
			case reflect.Map:
				row = append(row, sortedStringMapToString(field.Interface().(map[string]string)))
			case reflect.Slice:
				row = append(row, sortedStringSliceToString(field.Interface().([]string)))
			default:
				row = append(row, field.Interface())
			}
		}
	}
	return row
}

// headerStructFieldsToString returns a string from a struct.
//
// Headers are used from the struct tag `header:"<header>"`.
// The string is presented in the format:
// <header>: <value>
// <header>: <value>
// ...
func headerStructFieldsToString(i interface{}) string {
	var s []string

	v := reflect.ValueOf(i)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		if header, ok := v.Type().Field(i).Tag.Lookup("header"); ok && field.Interface() != "" {
			s = append(s, fmt.Sprintf("%s: %s", Bold(header), v.Field(i).Interface()))
		}
	}

	return StringSliceToString(s, "\n")
}

// sortedStringMapToString returns a string from a map.
//
// The results are sorted by the keys.
// The string is presented in the format:
// <key>: <value>
// <key>: <value>
// ...
func sortedStringMapToString(m map[string]string) string {
	var s []string

	for k, v := range m {
		s = append(s, fmt.Sprintf("%s: %s", Bold(k), v))
	}

	sort.Strings(s)

	return StringSliceToString(s, "\n")
}

// sortedStringSliceToString returns a string from a slice.
//
// The results are sorted.
// The string is presented in the format:
// <value>
// <value>
// ...
func sortedStringSliceToString(s []string) string {
	sort.Strings(s)
	return StringSliceToString(s, "\n")
}

// toBold returns a string in bold.
func toBold(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("\033[1m%s\033[0m", s)
}
