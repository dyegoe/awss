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

package common

import (
	"cmp"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Headers returns the `header` struct tags of row's fields, in declaration order.
//
// row must be a struct value (for example dataRow{}). Fields without a `header` tag are skipped.
func Headers(row interface{}) []interface{} {
	headers := []interface{}{}

	t := reflect.TypeOf(row)
	for i := 0; i < t.NumField(); i++ {
		if header, ok := t.Field(i).Tag.Lookup("header"); ok {
			headers = append(headers, header)
		}
	}

	return headers
}

// Rows converts a typed slice of rows into a slice of interface{} for the output layer.
func Rows[T any](data []T) []interface{} {
	rows := make([]interface{}, 0, len(data))
	for i := range data {
		rows = append(rows, data[i])
	}
	return rows
}

// SortFieldNames returns the `sort` struct tags of row's fields, sorted alphabetically.
//
// It is used to build help texts that stay in sync with the struct tags.
func SortFieldNames(row interface{}) []string {
	names := []string{}

	t := reflect.TypeOf(row)
	for i := 0; i < t.NumField(); i++ {
		if s, ok := t.Field(i).Tag.Lookup("sort"); ok {
			names = append(names, s)
		}
	}

	sort.Strings(names)
	return names
}

// SortFields returns a map of `sort` struct tag to struct field name for row.
//
// row must be a struct value (for example dataRow{}). It returns an error listing the valid
// options when f is not one of the `sort` tags.
func SortFields(row interface{}, f string) (map[string]string, error) {
	sortFields := map[string]string{}

	t := reflect.TypeOf(row)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if s, ok := field.Tag.Lookup("sort"); ok {
			sortFields[s] = field.Name
		}
	}

	if _, ok := sortFields[f]; !ok {
		options := StringSliceToString(SortFieldNames(row), ", ")
		return nil, fmt.Errorf("invalid sort field: %s. The options are: %s", f, options)
	}
	return sortFields, nil
}

// SortByField sorts data in place by the struct field named fieldName, using Less.
//
// T must be a struct type and fieldName one of its exported fields.
func SortByField[T any](data []T, fieldName string) {
	sort.SliceStable(data, func(p, q int) bool {
		return Less(
			reflect.ValueOf(data[p]).FieldByName(fieldName),
			reflect.ValueOf(data[q]).FieldByName(fieldName),
		)
	})
}

// Version is a version string, such as an engine version, that sorts naturally: 8.0.39 before
// 13.4 before 13.15. It prints and marshals as a plain string.
type Version string

// Less reports whether a sorts before b.
//
// Integers compare numerically, a Version compares naturally (see NaturalLess), slices compare by
// their sorted comma-joined elements, and everything else compares by its string form.
func Less(a, b reflect.Value) bool {
	if a.Type() == reflect.TypeFor[Version]() {
		return NaturalLess(a.String(), b.String())
	}
	switch a.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() < b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.Uint() < b.Uint()
	case reflect.Slice:
		return sliceSortKey(a) < sliceSortKey(b)
	default:
		return a.String() < b.String()
	}
}

// NaturalLess reports whether a sorts before b when the runs of digits compare as numbers: 13.4
// before 13.15, and 8.0 before 13.0. Other characters compare as text; at a tie, the shorter
// string sorts first.
func NaturalLess(a, b string) bool {
	for a != "" && b != "" {
		da, db := digitPrefix(a), digitPrefix(b)
		if da != "" && db != "" {
			if c := compareNumbers(da, db); c != 0 {
				return c < 0
			}
			a, b = a[len(da):], b[len(db):]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

// compareNumbers compares two runs of digits as numbers, of any length: -1, 0 or +1.
func compareNumbers(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	// Without leading zeros, a longer number is a larger one.
	if len(a) != len(b) {
		return cmp.Compare(len(a), len(b))
	}
	return strings.Compare(a, b)
}

// digitPrefix returns the run of ASCII digits s starts with, or "".
func digitPrefix(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// sliceSortKey returns a comparable key for a slice: its elements formatted, sorted and joined.
func sliceSortKey(v reflect.Value) string {
	parts := make([]string, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		parts = append(parts, fmt.Sprint(v.Index(i).Interface()))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
