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
		{name: "empty", args: args{tags: []string{}}, want: map[string][]string{}, wantErr: false},
		{name: "one", args: args{tags: []string{"key=value"}}, want: map[string][]string{"key": {"value"}}, wantErr: false},
		{name: "two", args: args{tags: []string{"key=value", "key2=value2"}},
			want: map[string][]string{"key": {"value"}, "key2": {"value2"}}, wantErr: false},
		{name: "one with two values", args: args{tags: []string{"key=value:value2"}},
			want: map[string][]string{"key": {"value", "value2"}}, wantErr: false},
		{name: "two with two values", args: args{tags: []string{"key=value:value2", "key2=value3:value4"}},
			want: map[string][]string{"key": {"value", "value2"}, "key2": {"value3", "value4"}}, wantErr: false},
		{name: "one with two values and one without", args: args{tags: []string{"key=value:value2", "key2"}},
			want: nil, wantErr: true},
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
		{name: "nil", args: args{s: nil}, want: ""},
		{name: "empty", args: args{s: &value1}, want: ""},
		{name: "value", args: args{s: &value2}, want: "value"},
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
		{name: "empty", args: args{s: value1}, want: &value1},
		{name: "value", args: args{s: value2}, want: &value2},
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
		{name: "empty", args: args{s: "value", slice: []string{}}, want: false},
		{name: "one", args: args{s: "value", slice: []string{"value"}}, want: true},
		{name: "two", args: args{s: "value", slice: []string{"value", "value2"}}, want: true},
		{name: "two not found", args: args{s: "value", slice: []string{"value2", "value3"}}, want: false},
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
		{name: "empty", args: args{s: []string{}, sep: ","}, want: ""},
		{name: "one", args: args{s: []string{"value"}, sep: ","}, want: "value"},
		{name: "two", args: args{s: []string{"value", "value2"}, sep: ","}, want: "value,value2"},
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
		{name: "empty", args: args{i: []net.IP{}}, want: []string{}},
		{name: "one", args: args{i: []net.IP{net.ParseIP("172.16.0.1")}}, want: []string{"172.16.0.1"}},
		{name: "two", args: args{i: []net.IP{net.ParseIP("172.16.0.1"), net.ParseIP("172.17.1.254")}}, want: []string{"172.16.0.1", "172.17.1.254"}},
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
	type args struct {
		s interface{}
	}

	type ec2Filters struct {
		Ids               []string `filter:"instance-id"`
		Names             []string `filter:"tag:Name"`
		Tags              []string `filter:"tag"`
		TagsKey           []string `filter:"tag-key"`
		InstanceTypes     []string `filter:"instance-type"`
		InstanceStates    []string `filter:"instance-state-name"`
		AvailabilityZones []string `filter:"availability-zone"`
		PrivateIPs        []net.IP `filter:"network-interface.addresses.private-ip-address"`
		PublicIPs         []net.IP `filter:"network-interface.addresses.association.public-ip"`
	}

	ec2F := ec2Filters{
		Ids:               []string{"i-1234567890abcdef0"},
		Names:             []string{"test"},
		Tags:              []string{"key=value"},
		TagsKey:           []string{"key2"},
		InstanceTypes:     []string{"t2.micro"},
		InstanceStates:    []string{"running"},
		AvailabilityZones: []string{"us-east-1a"},
		PrivateIPs:        []net.IP{net.ParseIP("172.16.0.1"), net.ParseIP("172.17.1.254")},
		PublicIPs:         []net.IP{net.ParseIP("52.28.19.20"), net.ParseIP("52.30.31.32")},
	}

	ec2FiltersWant := map[string][]string{
		"instance-id":         {"i-1234567890abcdef0"},
		"tag:Name":            {"test"},
		"tag":                 {"key=value"},
		"tag-key":             {"key2"},
		"instance-type":       {"t2.micro"},
		"instance-state-name": {"running"},
		"availability-zone":   {"us-east-1a"},
		"network-interface.addresses.private-ip-address":    {"172.16.0.1", "172.17.1.254"},
		"network-interface.addresses.association.public-ip": {"52.28.19.20", "52.30.31.32"},
	}

	tests := []struct {
		name    string
		args    args
		want    map[string][]string
		wantErr bool
	}{
		{name: "ec2Filters", args: args{s: ec2F}, want: ec2FiltersWant, wantErr: false},
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
