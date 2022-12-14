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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestTagName(t *testing.T) {
	type args struct {
		tags []types.Tag
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{tags: []types.Tag{}},
			want: "",
		},
		{
			name: "tag:Name",
			args: args{tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String("value")}}},
			want: "value",
		},
		{
			name: "tag:Environment",
			args: args{tags: []types.Tag{{Key: aws.String("Environment"), Value: aws.String("value")}}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagName(tt.args.tags); got != tt.want {
				t.Errorf("TagName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagsToMap(t *testing.T) {
	type args struct {
		tags []types.Tag
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "empty",
			args: args{
				tags: []types.Tag{},
			},
			want: map[string]string{},
		},
		{
			name: "tag:Name",
			args: args{tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String("value")}}},
			want: map[string]string{"Name": "value"},
		},
		{
			name: "tag:Name, tag:Environment",
			args: args{
				tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("value")},
					{Key: aws.String("Environment"), Value: aws.String("value2")},
				},
			},
			want: map[string]string{"Name": "value", "Environment": "value2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagsToMap(tt.args.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TagsToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterNames(t *testing.T) {
	type args struct {
		names []string
	}
	tests := []struct {
		name string
		args args
		want []types.Filter
	}{
		{
			name: "empty",
			args: args{names: []string{}},
			want: []types.Filter{},
		},
		{
			name: "one",
			args: args{names: []string{"name"}},
			want: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{"name"}}},
		},
		{
			name: "two",
			args: args{names: []string{"name1", "name2"}},
			want: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{"name1", "name2"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterNames(tt.args.names); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterTags(t *testing.T) {
	type args struct {
		tags []string
	}
	tests := []struct {
		name string
		args args
		want []types.Filter
	}{
		{
			name: "empty",
			args: args{tags: []string{}},
			want: []types.Filter{},
		},
		{
			name: "key=value",
			args: args{tags: []string{"key=value"}},
			want: []types.Filter{{Name: aws.String("tag:key"), Values: []string{"value"}}},
		},
		{
			name: "key=value,key2=value2",
			args: args{tags: []string{"key=value", "key2=value2"}},
			want: []types.Filter{
				{Name: aws.String("tag:key"), Values: []string{"value"}},
				{Name: aws.String("tag:key2"), Values: []string{"value2"}},
			},
		},
		{
			name: "key=value:value2",
			args: args{tags: []string{"key=value:value2"}},
			want: []types.Filter{{Name: aws.String("tag:key"), Values: []string{"value", "value2"}}},
		},
		{
			name: "key=value:value2,key2=value3:value4",
			args: args{tags: []string{"key=value:value2", "key2=value3:value4"}},
			want: []types.Filter{
				{Name: aws.String("tag:key"), Values: []string{"value", "value2"}},
				{Name: aws.String("tag:key2"), Values: []string{"value3", "value4"}},
			},
		},
		{
			name: "key",
			args: args{tags: []string{"key"}},
			want: []types.Filter{},
		},
		{
			name: "key=value:value2,key2",
			args: args{tags: []string{"key=value:value2", "key2"}},
			want: []types.Filter{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterTags(tt.args.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterAvailabilityZones(t *testing.T) {
	type args struct {
		availabilityZones []string
		region            string
	}
	tests := []struct {
		name string
		args args
		want []types.Filter
	}{
		{
			name: "empty",
			args: args{availabilityZones: []string{}, region: "us-east-1"},
			want: []types.Filter{},
		},
		{
			name: "AZ: a",
			args: args{availabilityZones: []string{"a"}, region: "us-east-1"},
			want: []types.Filter{{Name: aws.String("availability-zone"), Values: []string{"us-east-1a"}}},
		},
		{
			name: "AZ: a,b",
			args: args{availabilityZones: []string{"a", "b"}, region: "us-east-1"},
			want: []types.Filter{{Name: aws.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b"}}},
		},
		{
			name: "AZ: a,b,c",
			args: args{availabilityZones: []string{"a", "b", "c"}, region: "us-east-1"},
			want: []types.Filter{{Name: aws.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b", "us-east-1c"}}},
		},
		{
			name: "AZ: a,b,c,d",
			args: args{availabilityZones: []string{"a", "b", "c", "d"}, region: "us-east-1"},
			want: []types.Filter{{Name: aws.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d"}}},
		},
		{
			name: "AZ: a,b,c,d,e",
			args: args{availabilityZones: []string{"a", "b", "c", "d", "e"}, region: "us-east-1"},
			want: []types.Filter{{Name: aws.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d", "us-east-1e"}}},
		},
		{
			name: "AZ: a,b,c,d,e,f",
			args: args{availabilityZones: []string{"a", "b", "c", "d", "e", "f"}, region: "us-east-1"},
			want: []types.Filter{{Name: aws.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d", "us-east-1e", "us-east-1f"}}},
		},
		{
			name: "AZ: a,b,c,d,e,f,g",
			args: args{availabilityZones: []string{"a", "b", "c", "d", "e", "f", "g"}, region: "us-east-1"},
			want: []types.Filter{{Name: aws.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d", "us-east-1e", "us-east-1f"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterAvailabilityZones(tt.args.availabilityZones, tt.args.region); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterAvailabilityZones() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterDefault(t *testing.T) {
	type args struct {
		key    string
		values []string
	}
	tests := []struct {
		name string
		args args
		want []types.Filter
	}{
		{
			name: "empty",
			args: args{key: "", values: []string{}},
			want: []types.Filter{},
		},
		{
			name: "key",
			args: args{key: "key", values: []string{}},
			want: []types.Filter{},
		},
		{
			name: "key and value",
			args: args{key: "key", values: []string{"value"}},
			want: []types.Filter{{Name: aws.String("key"), Values: []string{"value"}}},
		},
		{
			name: "key and values",
			args: args{key: "key", values: []string{"value1", "value2"}},
			want: []types.Filter{{Name: aws.String("key"), Values: []string{"value1", "value2"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterDefault(tt.args.key, tt.args.values); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
