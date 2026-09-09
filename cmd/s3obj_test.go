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
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Test_s3objFilterFlags_coversStruct checks every s3objFilters field has a registered flag.
func Test_s3objFilterFlags_coversStruct(t *testing.T) {
	if s3objCmd.Flags().Lookup(flagBuckets) == nil {
		s3objInitFlags()
	}
	checkFilterFlags(t, "s3obj", s3objCmd, reflect.TypeOf(s3objFilters{}).NumField(), s3objFilterFlags)
}

// Test_s3objRunE_validation checks --buckets is required and bad patterns are rejected before searching.
func Test_s3objRunE_validation(t *testing.T) {
	old := s3objF
	t.Cleanup(func() { s3objF = old; viper.Set(labelS3objRegex, false) })

	s3objF = s3objFilters{Keys: []string{"*.gz"}}
	err := s3objRunE(s3objCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--buckets is required") {
		t.Errorf("s3objRunE() without buckets: error = %v, want --buckets is required", err)
	}

	s3objF = s3objFilters{Buckets: []string{"b"}, Keys: []string{"[x"}}
	if err := s3objRunE(s3objCmd, nil); err == nil {
		t.Error("s3objRunE() with bad glob: error = nil, want error")
	}

	viper.Set(labelS3objRegex, true)
	s3objF = s3objFilters{Buckets: []string{"b"}, Keys: []string{"(x"}}
	if err := s3objRunE(s3objCmd, nil); err == nil {
		t.Error("s3objRunE() with bad regex: error = nil, want error")
	}
}

// Test_intLabel checks the empty label shortcut and the viper lookup.
func Test_intLabel(t *testing.T) {
	if got := intLabel(""); got != 0 {
		t.Errorf("intLabel(\"\") = %d, want 0", got)
	}
	viper.Set("test.int", 7)
	t.Cleanup(func() { viper.Set("test.int", nil) })
	if got := intLabel("test.int"); got != 7 {
		t.Errorf("intLabel(test.int) = %d, want 7", got)
	}
}
