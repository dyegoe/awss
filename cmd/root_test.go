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

// Package cmd enables the CLI commands and flags.
//
// It is based on Cobra and Viper.
package cmd

import (
	"reflect"
	"testing"
)

// // Test_initConfig tests the initConfig function.
// func Test_initConfig(t *testing.T) {
// 	type args struct {
// 		cfg string
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if err := initConfig(tt.args.cfg); (err != nil) != tt.wantErr {
// 				t.Errorf("initConfig() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// Test_checkProfiles tests the checkProfiles function.
func Test_checkProfiles(t *testing.T) {
	// save the original variable, defer the restore and mock the variable
	oldGetAwsProfiles := getAwsProfiles
	defer func() { getAwsProfiles = oldGetAwsProfiles }()
	getAwsProfiles = func() ([]string, error) {
		return []string{"default", "profile1"}, nil
	}

	type args struct {
		profiles []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{profiles: []string{}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "all",
			args:    args{profiles: []string{"all"}},
			want:    []string{"default", "profile1"},
			wantErr: false,
		},
		{
			name:    "default",
			args:    args{profiles: []string{"default"}},
			want:    []string{"default"},
			wantErr: false,
		},
		{
			name:    "default,profile1",
			args:    args{profiles: []string{"default", "profile1"}},
			want:    []string{"default", "profile1"},
			wantErr: false,
		},
		{
			name:    "default,profile1,profile2",
			args:    args{profiles: []string{"default", "profile1", "profile2"}},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkProfiles(tt.args.profiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkProfiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("checkProfiles() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// Test_checkRegions tests the checkRegions function.
func Test_checkRegions(t *testing.T) {
	allRegions := []string{"us-east-1", "us-east-2"}

	type args struct {
		regions    []string
		allRegions []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{regions: []string{}, allRegions: allRegions},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "us-east-1",
			args:    args{regions: []string{"us-east-1"}, allRegions: allRegions},
			want:    []string{"us-east-1"},
			wantErr: false,
		},
		{
			name:    "us-east-2",
			args:    args{regions: []string{"us-east-2"}, allRegions: allRegions},
			want:    []string{"us-east-2"},
			wantErr: false,
		},
		{
			name:    "us-east-1,us-east-2",
			args:    args{regions: []string{"us-east-1", "us-east-2"}, allRegions: allRegions},
			want:    []string{"us-east-1", "us-east-2"},
			wantErr: false,
		},
		{
			name:    "us-east-1,us-east-2,us-west-1",
			args:    args{regions: []string{"us-east-1", "us-east-2", "us-west-1"}, allRegions: allRegions},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkRegions(tt.args.regions, tt.args.allRegions)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkRegions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("checkRegions() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_checkAvailabilityZones tests the checkAvailabilityZones function.
func Test_checkAvailabilityZones(t *testing.T) {
	type args struct {
		az []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{az: []string{}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkAvailabilityZones(tt.args.az); (err != nil) != tt.wantErr {
				t.Errorf("checkAvailabilityZones() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
