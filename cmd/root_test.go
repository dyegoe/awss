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
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// checkFilterFlags checks that a command's --all exclusivity list has one entry per filter struct
// field and that every listed flag is registered on the command.
func checkFilterFlags(t *testing.T, name string, cmd *cobra.Command, numFields int, flags []string) {
	t.Helper()
	if numFields != len(flags) {
		t.Errorf("%sFilters has %d fields but %sFilterFlags lists %d flags", name, numFields, name, len(flags))
	}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("%sFilterFlags lists %q but the %s command has no such flag", name, flag, name)
		}
	}
}

// Test_initConfig tests the initConfig function.
func Test_initConfig(t *testing.T) {
	type args struct {
		cfg string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{cfg: ""},
			wantErr: false,
		},
		{
			name:    "non-existent",
			args:    args{cfg: "non-existent"},
			wantErr: true,
		},
		{
			name:    "existent",
			args:    args{cfg: "testdata/config.yaml"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := initConfig(tt.args.cfg); (err != nil) != tt.wantErr {
				t.Errorf("initConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test_parseTimeout tests parseTimeout with flag and config file values.
func Test_parseTimeout(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    time.Duration
		wantErr bool
	}{
		{name: "flag default", value: "5m0s", want: 5 * time.Minute},
		{name: "seconds", value: "90s", want: 90 * time.Second},
		{name: "duration value", value: 2 * time.Minute, want: 2 * time.Minute},
		{name: "zero disables", value: "0", want: 0},
		{name: "zero from yaml int", value: 0, want: 0},
		{name: "bare number from yaml", value: 300, wantErr: true},
		{name: "negative", value: "-1s", wantErr: true},
		{name: "garbage", value: "soon", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeout(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTimeout(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseTimeout(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
