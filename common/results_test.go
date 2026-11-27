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
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// setRow is the row type of the RowSet tests.
type setRow struct {
	ID   string `json:"id" header:"ID" sort:"id"`
	Size int    `json:"size" header:"Size" sort:"size"`
	Note string `json:"note"`
}

// TestNewRowSet checks that NewRowSet sets the profile, region and sort field, with empty, non-nil
// errors and rows.
func TestNewRowSet(t *testing.T) {
	got := NewRowSet[setRow]("p", "r", "id")
	if got.Profile != "p" || got.Region != "r" || got.SortField != "id" {
		t.Errorf("NewRowSet(p, r, id) = %+v, want profile p, region r, sort field id", got.BaseResults)
	}
	if got.Errors == nil || len(got.Errors) != 0 {
		t.Errorf("NewRowSet(p, r, id).Errors = %#v, want an empty slice", got.Errors)
	}
	if got.Data == nil || len(got.Data) != 0 {
		t.Errorf("NewRowSet(p, r, id).Data = %#v, want an empty slice", got.Data)
	}
}

// TestRowSet_accessors checks Len, GetHeaders and GetRows against the rows and the row type's tags.
func TestRowSet_accessors(t *testing.T) {
	tb := NewRowSet[setRow]("p", "r", "")
	tb.Data = []setRow{{ID: "a", Size: 1}, {ID: "b", Size: 2}}

	if got := tb.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if got, want := tb.GetHeaders(), []interface{}{"ID", "Size"}; !reflect.DeepEqual(got, want) {
		t.Errorf("GetHeaders() = %v, want %v", got, want)
	}
	if got, want := tb.GetRows(), []interface{}{tb.Data[0], tb.Data[1]}; !reflect.DeepEqual(got, want) {
		t.Errorf("GetRows() = %v, want %v", got, want)
	}
}

// TestRowSet_json checks that a type embedding RowSet marshals as before: profile, region, errors,
// then data, with its own fields after.
func TestRowSet_json(t *testing.T) {
	type results struct {
		RowSet[setRow]
		Filters map[string][]string `json:"-"`
	}
	r := results{RowSet: NewRowSet[setRow]("p", "r", "id")}
	r.Data = append(r.Data, setRow{ID: "a", Size: 1})

	got, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	want := `{"profile":"p","region":"r","data":[{"id":"a","size":1,"note":""}]}`
	if string(got) != want {
		t.Errorf("json.Marshal() = %s, want %s", got, want)
	}
}

// TestRowSet_SortRows checks the sort by a `sort` tag, no sort without a sort field, and the error
// of an unknown sort field.
func TestRowSet_SortRows(t *testing.T) {
	tests := []struct {
		name      string
		sortField string
		wantIDs   []string
		wantErr   string
	}{
		{name: "by size, numerically", sortField: "size", wantIDs: []string{"b", "c", "a"}},
		{name: "by id", sortField: "id", wantIDs: []string{"a", "b", "c"}},
		{name: "no sort field", sortField: "", wantIDs: []string{"c", "b", "a"}},
		{name: "unknown sort field", sortField: "note", wantIDs: []string{"c", "b", "a"},
			wantErr: "invalid sort field: note"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := NewRowSet[setRow]("p", "r", tt.sortField)
			tb.Data = []setRow{{ID: "c", Size: 9}, {ID: "b", Size: 2}, {ID: "a", Size: 10}}

			tb.SortRows()

			if got := setIDs(tb.Data); !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("SortRows() with %q = %v, want %v", tt.sortField, got, tt.wantIDs)
			}
			checkSetErrors(t, tb.Errors, tt.wantErr)
		})
	}
}

// TestRowSet_SortRowsBy checks that SortRowsBy passes the struct field name to order, and records
// an error of sortFields without calling order.
func TestRowSet_SortRowsBy(t *testing.T) {
	tests := []struct {
		name      string
		sortField string
		fieldsErr error
		wantField string
		wantErr   string
	}{
		{name: "field name passed", sortField: "x", wantField: "X"},
		{name: "no sort field", sortField: ""},
		{name: "sortFields error", sortField: "x", fieldsErr: errors.New("bad field"), wantErr: "bad field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := NewRowSet[setRow]("p", "r", tt.sortField)
			gotField := ""
			tb.SortRowsBy(
				func(f string) (map[string]string, error) {
					return map[string]string{f: strings.ToUpper(f)}, tt.fieldsErr
				},
				func(_ []setRow, fieldName string) { gotField = fieldName },
			)
			if gotField != tt.wantField {
				t.Errorf("SortRowsBy() order got %q, want %q", gotField, tt.wantField)
			}
			checkSetErrors(t, tb.Errors, tt.wantErr)
		})
	}
}

// setIDs returns the IDs of rows, in order.
func setIDs(rows []setRow) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// checkSetErrors checks that errs is empty when want is "", or holds one error containing want.
func checkSetErrors(t *testing.T, errs []string, want string) {
	t.Helper()
	if want == "" {
		if len(errs) != 0 {
			t.Errorf("Errors = %v, want none", errs)
		}
		return
	}
	if len(errs) != 1 || !strings.Contains(errs[0], want) {
		t.Errorf("Errors = %v, want one containing %q", errs, want)
	}
}
