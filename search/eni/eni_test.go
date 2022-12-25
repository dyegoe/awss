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

// Package eni contains the search for ENIs.
//
// It implements the common.Results interface
package eni

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
	InterfaceInfo:      *mockENIInfo1,
	PrivateIPAddresses: []string{"172.16.0.1", "172.16.0.2"},
	PublicIPAddresses:  []string{"51.52.53.54", "51.52.53.55"},
	Tags: map[string]string{
		"Name":        "instance-name-1",
		"Environment": "test",
	},
}

var mockDataRow2 = &dataRow{
	InterfaceInfo:      *mockENIInfo2,
	PrivateIPAddresses: []string{"172.16.1.1", "172.16.1.2"},
	PublicIPAddresses:  []string{"51.52.54.54", "51.52.54.55"},
	Tags: map[string]string{
		"Name":        "instance-name-2",
		"Environment": "prod",
	},
}

var mockENIInfo1 = &eniInfo{
	NetworkInterfaceID: "eni-1234567890abcdef0",
	InterfaceType:      "interface-type-1",
	AvailabilityZone:   "us-east-1a",
	Status:             "status-1",
	SubnetID:           "subnet-1234567890abcdef0",
	InstanceID:         "i-1234567890abcdef0",
	InstanceName:       "instance-name-1",
}

var mockENIInfo2 = &eniInfo{
	NetworkInterfaceID: "eni-1234567890abcdef1",
	InterfaceType:      "interface-type-2",
	AvailabilityZone:   "us-east-1b",
	Status:             "status-2",
	SubnetID:           "subnet-1234567890abcdef1",
	InstanceID:         "i-1234567890abcdef1",
	InstanceName:       "instance-name-2",
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
			want:    []interface{}{"Interface Info", "Private IPs", "Public IPs", "Tags"},
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
