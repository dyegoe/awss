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
	"fmt"
	"reflect"
	"testing"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

type testStruct struct {
	Field1 testSubStruct     `header:"header1"`
	Field2 map[string]string `header:"header2"`
	Field3 []string          `header:"header3"`
	Field4 string            `header:"header4"`
}

type testSubStruct struct {
	SubField1 string `header:"subHeader1"`
	SubField2 string `header:"subHeader2"`
}

func Test_rowFromStruct(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name string
		args args
		want table.Row
	}{
		{
			name: "empty",
			args: args{i: struct{}{}},
			want: table.Row{},
		},
		{
			name: "test struct",
			args: args{
				i: testStruct{
					Field1: testSubStruct{SubField1: "subValue1", SubField2: "subValue2"},
					Field2: map[string]string{"key2.1": "value2.1", "key2.2": "value2.2"},
					Field3: []string{"value3.1", "value3.2"},
					Field4: "value1",
				},
			},
			want: table.Row{
				fmt.Sprintf("%s: subValue1\n%s: subValue2", text.Bold.Sprint("subHeader1"), text.Bold.Sprint("subHeader2")),
				fmt.Sprintf("%s: value2.1\n%s: value2.2", text.Bold.Sprint("key2.1"), text.Bold.Sprint("key2.2")),
				"value3.1\nvalue3.2",
				"value1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rowFromStruct(tt.args.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rowFromStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_headerStructFieldsToString(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{i: struct{}{}},
			want: "",
		},
		{
			name: "test struct",
			args: args{i: testSubStruct{SubField1: "value1", SubField2: "value2"}},
			want: fmt.Sprintf("%s: value1\n%s: value2", text.Bold.Sprint("subHeader1"), text.Bold.Sprint("subHeader2")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headerStructFieldsToString(tt.args.i); got != tt.want {
				t.Errorf("headerStructFieldsToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortedStringMapToString(t *testing.T) {
	type args struct {
		m map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{m: map[string]string{}},
			want: "",
		},
		{
			name: "one",
			args: args{m: map[string]string{"one": "1"}},
			want: fmt.Sprintf("%s: 1", text.Bold.Sprint("one")),
		},
		{
			name: "two",
			args: args{m: map[string]string{"one": "1", "two": "2"}},
			want: fmt.Sprintf("%s: 1\n%s: 2", text.Bold.Sprint("one"), text.Bold.Sprint("two")),
		},
		{
			name: "three",
			args: args{m: map[string]string{"one": "1", "two": "2", "three": "3"}},
			want: fmt.Sprintf("%s: 1\n%s: 3\n%s: 2", text.Bold.Sprint("one"), text.Bold.Sprint("three"), text.Bold.Sprint("two")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedStringMapToString(tt.args.m); got != tt.want {
				t.Errorf("sortedStringMapToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortedStringSliceToString(t *testing.T) {
	type args struct {
		s []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{s: []string{}},
			want: "",
		},
		{
			name: "one",
			args: args{s: []string{"one"}},
			want: "one",
		},
		{
			name: "two",
			args: args{s: []string{"one", "two"}},
			want: "one\ntwo",
		},
		{
			name: "three",
			args: args{s: []string{"one", "two", "three"}},
			want: "one\nthree\ntwo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedStringSliceToString(tt.args.s); got != tt.want {
				t.Errorf("sortedStringSliceToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
