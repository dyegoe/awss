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
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestGetAwsProfiles checks that profiles come from the file the AWS SDK reads: AWS_CONFIG_FILE
// when it is set, ~/.aws/config otherwise.
func TestGetAwsProfiles(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".aws"), 0o700); err != nil {
		t.Fatal(err)
	}
	homeConfig := "[default]\nregion = us-east-1\n[profile home-only]\nregion = us-east-1\n"
	if err := os.WriteFile(filepath.Join(home, ".aws", "config"), []byte(homeConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		home       string
		configFile string
		want       []string
		wantErr    bool
	}{
		{
			name: "AWS_CONFIG_FILE replaces ~/.aws/config", home: home, configFile: "testdata/config",
			want: []string{"default", "profile1", "profile2"},
		},
		{
			name: "~/.aws/config when AWS_CONFIG_FILE is not set", home: home,
			want: []string{"default", "home-only"},
		},
		{
			name: "missing AWS_CONFIG_FILE is an error, with no fallback", home: home,
			configFile: filepath.Join(t.TempDir(), "missing"), wantErr: true,
		},
		{
			name: "missing ~/.aws/config is an error", home: t.TempDir(), wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.home)
			t.Setenv("AWS_CONFIG_FILE", tt.configFile)

			got, err := GetAwsProfiles()

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetAwsProfiles() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAwsProfiles()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// mockAwsProfiles makes ~/.aws/config contain the default and profile1 profiles for one test.
func mockAwsProfiles(t *testing.T) {
	t.Helper()
	old := getAwsProfilesFn
	t.Cleanup(func() { getAwsProfilesFn = old })
	getAwsProfilesFn = func() ([]string, error) {
		return []string{"default", "profile1"}, nil
	}
}

// TestCheckProfiles tests the CheckProfiles function without an all-profiles list.
func TestCheckProfiles(t *testing.T) {
	mockAwsProfiles(t)

	tests := []struct {
		name     string
		profiles []string
		want     []string
		wantErr  bool
	}{
		{name: "empty", profiles: []string{}, want: []string{""}},
		{name: "all falls back to ~/.aws/config", profiles: []string{"all"}, want: []string{"default", "profile1"}},
		{name: "default", profiles: []string{"default"}, want: []string{"default"}},
		{name: "default,profile1", profiles: []string{"default", "profile1"}, want: []string{"default", "profile1"}},
		{name: "default,profile1,profile2", profiles: []string{"default", "profile1", "profile2"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckProfiles(tt.profiles, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckProfiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CheckProfiles()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestCheckProfiles_allProfiles tests how the all-profiles list of the awss config changes `all`.
func TestCheckProfiles_allProfiles(t *testing.T) {
	mockAwsProfiles(t)

	tests := []struct {
		name        string
		profiles    []string
		allProfiles []string
		want        []string
		wantErr     bool
	}{
		{name: "all uses all-profiles", profiles: []string{"all"}, allProfiles: []string{"profile1"},
			want: []string{"profile1"}},
		{name: "unknown all-profiles entry", profiles: []string{"all"}, allProfiles: []string{"profile1", "typo"},
			wantErr: true},
		{name: "ignored without all", profiles: []string{"default"}, allProfiles: []string{"profile1"},
			want: []string{"default"}},
		{name: "ignored without profiles", profiles: []string{}, allProfiles: []string{"profile1"},
			want: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckProfiles(tt.profiles, tt.allProfiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckProfiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CheckProfiles()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestCheckRegions tests the CheckRegions function.
func TestCheckRegions(t *testing.T) {
	// save the original variable, defer the restore and mock the variable
	oldGetAwsRegionEnvFn := getAwsRegionEnvFn
	defer func() { getAwsRegionEnvFn = oldGetAwsRegionEnvFn }()
	getAwsRegionEnvFn = func() string { return "" }

	allRegions := []string{"us-east-1", "us-east-2"}

	tests := []struct {
		name    string
		regions []string
		want    []string
		wantErr bool
	}{
		{
			name:    "empty falls back to DefaultRegion",
			regions: []string{},
			want:    []string{DefaultRegion},
		},
		{
			name:    "us-east-1",
			regions: []string{"us-east-1"},
			want:    []string{"us-east-1"},
		},
		{
			name:    "us-east-2",
			regions: []string{"us-east-2"},
			want:    []string{"us-east-2"},
		},
		{
			name:    "us-east-1,us-east-2",
			regions: []string{"us-east-1", "us-east-2"},
			want:    []string{"us-east-1", "us-east-2"},
		},
		{
			name:    "us-east-1,us-east-2,us-west-1",
			regions: []string{"us-east-1", "us-east-2", "us-west-1"},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckRegions(tt.regions, allRegions)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckRegions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CheckRegions()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestCheckRegions_envFallback tests that CheckRegions uses AWS_REGION/AWS_DEFAULT_REGION,
// bypassing the allRegions validation, when no --regions flag is passed.
func TestCheckRegions_envFallback(t *testing.T) {
	oldGetAwsRegionEnvFn := getAwsRegionEnvFn
	defer func() { getAwsRegionEnvFn = oldGetAwsRegionEnvFn }()
	getAwsRegionEnvFn = func() string { return "af-south-1" }

	got, err := CheckRegions([]string{}, []string{"us-east-1", "us-east-2"})
	if err != nil {
		t.Fatalf("CheckRegions() unexpected error = %v", err)
	}
	want := []string{"af-south-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CheckRegions()\n%#v\nwant\n%#v", got, want)
	}
}

// TestCheckAvailabilityZones tests the CheckAvailabilityZones function.
func TestCheckAvailabilityZones(t *testing.T) {
	tests := []struct {
		name    string
		az      []string
		wantErr bool
	}{
		{
			name:    "empty",
			az:      []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckAvailabilityZones(tt.az); (err != nil) != tt.wantErr {
				t.Errorf("CheckAvailabilityZones() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestTagName tests the TagName function.
func TestTagName(t *testing.T) {
	tests := []struct {
		name string
		tags []types.Tag
		want string
	}{
		{
			name: "empty",
			tags: []types.Tag{},
			want: "",
		},
		{
			name: "tag:Name",
			tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String("value")}},
			want: "value",
		},
		{
			name: "tag:Environment",
			tags: []types.Tag{{Key: aws.String("Environment"), Value: aws.String("value")}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagName(tt.tags); got != tt.want {
				t.Errorf("TagName()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestTagsToMap tests the TagsToMap function.
func TestTagsToMap(t *testing.T) {
	tests := []struct {
		name string
		tags []types.Tag
		want map[string]string
	}{
		{
			name: "empty",
			tags: []types.Tag{},
			want: map[string]string{},
		},
		{
			name: "tag:Name",
			tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String("value")}},
			want: map[string]string{"Name": "value"},
		},
		{
			name: "tag:Name, tag:Environment",
			tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String("value")},
				{Key: aws.String("Environment"), Value: aws.String("value2")},
			},
			want: map[string]string{"Name": "value", "Environment": "value2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagsToMap(tt.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TagsToMap()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestS3TagsToMap tests the S3TagsToMap function.
func TestS3TagsToMap(t *testing.T) {
	tests := []struct {
		name string
		tags []s3types.Tag
		want map[string]string
	}{
		{name: "empty", tags: []s3types.Tag{}, want: map[string]string{}},
		{
			name: "two tags",
			tags: []s3types.Tag{
				{Key: aws.String("Name"), Value: aws.String("value")},
				{Key: aws.String("Environment"), Value: aws.String("value2")},
			},
			want: map[string]string{"Name": "value", "Environment": "value2"},
		},
		{
			name: "nil key skipped, nil value empty",
			tags: []s3types.Tag{{Value: aws.String("orphan")}, {Key: aws.String("Empty")}},
			want: map[string]string{"Empty": ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := S3TagsToMap(tt.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("S3TagsToMap()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestFilterNames tests the FilterNames function.
func TestFilterNames(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  []types.Filter
	}{
		{
			name:  "empty",
			names: []string{},
			want:  []types.Filter{},
		},
		{
			name:  "one",
			names: []string{"name"},
			want:  []types.Filter{{Name: aws.String("tag:Name"), Values: []string{"name"}}},
		},
		{
			name:  "two",
			names: []string{"name1", "name2"},
			want:  []types.Filter{{Name: aws.String("tag:Name"), Values: []string{"name1", "name2"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterNames(tt.names); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterNames()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestFilterTags tests the FilterTags function.
//
//nolint:funlen // one table of tag cases, kept together so the formats read side by side
func TestFilterTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		want    []types.Filter
		wantErr bool
	}{
		{
			name: "empty",
			tags: []string{},
			want: []types.Filter{},
		},
		{
			name: "key=value",
			tags: []string{"key=value"},
			want: []types.Filter{{Name: aws.String("tag:key"), Values: []string{"value"}}},
		},
		{
			name: "key=value,key2=value2",
			tags: []string{"key=value", "key2=value2"},
			want: []types.Filter{
				{Name: aws.String("tag:key"), Values: []string{"value"}},
				{Name: aws.String("tag:key2"), Values: []string{"value2"}},
			},
		},
		{
			name: "key=value:value2",
			tags: []string{"key=value:value2"},
			want: []types.Filter{{Name: aws.String("tag:key"), Values: []string{"value", "value2"}}},
		},
		{
			name: "key=value:value2,key2=value3:value4",
			tags: []string{"key=value:value2", "key2=value3:value4"},
			want: []types.Filter{
				{Name: aws.String("tag:key"), Values: []string{"value", "value2"}},
				{Name: aws.String("tag:key2"), Values: []string{"value3", "value4"}},
			},
		},
		{
			name:    "malformed tag without equals sign",
			tags:    []string{"key"},
			wantErr: true,
		},
		{
			name:    "mixed valid and malformed tags",
			tags:    []string{"key=value:value2", "key2"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterTags(tt.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterTags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, i := range got {
				for _, j := range tt.want {
					if *i.Name != *j.Name {
						continue
					}
					if !reflect.DeepEqual(i.Values, j.Values) {
						t.Errorf("FilterTags()\n%#v\nwant\n%#v", got, tt.want)
					}
				}
			}
		})
	}
}

// TestFilterAvailabilityZones tests the FilterAvailabilityZones function.
func TestFilterAvailabilityZones(t *testing.T) {
	tests := []struct {
		name              string
		availabilityZones []string
		region            string
		want              []types.Filter
	}{
		{
			name:              "empty",
			availabilityZones: []string{},
			region:            "us-east-1",
			want:              []types.Filter{},
		},
		{
			name:              "AZ: a",
			availabilityZones: []string{"a"},
			region:            "us-east-1",
			want: []types.Filter{{Name: aws.String("availability-zone"),
				Values: []string{"us-east-1a"}}},
		},
		{
			name:              "AZ: a,b,c,d,e,f,g",
			availabilityZones: []string{"a", "b", "c", "d", "e", "f", "g"},
			region:            "us-east-1",
			want: []types.Filter{{Name: aws.String("availability-zone"),
				Values: []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d", "us-east-1e", "us-east-1f"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterAvailabilityZones(tt.availabilityZones, tt.region); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterAvailabilityZones()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestFilterDefault tests the FilterDefault function.
func TestFilterDefault(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		values []string
		want   []types.Filter
	}{
		{
			name:   "empty",
			key:    "",
			values: []string{},
			want:   []types.Filter{},
		},
		{
			name:   "key",
			key:    "key",
			values: []string{},
			want:   []types.Filter{},
		},
		{
			name:   "key and value",
			key:    "key",
			values: []string{"value"},
			want:   []types.Filter{{Name: aws.String("key"), Values: []string{"value"}}},
		},
		{
			name:   "key and values",
			key:    "key",
			values: []string{"value1", "value2"},
			want:   []types.Filter{{Name: aws.String("key"), Values: []string{"value1", "value2"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterDefault(tt.key, tt.values); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterDefault()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}
