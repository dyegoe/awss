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

// Package common contains the common code for awss.
package common

type Filter struct {
	flag      string
	shortflag string
	values    []string
	usage     string
	name      string
}

type FilterType int

const (
	FilterString FilterType = iota
	FilterIP
)

// EmptyFilterFieldError is the error returned when a filter field is empty.
type EmptyFilterFieldError struct {
	Field string
}

// Error returns the error message.
func (e *EmptyFilterFieldError) Error() string {
	return "filter " + e.Field + " cannot be empty"
}

func NewFilter(flag, shortflag string, filterType FilterType, usage, name string) (Filter, error) {
	f := Filter{}

	if flag == "" {
		return f, &EmptyFilterFieldError{Field: "flag"}
	}

	if shortflag == "" {
		return f, &EmptyFilterFieldError{Field: "shortflag"}
	}

	switch filterType {
	case FilterString:
	case FilterIP:
	default:
		return f, &EmptyFilterFieldError{Field: "filterType"}
	}

	if usage == "" {
		return f, &EmptyFilterFieldError{Field: "usage"}
	}

	if name == "" {
		return f, &EmptyFilterFieldError{Field: "name"}
	}

	f.flag = flag
	f.shortflag = shortflag
	f.values = []string{}
	f.usage = usage
	f.name = name

	return f, nil
}

func (f *Filter) Flag() string {
	return f.flag
}

func (f *Filter) ShortFlag() string {
	return f.shortflag
}

func (f *Filter) Values() []string {
	return f.values
}

func (f *Filter) Usage() string {
	return f.usage
}

func (f *Filter) Name() string {
	return f.name
}

type Filters []Filter
