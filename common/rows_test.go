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
	"reflect"
	"testing"
)

type testRow struct {
	ID     string   `header:"ID" sort:"id"`
	Size   int32    `header:"Size" sort:"size"`
	Count  uint8    `sort:"count"`
	Items  []string `header:"Items" sort:"items"`
	Hidden string
	Tags   map[string]string `header:"Tags"`
}

// TestHeaders checks that only fields with a header tag become columns, in field order.
func TestHeaders(t *testing.T) {
	want := []interface{}{"ID", "Size", "Items", "Tags"}
	if got := Headers(testRow{}); !reflect.DeepEqual(got, want) {
		t.Errorf("Headers() = %#v, want %#v", got, want)
	}
}

// TestRows checks that each element of a slice becomes one row, and an empty slice none.
func TestRows(t *testing.T) {
	data := []testRow{{ID: "a"}, {ID: "b"}}
	got := Rows(data)
	want := []interface{}{testRow{ID: "a"}, testRow{ID: "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Rows() = %#v, want %#v", got, want)
	}
	if got := Rows([]testRow{}); len(got) != 0 {
		t.Errorf("Rows(empty) = %#v, want empty", got)
	}
}

// TestSortFieldNames checks that the sort tags are listed alphabetically, and none without tags.
func TestSortFieldNames(t *testing.T) {
	want := []string{"count", "id", "items", "size"}
	if got := SortFieldNames(testRow{}); !reflect.DeepEqual(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
	if got := SortFieldNames(struct{ A string }{}); len(got) != 0 {
		t.Errorf("SortFieldNames(no tags) = %v, want empty", got)
	}
}

// TestSortFields checks the sort tag to field mapping, and that an unknown field lists the options.
func TestSortFields(t *testing.T) {
	tests := []struct {
		name    string
		f       string
		want    map[string]string
		wantErr string
	}{
		{
			name: "valid field",
			f:    "size",
			want: map[string]string{"id": "ID", "size": "Size", "count": "Count", "items": "Items"},
		},
		{
			name:    "invalid field lists sorted options",
			f:       "nope",
			wantErr: "invalid sort field: nope. The options are: count, id, items, size",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SortFields(testRow{}, tt.f)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("SortFields() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SortFields() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortFields() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestSortByField checks sorting by strings, by numbers numerically, and by slices.
func TestSortByField(t *testing.T) {
	newData := func() []testRow {
		return []testRow{
			{ID: "b", Size: 10, Count: 2, Items: []string{"z", "a"}},
			{ID: "a", Size: 9, Count: 10, Items: []string{"b"}},
			{ID: "c", Size: 100, Count: 1, Items: nil},
		}
	}
	tests := []struct {
		name  string
		field string
		want  []string
	}{
		{name: "string", field: "ID", want: []string{"a", "b", "c"}},
		{name: "int32 numeric not lexicographic", field: "Size", want: []string{"a", "b", "c"}},
		{name: "uint numeric", field: "Count", want: []string{"c", "b", "a"}},
		{name: "slice by sorted joined elements", field: "Items", want: []string{"c", "b", "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := newData()
			SortByField(data, tt.field)
			got := make([]string, 0, len(data))
			for i := range data {
				got = append(got, data[i].ID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortByField(%q) order = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// TestSortByField_isStable checks that rows with equal keys keep their input order.
func TestSortByField_isStable(t *testing.T) {
	data := []testRow{{ID: "x", Size: 1}, {ID: "y", Size: 1}, {ID: "w", Size: 0}}
	SortByField(data, "Size")
	got := []string{data[0].ID, data[1].ID, data[2].ID}
	want := []string{"w", "x", "y"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortByField() order = %v, want %v (equal keys keep input order)", got, want)
	}
}

// TestLess_sliceDoesNotMutateInput checks that comparing slices does not sort the compared values.
func TestLess_sliceDoesNotMutateInput(t *testing.T) {
	a := []string{"z", "a"}
	b := []string{"m"}
	Less(reflect.ValueOf(a), reflect.ValueOf(b))
	if !reflect.DeepEqual(a, []string{"z", "a"}) {
		t.Errorf("Less() mutated its input slice: %v", a)
	}
}

// TestNaturalLess checks that runs of digits compare as numbers and the rest as text.
func TestNaturalLess(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "minor 4 before 15", a: "13.4", b: "13.15", want: true},
		{name: "minor 15 after 4", a: "13.15", b: "13.4", want: false},
		{name: "major 8 before 13", a: "8.0.39", b: "13.4", want: true},
		{name: "equal", a: "16.4", b: "16.4", want: false},
		{name: "prefix first", a: "16", b: "16.4", want: true},
		{name: "leading zeros are the same number", a: "1.05", b: "1.5", want: false},
		{name: "text part", a: "5.7.mysql_aurora.2", b: "5.7.mysql_aurora.10", want: true},
		{name: "letters as text", a: "10.a", b: "10.b", want: true},
		{name: "digit before letter", a: "1", b: "a", want: true},
		{name: "empty first", a: "", b: "1", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NaturalLess(tt.a, tt.b); got != tt.want {
				t.Errorf("NaturalLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestSortByField_version checks that a Version field sorts naturally and a plain string as text.
func TestSortByField_version(t *testing.T) {
	type row struct {
		V Version
		S string
	}
	data := []row{{"13.15", "13.15"}, {"8.0.39", "8.0.39"}, {"13.4", "13.4"}}

	SortByField(data, "V")
	if got := []Version{data[0].V, data[1].V, data[2].V}; !reflect.DeepEqual(got, []Version{"8.0.39", "13.4", "13.15"}) {
		t.Errorf("SortByField(V) = %v, want [8.0.39 13.4 13.15]", got)
	}
	SortByField(data, "S")
	if got := []string{data[0].S, data[1].S, data[2].S}; !reflect.DeepEqual(got, []string{"13.15", "13.4", "8.0.39"}) {
		t.Errorf("SortByField(S) = %v, want [13.15 13.4 8.0.39]", got)
	}
}
