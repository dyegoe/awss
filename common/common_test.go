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

type testStructToFilters struct {
	SliceOfStringField []string `filter:"slice-of-string-field"`
	NetIPField         []net.IP `filter:"net-ip-field"`
	StringField        string   `filter:"string-field"`
	FieldNotTagged     string
}

type testStructToFiltersCase struct {
	name    string
	input   interface{}
	want    map[string][]string
	wantErr bool
}

func getStructToFiltersCases() []testStructToFiltersCase {
	return []testStructToFiltersCase{
		{
			name: "slice of string and net.IP",
			input: testStructToFilters{
				SliceOfStringField: []string{"value1", "value2"},
				StringField:        "value3",
				NetIPField:         []net.IP{net.ParseIP("172.16.0.1"), net.ParseIP("172.17.1.254")},
				FieldNotTagged:     "value4",
			},
			want: map[string][]string{
				"slice-of-string-field": {"value1", "value2"},
				"net-ip-field":          {"172.16.0.1", "172.17.1.254"},
			},
			wantErr: false,
		},
		{
			name:    "empty struct",
			input:   testStructToFilters{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "pointer to struct",
			input: &testStructToFilters{
				SliceOfStringField: []string{"value1"},
			},
			want: map[string][]string{
				"slice-of-string-field": {"value1"},
			},
			wantErr: false,
		},
		{
			name:    "non-struct value",
			input:   "not a struct",
			want:    nil,
			wantErr: true,
		},
	}
}

// TestParseTags tests the ParseTags function.
//
//nolint:funlen // one table of tag cases, kept together so the formats read side by side
func TestParseTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		want    map[string][]string
		wantErr bool
	}{
		{
			name:    "empty",
			tags:    []string{},
			want:    map[string][]string{},
			wantErr: false,
		},
		{
			name:    "key=value",
			tags:    []string{"key=value"},
			want:    map[string][]string{"key": {"value"}},
			wantErr: false,
		},
		{
			name:    "key=value,key2=value2",
			tags:    []string{"key=value", "key2=value2"},
			want:    map[string][]string{"key": {"value"}, "key2": {"value2"}},
			wantErr: false,
		},
		{
			name:    "key=value:value2",
			tags:    []string{"key=value:value2"},
			want:    map[string][]string{"key": {"value", "value2"}},
			wantErr: false,
		},
		{
			name:    "key=value:value2,key2=value3:value4",
			tags:    []string{"key=value:value2", "key2=value3:value4"},
			want:    map[string][]string{"key": {"value", "value2"}, "key2": {"value3", "value4"}},
			wantErr: false,
		},
		{
			name:    "key",
			tags:    []string{"key"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "key=value:value2,key2",
			tags:    []string{"key=value:value2", "key2"},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTags(tt.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTags()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestCheckCIDRs tests the CheckCIDRs function.
func TestCheckCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		wantErr bool
	}{
		{name: "empty list", cidrs: []string{}, wantErr: false},
		{name: "valid ipv4", cidrs: []string{"10.0.0.0/16", "192.168.1.0/24"}, wantErr: false},
		{name: "valid ipv6", cidrs: []string{"2001:db8::/32"}, wantErr: false},
		{name: "wildcard only", cidrs: []string{"*"}, wantErr: false},
		{name: "wildcard inside", cidrs: []string{"10.0.*"}, wantErr: false},
		{name: "missing prefix length", cidrs: []string{"10.0.0.0"}, wantErr: true},
		{name: "not an address", cidrs: []string{"not-a-cidr"}, wantErr: true},
		{name: "one bad among good", cidrs: []string{"10.0.0.0/16", "bad"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckCIDRs(tt.cidrs); (err != nil) != tt.wantErr {
				t.Errorf("CheckCIDRs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseIPv4CIDRs tests the ParseIPv4CIDRs function.
func TestParseIPv4CIDRs(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		want    []string
		wantErr bool
	}{
		{name: "empty list", cidrs: []string{}, want: []string{}},
		{name: "valid", cidrs: []string{"10.0.0.0/16", "100.64.0.0/16"}, want: []string{"10.0.0.0/16", "100.64.0.0/16"}},
		{name: "host bits normalized", cidrs: []string{"10.0.1.5/24"}, want: []string{"10.0.1.0/24"}},
		{name: "wildcard rejected", cidrs: []string{"10.0.*"}, wantErr: true},
		{name: "ipv6 rejected", cidrs: []string{"2001:db8::/32"}, wantErr: true},
		{name: "missing prefix length", cidrs: []string{"10.0.0.0"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIPv4CIDRs(tt.cidrs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseIPv4CIDRs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			gotStrings := make([]string, 0, len(got))
			for _, n := range got {
				gotStrings = append(gotStrings, n.String())
			}
			if !reflect.DeepEqual(gotStrings, tt.want) {
				t.Errorf("ParseIPv4CIDRs() = %v, want %v", gotStrings, tt.want)
			}
		})
	}
}

// TestCIDRsOverlap tests the CIDRsOverlap function.
func TestCIDRsOverlap(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "equal", a: "10.0.1.0/24", b: "10.0.1.0/24", want: true},
		{name: "a contains b", a: "10.0.0.0/16", b: "10.0.1.0/24", want: true},
		{name: "b contains a", a: "10.0.1.0/28", b: "10.0.1.0/24", want: true},
		{name: "disjoint", a: "10.121.224.0/20", b: "100.64.0.0/16", want: false},
		{name: "adjacent", a: "10.0.0.0/24", b: "10.0.1.0/24", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, a, _ := net.ParseCIDR(tt.a)
			_, b, _ := net.ParseCIDR(tt.b)
			if got := CIDRsOverlap(a, b); got != tt.want {
				t.Errorf("CIDRsOverlap(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestStringInSlice tests the StringInSlice function.
func TestStringInSlice(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		slice []string
		want  bool
	}{
		{
			name:  "empty",
			s:     "value",
			slice: []string{},
			want:  false,
		},
		{
			name:  "one value",
			s:     "value",
			slice: []string{"value"},
			want:  true,
		},
		{
			name:  "two values",
			s:     "value",
			slice: []string{"value", "value2"},
			want:  true,
		},
		{
			name:  "two but one not found",
			s:     "value",
			slice: []string{"value2", "value3"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringInSlice(tt.s, tt.slice); got != tt.want {
				t.Errorf("StringInSlice()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestStringSliceToString tests the StringSliceToString function.
func TestStringSliceToString(t *testing.T) {
	tests := []struct {
		name string
		s    []string
		sep  string
		want string
	}{
		{
			name: "empty",
			s:    []string{},
			sep:  ",",
			want: "",
		},
		{
			name: "one value",
			s:    []string{"value"},
			sep:  ",",
			want: "value",
		},
		{
			name: "two values",
			s:    []string{"value", "value2"},
			sep:  ",",
			want: "value,value2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringSliceToString(tt.s, tt.sep); got != tt.want {
				t.Errorf("StringSliceToString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestIPtoString tests the IPtoString function.
func TestIPtoString(t *testing.T) {
	tests := []struct {
		name string
		i    []net.IP
		want []string
	}{
		{
			name: "empty",
			i:    []net.IP{},
			want: []string{},
		},
		{
			name: "one ip",
			i:    []net.IP{net.ParseIP("172.16.0.1")},
			want: []string{"172.16.0.1"},
		},
		{
			name: "two ips",
			i:    []net.IP{net.ParseIP("172.16.0.1"), net.ParseIP("172.17.1.254")},
			want: []string{"172.16.0.1", "172.17.1.254"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IPtoString(tt.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IPtoString()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestStructToFilters tests the StructToFilters function.
func TestStructToFilters(t *testing.T) {
	for _, tt := range getStructToFiltersCases() {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StructToFilters(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("StructToFilters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StructToFilters()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestBaseResults_AddError checks that AddError appends to the errors already recorded.
func TestBaseResults_AddError(t *testing.T) {
	b := &BaseResults{Errors: []string{"first"}}
	b.AddError("second")
	if want := []string{"first", "second"}; !reflect.DeepEqual(b.GetErrors(), want) {
		t.Errorf("GetErrors() = %v, want %v", b.GetErrors(), want)
	}
}
