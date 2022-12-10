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
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"

	"golang.org/x/term"
)

// TermWidth is the width of the terminal.
var TermWidth int

// TermWHeight is the height of the terminal.
var TermWHeight int

// init sets the terminal width and height.
func init() {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil {
		TermWidth = w
		TermWHeight = h
	}
}

// results is an interface that defines the methods that a result must implement.
type Results interface {
	Search()
	Len() int
	GetProfile() string
	GetRegion() string
	GetErrors() []string
	GetSortField() string
	GetHeaders() []interface{}
	GetRows() []interface{}
}

// ParseTags receives a slice of strings and returns a map of tags and values.
//
// It expects a string in the format of "key1=value1:value2,key2=value3" and returns a
// map[string][]string{"key1":{"value1","value2"},"key2":{"value3"}}.
func ParseTags(tags []string) (map[string][]string, error) {
	m := map[string][]string{}
	for _, tag := range tags {
		splited := strings.Split(tag, "=")
		if len(splited) != 2 {
			return nil, fmt.Errorf("invalid tag: %s", tag)
		}
		key := splited[0]
		values := strings.Split(splited[1], ":")
		m[key] = values
	}
	return m, nil
}

// StringValue returns an empty string if the pointer is nil.
func StringValue(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// String returns a pointer to a string.
func String(s string) *string {
	return &s
}

// StringInSlice returns true if the string is in the slice.
func StringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if s == v {
			return true
		}
	}
	return false
}

// StringSliceToString returns a string from a slice of strings.
//
// The separator is the string that will be used to join the strings.
func StringSliceToString(s []string, sep string) string {
	return strings.Join(s, sep)
}

// IPtoString returns a slice of strings from a slice of IPs.
func IPtoString(i []net.IP) []string {
	ips := []string{}
	for _, ip := range i {
		ips = append(ips, ip.String())
	}
	return ips
}

// StructToFilters returns a map of filters from a struct.
//
// The struct must have the tag "filter" in the fields that should be used as filters.
func StructToFilters(s interface{}) (map[string][]string, error) {
	filters := map[string][]string{}
	v := reflect.ValueOf(s)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Len() > 0 {
			switch reflect.TypeOf(v.Field(i).Interface()) {
			case reflect.TypeOf([]net.IP{}):
				filters[v.Type().Field(i).Tag.Get("filter")] = IPtoString(v.Field(i).Interface().([]net.IP))
			case reflect.TypeOf([]string{}):
				filters[v.Type().Field(i).Tag.Get("filter")] = v.Field(i).Interface().([]string)
			}
		}
	}
	if len(filters) == 0 {
		return nil, fmt.Errorf("you must provide at least one filter")
	}
	return filters, nil
}
