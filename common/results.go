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

// BaseResults contains the common fields shared by all resource result types.
type BaseResults struct {
	// Profile is the profile used to search.
	Profile string `json:"profile"`

	// Region is the region used to search.
	Region string `json:"region"`

	// Errors contains the errors found during the search.
	Errors []string `json:"errors,omitempty"`

	// SortField is the field used to sort the results.
	SortField string `json:"-"`
}

// GetProfile returns the profile used to search.
func (b *BaseResults) GetProfile() string { return b.Profile }

// GetRegion returns the region used to search.
func (b *BaseResults) GetRegion() string { return b.Region }

// GetErrors returns the errors found during the search.
func (b *BaseResults) GetErrors() []string { return b.Errors }

// GetSortField returns the field used to sort the results.
func (b *BaseResults) GetSortField() string { return b.SortField }

// AddError records an error of the search.
func (b *BaseResults) AddError(msg string) { b.Errors = append(b.Errors, msg) }

// RowSet holds the rows of a result set and gives the Results methods that need only the rows.
//
// A resource type embeds it with its row type (common.RowSet[dataRow]) and keeps only its AWS
// calls, its row parsing and its filters. The row type's struct tags drive the headers, the
// rows and the sort fields (see Headers, Rows and SortFields).
type RowSet[T any] struct {
	BaseResults

	// Data contains the rows found.
	Data []T `json:"data"`
}

// NewRowSet returns an empty RowSet for the given profile, region and sort field.
func NewRowSet[T any](profile, region, sortField string) RowSet[T] {
	return RowSet[T]{
		BaseResults: BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data: []T{},
	}
}

// Len returns the number of rows.
func (t *RowSet[T]) Len() int { return len(t.Data) }

// GetHeaders returns the `header` tags of the row type's fields.
func (t *RowSet[T]) GetHeaders() []interface{} {
	var row T
	return Headers(row)
}

// GetRows returns the rows as a slice of interface{}.
func (t *RowSet[T]) GetRows() []interface{} { return Rows(t.Data) }

// SortRows sorts the rows by the `sort` tag SortField names, when one is set (see SortByField).
//
// A sort field that is not a `sort` tag of the row type is recorded with AddError and leaves the
// rows in their order.
func (t *RowSet[T]) SortRows() {
	var row T
	t.SortRowsBy(func(f string) (map[string]string, error) { return SortFields(row, f) }, SortByField[T])
}

// SortRowsBy sorts the rows with order, when SortField is set, for a type that sorts its own way.
//
// sortFields maps a sort field to the struct field it sorts by, as SortFields does, and order
// sorts rows by that struct field name. A type whose `sort` tags are on a nested struct passes
// SortFields of that struct; a type that orders the rows before the field passes its own order.
// An error of sortFields is recorded with AddError and leaves the rows in their order.
func (t *RowSet[T]) SortRowsBy(
	sortFields func(field string) (map[string]string, error),
	order func(rows []T, fieldName string),
) {
	if t.SortField == "" {
		return
	}
	fields, err := sortFields(t.SortField)
	if err != nil {
		t.AddError(err.Error())
		return
	}
	order(t.Data, fields[t.SortField])
}
