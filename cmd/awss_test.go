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
	"testing"

	"github.com/spf13/cobra"
)

func Test_initPersistentFlags(t *testing.T) {
	testCmd := &cobra.Command{}
	initPersistentFlags(testCmd)

	if testCmd.PersistentFlags().HasFlags() == false {
		t.Error("initPersistentFlags() has no flags")
	}

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
		if testCmd.PersistentFlags().Lookup(tt.flag) == nil {
			t.Errorf("initPersistentFlags() has no %s flag", tt.flag)
		}
		if testCmd.PersistentFlags().Lookup(tt.flag).DefValue != tt.value {
			t.Errorf("initPersistentFlags() flag: %s has wrong default value: %s", tt.flag, tt.value)
		}
	}
}
