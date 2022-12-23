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
	"io"
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
	// Table is the table output format.
	Table = "table"
)

// outputs is a map of output formats to functions that print the results in the given format.
//
// The key is the output format.
// The value is the function that prints the results in the given format.
var outputs = map[string]func(Results, bool, bool) (string, error){
	JSON:       toJSON,
	JSONPretty: toJSONPretty,
	Table:      toTable,
}

// ValidOutputs returns the valid output formats and if the given output is valid.
func ValidOutputs(o string) (string, bool) {
	var valid []string
	for k := range outputs {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	_, ok := outputs[o]
	return StringSliceToString(valid, ", "), ok
}

// PrintResults prints the results in the given format.
//
// The results are read from the resultsChan channel.
// The done channel is used to signal that the results were printed.
// The output is the format of the output.
// The showEmpty flag indicates if empty results should be shown.
// The showTags flag indicates if the tags should be shown.
func PrintResults(w io.Writer, resultsChan <-chan Results, done chan<- bool, output string, showEmpty, showTags bool) {
	for results := range resultsChan {
		printResults, ok := outputs[output]
		if !ok {
			fmt.Fprintf(w, "Invalid output format: %s\n", output)
			continue
		}
		s, err := printResults(results, showEmpty, showTags)
		if err != nil {
			fmt.Fprintln(w, err)
			continue
		}
		if s != "" {
			fmt.Fprintln(w, s)
		}
	}
	done <- true
}

// Bold is the function used to bold text.
//
// We use this var to allow tests to mock the function.
var Bold = toBold

// toBold returns a string in bold.
func toBold(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("\033[1m%s\033[0m", s)
}

// toJSON returns the results in JSON format.
//
// showEmpty indicates if empty results should be shown.
// showTags indicates if the tags should be shown. It is ignored for json format.
func toJSON(r Results, showEmpty, showTags bool) (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	if r.Len() == 0 && !showEmpty {
		return "", nil
	}
	return string(b), nil
}

// toJSONPretty returns the results in JSON format.
//
// showEmpty indicates if empty results should be shown.
// showTags indicates if the tags should be shown. It is ignored for json format.
func toJSONPretty(r Results, showEmpty, showTags bool) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if r.Len() == 0 && !showEmpty {
		return "", nil
	}
	return string(b), nil
}

// toTable returns the results in table format.
//
// showEmpty indicates if empty results should be shown.
// showTags indicates if the tags should be shown.
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
