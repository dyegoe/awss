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

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"
)

// Test_ec2Filters_volumeIDs checks that --volume-ids maps to the AWS block-device-mapping.volume-id filter.
func Test_ec2Filters_volumeIDs(t *testing.T) {
	f := ec2Filters{VolumeIDs: []string{"vol-1", "vol-2"}}
	got, err := common.StructToFilters(f)
	if err != nil {
		t.Fatalf("StructToFilters() error = %v", err)
	}
	want := map[string][]string{"block-device-mapping.volume-id": {"vol-1", "vol-2"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StructToFilters() = %#v, want %#v", got, want)
	}
}

// Test_ec2FilterFlags_coversStruct checks every ec2Filters field has a flag listed for --all exclusivity.
func Test_ec2FilterFlags_coversStruct(t *testing.T) {
	if n := reflect.TypeOf(ec2Filters{}).NumField(); n != len(ec2FilterFlags) {
		t.Errorf("ec2Filters has %d fields but ec2FilterFlags lists %d flags", n, len(ec2FilterFlags))
	}
	if !common.StringInSlice("volume-ids", ec2FilterFlags) {
		t.Errorf("ec2FilterFlags must include volume-ids, got %v", ec2FilterFlags)
	}
}

// Test_sortHelp_listsEveryField checks the --sort help text of each command names every valid sort field.
func Test_sortHelp_listsEveryField(t *testing.T) {
	for _, cmd := range []string{"ec2", "eni", "ebs", "vpc"} {
		help := sortHelp(cmd, "things", "id")
		for _, name := range search.SortFieldNames(cmd) {
			if !strings.Contains(help, name) {
				t.Errorf("sortHelp(%q) = %q does not mention sort field %q", cmd, help, name)
			}
		}
	}
}
