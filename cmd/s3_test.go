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

package cmd

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

// Test_s3FilterFlags_coversStruct checks every s3Filters field has a registered flag in the --all list.
func Test_s3FilterFlags_coversStruct(t *testing.T) {
	if s3Cmd.Flags().Lookup("all") == nil {
		s3InitFlags()
	}
	checkFilterFlags(t, "s3", s3Cmd, reflect.TypeOf(s3Filters{}).NumField(), s3FilterFlags)
}

// Test_s3RunE_invalidPattern checks that a bad glob or regex is rejected before searching.
func Test_s3RunE_invalidPattern(t *testing.T) {
	old := s3F
	t.Cleanup(func() { s3F = old; viper.Set(labelS3Regex, false) })

	s3F = s3Filters{Names: []string{"[unclosed"}}
	viper.Set(labelS3Regex, false)
	if err := s3RunE(s3Cmd, nil); err == nil {
		t.Error("s3RunE() with bad glob: error = nil, want error")
	}

	s3F = s3Filters{Names: []string{"(unclosed"}}
	viper.Set(labelS3Regex, true)
	if err := s3RunE(s3Cmd, nil); err == nil {
		t.Error("s3RunE() with bad regex: error = nil, want error")
	}
}

// Test_boolLabel checks the empty label shortcut and the viper lookup.
func Test_boolLabel(t *testing.T) {
	if boolLabel("") {
		t.Error("boolLabel(\"\") = true, want false")
	}
	viper.Set("test.bool", true)
	t.Cleanup(func() { viper.Set("test.bool", nil) })
	if !boolLabel("test.bool") {
		t.Error("boolLabel(test.bool) = false, want true")
	}
}

// Test_subcommands_rejectPositionalArgs checks a stray argument (e.g. a pattern given to --regex) fails loudly.
func Test_subcommands_rejectPositionalArgs(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "completion" || c.Name() == "help" {
			continue
		}
		if err := c.ValidateArgs([]string{"^(prod|stag)-.*"}); err == nil {
			t.Errorf("command %q accepts positional arguments, want cobra.NoArgs", c.Name())
		}
	}
}
