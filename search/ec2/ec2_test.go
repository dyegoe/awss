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

// Package ec2 contains the EC2 search functions.
//
// It implements the common.Results interface
package ec2

import (
	"reflect"
	"testing"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	type args struct {
		profile   string
		region    string
		filters   map[string][]string
		sortField string
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
				filters:   map[string][]string{"tag:Name": {"test"}},
				sortField: "id",
			},
			want: &Results{
				Profile:   "default",
				Region:    "us-east-1",
				Errors:    []string{},
				Data:      []dataRow{},
				Filters:   map[string][]string{"tag:Name": {"test"}},
				SortField: "id",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.args.profile, tt.args.region, tt.args.filters, tt.args.sortField)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

var mockResultsEmpty = &Results{
	Profile:   "",
	Region:    "",
	Errors:    []string{},
	Data:      []dataRow{},
	Filters:   map[string][]string{},
	SortField: "",
}

var mockResults = &Results{
	Profile: "default",
	Region:  "us-east-1",
	Errors: []string{
		"error1",
		"error2",
	},
	Data: []dataRow{
		*mockDataRow1,
		*mockDataRow2,
	},
	Filters:   map[string][]string{"tag:Name": {"test"}},
	SortField: "id",
}

var mockDataRow1 = &dataRow{
	InstanceID:        "i-1234567890abcdef0",
	InstanceName:      "instance-name-1",
	InstanceType:      "t3.micro",
	AvailabilityZone:  "us-east-1a",
	InstanceState:     "running",
	PrivateIPAddress:  "172.16.0.1",
	PublicIPAddress:   "52.53.54.55",
	NetworkInterfaces: []string{"eni-1234567890abcdef0"},
	Tags: map[string]string{
		"Name":        "instance-name-1",
		"Environment": "test",
	},
}

var mockDataRow2 = &dataRow{
	InstanceID:        "i-1234567890abcdef1",
	InstanceName:      "instance-name-2",
	InstanceType:      "t3.medium",
	AvailabilityZone:  "us-east-1b",
	InstanceState:     "running",
	PrivateIPAddress:  "172.16.0.2",
	PublicIPAddress:   "52.53.54.56",
	NetworkInterfaces: []string{"eni-1234567890abcdef1"},
	Tags: map[string]string{
		"Name":        "instance-name-2",
		"Environment": "prod",
	},
}

// // TestResults_Search tests the Search function.
// func TestResults_Search(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		results *Results
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			tt.results.Search()
// 		})
// 	}
// }

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
				t.Errorf("Results.Len() = %#v, want %#v", got, tt.want)
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
				t.Errorf("Results.GetProfile() = %#v, want %#v", got, tt.want)
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
				t.Errorf("Results.GetRegion() = %#v, want %#v", got, tt.want)
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
				t.Errorf("Results.GetErrors() = %#v, want %#v", got, tt.want)
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
				t.Errorf("Results.GetSortField() = %#v, want %#v", got, tt.want)
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
			want:    []interface{}{"ID", "Name", "Type", "AZ", "State", "Private IP", "Public IP", "ENIs", "Tags"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetHeaders(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Results.GetHeaders() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetRows tests the GetRows method
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
				t.Errorf("Results.GetRows() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// // TestResults_getFilters tests the getFilters function.
// func TestResults_getFilters(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		results *Results
// 		want    *ec2.DescribeInstancesInput
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := tt.results.getFilters(); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("Results.getFilters() = %#v, want %#v", got, tt.want)
// 			}
// 		})
// 	}
// }

// // TestResults_sortResults tests the sortResults function.
// func TestResults_sortResults(t *testing.T) {
// 	type args struct {
// 		field string
// 	}
// 	tests := []struct {
// 		name    string
// 		results *Results
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if err := tt.results.sortResults(tt.args.field); (err != nil) != tt.wantErr {
// 				t.Errorf("Results.sortResults() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

func TestGetSortFields(t *testing.T) {
	type args struct {
		f string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "TestGetSortFields",
			args: args{
				f: "id",
			},
			want: map[string]string{
				"id":         "InstanceID",
				"name":       "InstanceName",
				"type":       "InstanceType",
				"az":         "AvailabilityZone",
				"state":      "InstanceState",
				"private-ip": "PrivateIPAddress",
				"public-ip":  "PublicIPAddress",
				"enis":       "NetworkInterfaces",
			},
			wantErr: false,
		},
		{
			name: "TestGetSortFields",
			args: args{
				f: "invalid",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSortFields(tt.args.f)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSortFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSortFields() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// // TestSearchInstanceName tests the SearchInstanceName function.
// func TestSearchInstanceName(t *testing.T) {
// 	type args struct {
// 		profile    string
// 		region     string
// 		instanceID string
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		want    string
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := SearchInstanceName(tt.args.profile, tt.args.region, tt.args.instanceID)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("SearchInstanceName() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if got != tt.want {
// 				t.Errorf("SearchInstanceName() = %#v, want %#v", got, tt.want)
// 			}
// 		})
// 	}
// }
