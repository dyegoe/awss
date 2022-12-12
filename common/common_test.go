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
// common defines an important interface that all results must implement.
// It has AWS related functions and types.
// It also has functions to print the results in different formats.
package common

import (
	"net"
	"reflect"
	"testing"
)

func TestParseTags(t *testing.T) {
	type args struct {
		tags []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string][]string
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{tags: []string{}},
			want:    map[string][]string{},
			wantErr: false,
		},
		{
			name:    "key=value",
			args:    args{tags: []string{"key=value"}},
			want:    map[string][]string{"key": {"value"}},
			wantErr: false,
		},
		{
			name:    "key=value,key2=value2",
			args:    args{tags: []string{"key=value", "key2=value2"}},
			want:    map[string][]string{"key": {"value"}, "key2": {"value2"}},
			wantErr: false,
		},
		{
			name:    "key=value:value2",
			args:    args{tags: []string{"key=value:value2"}},
			want:    map[string][]string{"key": {"value", "value2"}},
			wantErr: false,
		},
		{
			name:    "key=value:value2,key2=value3:value4",
			args:    args{tags: []string{"key=value:value2", "key2=value3:value4"}},
			want:    map[string][]string{"key": {"value", "value2"}, "key2": {"value3", "value4"}},
			wantErr: false,
		},
		{
			name:    "key",
			args:    args{tags: []string{"key"}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "key=value:value2,key2",
			args:    args{tags: []string{"key=value:value2", "key2"}},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTags(tt.args.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringValue(t *testing.T) {
	type args struct {
		s *string
	}
	value1 := ""
	value2 := "value"
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "nil",
			args: args{s: nil},
			want: "",
		},
		{
			name: "empty",
			args: args{s: &value1},
			want: "",
		},
		{
			name: "value",
			args: args{s: &value2},
			want: "value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringValue(tt.args.s); got != tt.want {
				t.Errorf("StringValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	type args struct {
		s string
	}
	value1 := ""
	value2 := "value"
	tests := []struct {
		name string
		args args
		want *string
	}{
		{
			name: "empty",
			args: args{s: value1},
			want: &value1,
		},
		{
			name: "value",
			args: args{s: value2},
			want: &value2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := String(tt.args.s); *got != *tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringInSlice(t *testing.T) {
	type args struct {
		s     string
		slice []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty",
			args: args{s: "value", slice: []string{}},
			want: false,
		},
		{
			name: "one value",
			args: args{s: "value", slice: []string{"value"}},
			want: true,
		},
		{
			name: "two values",
			args: args{s: "value", slice: []string{"value", "value2"}},
			want: true,
		},
		{
			name: "two but one not found",
			args: args{s: "value", slice: []string{"value2", "value3"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringInSlice(tt.args.s, tt.args.slice); got != tt.want {
				t.Errorf("StringInSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringSliceToString(t *testing.T) {
	type args struct {
		s   []string
		sep string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{s: []string{}, sep: ","},
			want: "",
		},
		{
			name: "one value",
			args: args{s: []string{"value"}, sep: ","},
			want: "value",
		},
		{
			name: "two values",
			args: args{s: []string{"value", "value2"}, sep: ","},
			want: "value,value2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringSliceToString(tt.args.s, tt.args.sep); got != tt.want {
				t.Errorf("StringSliceToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIPtoString(t *testing.T) {
	type args struct {
		i []net.IP
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty",
			args: args{i: []net.IP{}},
			want: []string{},
		},
		{
			name: "one ip",
			args: args{i: []net.IP{net.ParseIP("172.16.0.1")}},
			want: []string{"172.16.0.1"},
		},
		{
			name: "two ips",
			args: args{i: []net.IP{net.ParseIP("172.16.0.1"), net.ParseIP("172.17.1.254")}},
			want: []string{"172.16.0.1", "172.17.1.254"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IPtoString(tt.args.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IPtoString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStructToFilters(t *testing.T) {
	// This is a struct that will be used to test the StructToFilters function
	type testFilters struct {
		SliceOfStringField []string `filter:"slice-of-string-field"`
		NetIPField         []net.IP `filter:"net-ip-field"`
		StringField        string   `filter:"string-field"`
		FieldNotTagged     string
	}

	type args struct {
		s interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string][]string
		wantErr bool
	}{
		{
			name: "slice of string and net.IP",
			args: args{
				s: testFilters{
					SliceOfStringField: []string{"value1", "value2"},
					StringField:        "value3",
					NetIPField:         []net.IP{net.ParseIP("172.16.0.1"), net.ParseIP("172.17.1.254")},
					FieldNotTagged:     "value4",
				},
			},
			want: map[string][]string{
				"slice-of-string-field": {"value1", "value2"},
				"net-ip-field":          {"172.16.0.1", "172.17.1.254"},
			},
			wantErr: false,
		},
		{
			name: "empty struct",
			args: args{
				s: testFilters{},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StructToFilters(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("StructToFilters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StructToFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}
