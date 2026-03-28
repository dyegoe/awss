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

// Package ebs contains the search for EBS volumes.
//
// It implements the common.Results interface.
package ebs

import (
	"reflect"
	"testing"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	type args struct {
		profile        string
		region         string
		filters        map[string][]string
		sortField      string
		noInstanceName bool
	}
	tests := []struct {
		name string
		args args
		want *Results
	}{
		{
			name: "TestNew",
			args: args{
				profile:   "default",
				region:    "us-east-1",
				filters:   map[string][]string{"volume-id": {"vol-1234567890abcdef0"}},
				sortField: "id",
			},
			want: &Results{
				BaseResults: common.BaseResults{
					Profile:   "default",
					Region:    "us-east-1",
					Errors:    []string{},
					SortField: "id",
				},
				Data:    []dataRow{},
				Filters: map[string][]string{"volume-id": {"vol-1234567890abcdef0"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.args.profile, tt.args.region, tt.args.filters, tt.args.sortField, tt.args.noInstanceName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

var mockResultsEmpty = &Results{
	BaseResults: common.BaseResults{
		Profile: "",
		Region:  "",
		Errors:  []string{},
	},
	Data:    []dataRow{},
	Filters: map[string][]string{},
}

var mockResults = &Results{
	BaseResults: common.BaseResults{
		Profile: "default",
		Region:  "us-east-1",
		Errors: []string{
			"error1",
			"error2",
		},
		SortField: "id",
	},
	Data: []dataRow{
		*mockDataRow1,
		*mockDataRow2,
	},
	Filters: map[string][]string{
		"volume-id":         {"vol-1234567890abcdef0"},
		"tag":               {"key=value:value3", "key2=value2"},
		"availability-zone": {"a", "b"},
		"status":            {"available"},
	},
}

var mockDataRow1 = &dataRow{
	VolumeID:         "vol-1234567890abcdef0",
	Size:             100,
	VolumeType:       "gp3",
	State:            "in-use",
	AvailabilityZone: "us-east-1a",
	Encrypted:        "true",
	InstanceID:       "i-1234567890abcdef0",
	InstanceName:     "instance-name-1",
	Device:           "/dev/sda1",
	Tags: map[string]string{
		"Name":        "volume-1",
		"Environment": "test",
	},
}

var mockDataRow2 = &dataRow{
	VolumeID:         "vol-1234567890abcdef1",
	Size:             200,
	VolumeType:       "io2",
	State:            "available",
	AvailabilityZone: "us-east-1b",
	Encrypted:        "false",
	InstanceID:       "",
	InstanceName:     "",
	Device:           "",
	Tags: map[string]string{
		"Name":        "volume-2",
		"Environment": "prod",
	},
}

// TestResults_Len tests the Len function.
func TestResults_Len(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    int
	}{
		{
			name:    "TestResults_Len",
			results: mockResults,
			want:    2,
		},
		{
			name:    "TestResults_Len_Empty",
			results: mockResultsEmpty,
			want:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.Len(); got != tt.want {
				t.Errorf("Results.Len()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetProfile tests the GetProfile function.
func TestResults_GetProfile(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    string
	}{
		{
			name:    "TestResults_GetProfile",
			results: mockResults,
			want:    "default",
		},
		{
			name:    "TestResults_GetProfile_Empty",
			results: mockResultsEmpty,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetProfile(); got != tt.want {
				t.Errorf("Results.GetProfile()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetRegion tests the GetRegion function.
func TestResults_GetRegion(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    string
	}{
		{
			name:    "TestResults_GetRegion",
			results: mockResults,
			want:    "us-east-1",
		},
		{
			name:    "TestResults_GetRegion_Empty",
			results: mockResultsEmpty,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetRegion(); got != tt.want {
				t.Errorf("Results.GetRegion()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetErrors tests the GetErrors function.
func TestResults_GetErrors(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    []string
	}{
		{
			name:    "TestResults_GetErrors",
			results: mockResults,
			want:    []string{"error1", "error2"},
		},
		{
			name:    "TestResults_GetErrors_Empty",
			results: mockResultsEmpty,
			want:    []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetErrors(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Results.GetErrors()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetSortField tests the GetSortField function.
func TestResults_GetSortField(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    string
	}{
		{
			name:    "TestResults_GetSortField",
			results: mockResults,
			want:    "id",
		},
		{
			name:    "TestResults_GetSortField_Empty",
			results: mockResultsEmpty,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetSortField(); got != tt.want {
				t.Errorf("Results.GetSortField()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetHeaders tests the GetHeaders function.
func TestResults_GetHeaders(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    []interface{}
	}{
		{
			name:    "TestResults_GetHeaders",
			results: mockResults,
			want: []interface{}{
				"ID", "Size (GiB)", "Type", "State", "AZ",
				"Encrypted", "Instance ID", "Instance Name", "Device", "Tags",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetHeaders(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Results.GetHeaders()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetRows tests the GetRows method.
func TestResults_GetRows(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    []interface{}
	}{
		{
			name:    "TestResults_GetRows",
			results: mockResults,
			want: []interface{}{
				*mockDataRow1,
				*mockDataRow2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetRows(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Results.GetRows()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_getFilters tests the getFilters function.
func TestResults_getFilters(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    *ec2.DescribeVolumesInput
	}{
		{
			name:    "multiple filters",
			results: mockResults,
			want: &ec2.DescribeVolumesInput{
				VolumeIds: []string{"vol-1234567890abcdef0"},
				Filters: []types.Filter{
					{Name: common.String("tag:key"), Values: []string{"value", "value3"}},
					{Name: common.String("tag:key2"), Values: []string{"value2"}},
					{Name: common.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b"}},
					{Name: common.String("status"), Values: []string{"available"}},
				},
			},
		},
		{
			name: "empty filters",
			results: &Results{
				BaseResults: common.BaseResults{},
				Data:        []dataRow{},
				Filters:     map[string][]string{},
			},
			want: &ec2.DescribeVolumesInput{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.results.getFilters()
			if err != nil {
				t.Fatalf("Results.getFilters() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got.VolumeIds, tt.want.VolumeIds) {
				t.Errorf("VolumeIds = %v, want %v", got.VolumeIds, tt.want.VolumeIds)
			}
			if len(got.Filters) != len(tt.want.Filters) {
				t.Fatalf("Filters count = %d, want %d", len(got.Filters), len(tt.want.Filters))
			}
			gotByName := make(map[string][]string, len(got.Filters))
			for _, f := range got.Filters {
				gotByName[*f.Name] = f.Values
			}
			for _, wf := range tt.want.Filters {
				gv, ok := gotByName[*wf.Name]
				if !ok {
					t.Errorf("missing filter %q", *wf.Name)
					continue
				}
				if !reflect.DeepEqual(gv, wf.Values) {
					t.Errorf("filter %q values = %v, want %v", *wf.Name, gv, wf.Values)
				}
			}
		})
	}
}

// TestResults_getFilters_malformedTag tests getFilters with a malformed tag filter.
func TestResults_getFilters_malformedTag(t *testing.T) {
	r := &Results{
		BaseResults: common.BaseResults{},
		Data:        []dataRow{},
		Filters:     map[string][]string{"tag": {"invalid"}},
	}
	_, err := r.getFilters()
	if err == nil {
		t.Error("Results.getFilters() expected error for malformed tag, got nil")
	}
}

// TestResults_sortResults tests the sortResults function.
func TestResults_sortResults(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		wantErr   bool
		wantFirst string // expected VolumeID of first row after sort
	}{
		{
			name:      "sort by id ascending",
			field:     "id",
			wantFirst: "vol-1234567890abcdef0",
		},
		{
			name:      "sort by size ascending (numeric)",
			field:     "size",
			wantFirst: "vol-1234567890abcdef0",
		},
		{
			name:      "sort by state ascending",
			field:     "state",
			wantFirst: "vol-1234567890abcdef1",
		},
		{
			name:    "invalid field",
			field:   "invalid",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Results{
				Data: []dataRow{
					{VolumeID: "vol-1234567890abcdef1", Size: 200, State: "available"},
					{VolumeID: "vol-1234567890abcdef0", Size: 100, State: "in-use"},
				},
			}
			err := r.sortResults(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("sortResults(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
				return
			}
			if !tt.wantErr && r.Data[0].VolumeID != tt.wantFirst {
				t.Errorf("sortResults(%q) first row = %s, want %s", tt.field, r.Data[0].VolumeID, tt.wantFirst)
			}
		})
	}
}

// TestResults_sortResults_sizeNumeric verifies size sorts numerically, not lexicographically.
func TestResults_sortResults_sizeNumeric(t *testing.T) {
	r := &Results{
		Data: []dataRow{
			{VolumeID: "vol-a", Size: 100},
			{VolumeID: "vol-b", Size: 20},
			{VolumeID: "vol-c", Size: 1000},
		},
	}
	if err := r.sortResults("size"); err != nil {
		t.Fatalf("sortResults(size) unexpected error: %v", err)
	}
	want := []int32{20, 100, 1000}
	for i, w := range want {
		if r.Data[i].Size != w {
			t.Errorf("sortResults(size) index %d = %d, want %d", i, r.Data[i].Size, w)
		}
	}
}

// TestGetSortFields tests the GetSortFields function.
func TestGetSortFields(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		wantErr bool
	}{
		{name: "valid sort field id", field: "id"},
		{name: "valid sort field size", field: "size"},
		{name: "valid sort field type", field: "type"},
		{name: "valid sort field state", field: "state"},
		{name: "valid sort field az", field: "az"},
		{name: "valid sort field encrypted", field: "encrypted"},
		{name: "valid sort field instance-id", field: "instance-id"},
		{name: "valid sort field instance-name", field: "instance-name"},
		{name: "valid sort field device", field: "device"},
		{name: "invalid sort field", field: "invalid", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetSortFields(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSortFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
