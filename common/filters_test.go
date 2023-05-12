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

// Package common contains the common code for awss.
package common

import (
	"reflect"
	"testing"
)

func TestEmptyFilterFieldError_Error(t *testing.T) {
	e := &EmptyFilterFieldError{Field: "flag"}
	want := "filter flag cannot be empty"
	if e.Error() != want {
		t.Errorf("EmptyFilterFieldError.Error() = %v, want %v", e.Error(), want)
	}
}

//nolint:funlen
func TestNewFilter(t *testing.T) {
	type args struct {
		flag       string
		shortflag  string
		filterType FilterType
		usage      string
		name       string
	}
	tests := []struct {
		name    string
		args    args
		want    Filter
		wantErr bool
	}{
		{
			name:    "TestNewFilter",
			args:    args{flag: "flag", shortflag: "s", filterType: FilterString, usage: "usage", name: "name"},
			want:    Filter{flag: "flag", shortflag: "s", values: []string{}, usage: "usage", name: "name"},
			wantErr: false,
		},
		{
			name:    "TestNewFilterEmptyFlag",
			args:    args{flag: "", shortflag: "s", filterType: FilterString, usage: "usage", name: "name"},
			want:    Filter{},
			wantErr: true,
		},
		{
			name:    "TestNewFilterEmptyShortFlag",
			args:    args{flag: "flag", shortflag: "", filterType: FilterString, usage: "usage", name: "name"},
			want:    Filter{},
			wantErr: true,
		},
		{
			name:    "TestNewFilterWrongFilterType",
			args:    args{flag: "flag", shortflag: "s", filterType: 2, usage: "usage", name: "name"},
			want:    Filter{},
			wantErr: true,
		},
		// {
		// 	name: "TestNewFilterEmptyUsage",
		// 	args: args{
		// 		flag:      "flag",
		// 		shortflag: "s",
		// 		filterType: FilterString,
		// 		usage:     "",
		// 		name:      "name",
		// 	},
		// 	want:    Filter{},
		// 	wantErr: true,
		// },
		// {
		// 	name: "TestNewFilterEmptyName",
		// 	args: args{
		// 		flag:      "flag",
		// 		shortflag: "s",
		// 		filterType: FilterString,
		// 		usage:     "usage",
		// 		name:      "",
		// 	},
		// 	want:    Filter{},
		// 	wantErr: true,
		// },
		// {
		// 	name: "TestNewFilterEmptyAll",
		// 	args: args{
		// 		flag:      "",
		// 		shortflag: "",
		// 		filterType: ,     "",
		// 		usage:     "",
		// 		name:      "",
		// 	},
		// 	want:    Filter{},
		// 	wantErr: true,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewFilter(tt.args.flag, tt.args.shortflag, tt.args.filterType, tt.args.usage, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Flag() != tt.want.Flag() {
				t.Errorf("NewFilter() got = %v, want %v", got, tt.want)
			}
			if got.ShortFlag() != tt.want.ShortFlag() {
				t.Errorf("NewFilter() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got.Values(), tt.want.Values()) {
				t.Errorf("NewFilter() got = %v, want %v", got, tt.want)
			}
			if got.Usage() != tt.want.Usage() {
				t.Errorf("NewFilter() got = %v, want %v", got, tt.want)
			}
			if got.Name() != tt.want.Name() {
				t.Errorf("NewFilter() got = %v, want %v", got, tt.want)
			}
		})
	}
}
