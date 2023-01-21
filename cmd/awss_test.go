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

// Package cmd contains the persistent flags and the root command.
//
// The root command does not execute any further action but print Help().
// It contains the persistent flags and persistent pre-run function.
// The persistent flags are used by all the subcommands.
// The persistent pre-run function is executed before the subcommands and does sanity checks.
// The subcommands are in the subdirectories of the search engines and should be imported.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestInitialize(t *testing.T) {

}

func Test_initPersistentFlags(t *testing.T) {
	testCmd := &cobra.Command{}
	initPersistentFlags(testCmd)

	t.Run("has flags", func(t *testing.T) {
		if testCmd.PersistentFlags().HasFlags() == false {
			t.Error("initPersistentFlags() has no flags")
		}
	})

	tests := []struct {
		flag  string
		value string
	}{
		{"config", ""},
		{"profiles", "[default]"},
		{"regions", "[us-east-1]"},
		{"output", "table"},
		{"show-empty", "false"},
		{"show-tags", "false"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("has %s flag", tt.flag), func(t *testing.T) {
			if testCmd.PersistentFlags().Lookup(tt.flag) == nil {
				t.Errorf("initPersistentFlags() has no %s flag", tt.flag)
			}
		})
		t.Run(fmt.Sprintf("has %s flag with default value", tt.flag), func(t *testing.T) {
			if got := testCmd.PersistentFlags().Lookup(tt.flag).DefValue; got != tt.value {
				t.Errorf("initPersistentFlags() flag: %s has wrong default value: got %s, want %s", tt.flag, got, tt.value)
			}
		})
	}
}

func Test_initViperBind(t *testing.T) {
	testCmd := &cobra.Command{}
	initPersistentFlags(testCmd)

	t.Run("bind viper to initialized cobra flags", func(t *testing.T) {
		if err := initViperBind(testCmd); err != nil {
			t.Errorf("initViperBind() error: %v", err)
		}
	})

	testCmd.ResetFlags()

	t.Run("bind viper to uninitialized cobra flags", func(t *testing.T) {
		if err := initViperBind(testCmd); err == nil {
			t.Error("initViperBind() should return an error")
		}
	})
}

//nolint:funlen
func Test_initViperConfig(t *testing.T) {
	oldFilePathAbs := filepathAbs
	oldOsStat := osStat
	filepathAbs = func(path string) (string, error) {
		if path == "invalid-path" {
			return "", fmt.Errorf("invalid-path")
		}
		return filepath.Abs(path)
	}
	osStat = func(name string) (os.FileInfo, error) {
		if strings.HasSuffix(name, "invalid-os-stat") {
			return nil, fmt.Errorf("invalid-os-stat")
		}
		return os.Stat(name)
	}

	defer func() {
		filepathAbs = oldFilePathAbs
		osStat = oldOsStat
	}()

	type args struct {
		cfg string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "file path abs error",
			args:    args{cfg: "invalid-path"},
			wantErr: true,
		},
		{
			name:    "os stat error",
			args:    args{cfg: "invalid-os-stat"},
			wantErr: true,
		},
		{
			name:    "existent directory but no config file",
			args:    args{cfg: "testdata/dirNoConfig/"},
			wantErr: true,
		},
		{
			name:    "existent directory with config file",
			args:    args{cfg: "testdata/dirWithConfig/"},
			wantErr: false,
		},
		{
			name:    "non-existent-file",
			args:    args{cfg: "non-existent-file"},
			wantErr: true,
		},
		{
			name:    "non-dir/non-existent-file",
			args:    args{cfg: "non-dir/non-existent-file"},
			wantErr: true,
		},
		{
			name:    "existent file",
			args:    args{cfg: "testdata/dirNoConfig/another-config.yaml"},
			wantErr: false,
		},
		{
			name:    "existent file same directory without extension",
			args:    args{cfg: "test-file-current-directory"},
			wantErr: false,
		},
		{
			name:    "existent file same directory with extension",
			args:    args{cfg: "test-file-current-directory.yaml"},
			wantErr: false,
		},
		{
			name:    "empty",
			args:    args{cfg: ""},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			err := initViperConfig(tt.args.cfg)
			t.Logf("using '%v' config file", viper.ConfigFileUsed())
			if (err != nil) != tt.wantErr {
				t.Errorf("initViperConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}