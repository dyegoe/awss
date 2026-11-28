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

package cmd

//go:generate go run ../internal/fielddoc

import (
	"fmt"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dyegoe/awss/search"
)

const (
	// fieldsTitle opens the Fields section of --help.
	fieldsTitle = `Fields (each row of "data" in --output json; the ones with a column are in the table too):`

	// jsonOnly is the column of a field that only the JSON output has.
	jsonOnly = "JSON only"
)

// field is one line of the Fields section: a field of the result rows.
type field struct {
	// json is the JSON name, with its parents for a nested field: interface_info.id, members[].id.
	json string

	// column is the table column that shows the field, or jsonOnly.
	column string

	// sort is the --sort value of the field; empty when it cannot sort.
	sort string

	// desc is the first sentence of the field's doc comment (fieldDocs).
	desc string
}

// fieldsHelp returns the Fields section of the --help of cmd, or "" for a command with no engine.
func fieldsHelp(cmd *cobra.Command) string {
	e := engineOf(cmd)
	if e == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(fieldsTitle + "\n")
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, f := range rowFields(rowType(e), "", "", false) {
		sort := ""
		if f.sort != "" {
			sort = "sort: " + f.sort
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", f.json, f.column, sort, f.desc)
	}
	_ = w.Flush()
	return strings.TrimRight(b.String(), "\n")
}

// rowType returns the row type of the results e builds: the element type of their Data slice
// (common.RowSet).
func rowType(e *search.Engine) reflect.Type {
	r := e.New("", "", map[string][]string{}, &search.Options{})
	return reflect.Indirect(reflect.ValueOf(r)).FieldByName("Data").Type().Elem()
}

// rowFields lists the JSON fields of the struct type t, in declaration order, each followed by
// the fields of the struct it holds.
//
// prefix goes before each JSON name. cell is the table column that shows t as one cell, in which
// a field with a `header` shows as "Header: value"; empty when t is a row. hidden marks every
// field JSON only, for a struct the table does not show.
func rowFields(t reflect.Type, prefix, cell string, hidden bool) []field {
	var fields []field
	column := ""
	for i := range t.NumField() {
		sf := t.Field(i)
		name, ok := jsonName(&sf)
		if !ok {
			continue
		}
		f := field{
			json: prefix + name,
			sort: sf.Tag.Get("sort"),
			desc: fieldDocs[t.PkgPath()+"."+t.Name()+"."+sf.Name],
		}
		f.column, column = fieldColumn(&sf, cell, column, hidden)
		fields = append(fields, f)
		fields = append(fields, nestedFields(sf.Type, f, hidden)...)
	}
	return fields
}

// nestedFields lists the fields of the struct that the field f, of type t, holds: a struct shows
// in f's cell, a slice of structs in the JSON output only. Any other type has none.
func nestedFields(t reflect.Type, f field, hidden bool) []field {
	switch {
	case t.Kind() == reflect.Struct:
		cell := f.column
		if cell == jsonOnly {
			cell = ""
		}
		return rowFields(t, f.json+".", cell, hidden || cell == "")
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Struct:
		return rowFields(t.Elem(), f.json+"[].", "", true)
	}
	return nil
}

// fieldColumn returns the column of the field sf, and the column the next stacked field goes
// under. last is the column of the field before sf; a stacked field goes on a new line under it.
func fieldColumn(sf *reflect.StructField, cell, last string, hidden bool) (column, base string) {
	header, ok := sf.Tag.Lookup("header")
	switch {
	case !ok || hidden:
		return jsonOnly, last
	case cell != "":
		return cell + ": " + header, last
	}
	if _, stacked := sf.Tag.Lookup("stack"); stacked && last != "" {
		return header + " (under " + last + ")", last
	}
	return header, header
}

// jsonName returns the JSON name of the field sf, as encoding/json names it, and false for a field
// the JSON output leaves out.
func jsonName(sf *reflect.StructField) (string, bool) {
	if !sf.IsExported() {
		return "", false
	}
	tag := sf.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if name, _, _ := strings.Cut(tag, ","); name != "" {
		return name, true
	}
	return sf.Name, true
}
