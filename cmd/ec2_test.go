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

// Test_ec2FilterFlags_coversStruct checks every ec2Filters field has a registered flag in the --all list.
func Test_ec2FilterFlags_coversStruct(t *testing.T) {
	if ec2Cmd.Flags().Lookup("all") == nil {
		ec2InitFlags()
	}
	checkFilterFlags(t, "ec2", ec2Cmd, reflect.TypeOf(ec2Filters{}).NumField(), ec2FilterFlags)
}

// Test_ec2Filters_cidrs checks --cidrs maps to the cidr pseudo-filter key.
func Test_ec2Filters_cidrs(t *testing.T) {
	got, err := common.StructToFilters(ec2Filters{CIDRs: []string{"10.0.1.0/24"}})
	if err != nil {
		t.Fatalf("StructToFilters() error = %v", err)
	}
	if want := map[string][]string{"cidr": {"10.0.1.0/24"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("StructToFilters() = %#v, want %#v", got, want)
	}
}

// Test_ec2RunE_invalidCIDR checks that a malformed --cidrs value is rejected before searching.
func Test_ec2RunE_invalidCIDR(t *testing.T) {
	old := ec2F
	t.Cleanup(func() { ec2F = old })
	ec2F = ec2Filters{CIDRs: []string{"10.0.1.0"}}

	if err := ec2RunE(ec2Cmd, nil); err == nil {
		t.Error("ec2RunE() error = nil, want invalid CIDR error")
	}
}

// Test_sortHelp_listsEveryField checks the --sort help text of each command names every valid sort field.
func Test_sortHelp_listsEveryField(t *testing.T) {
	for _, cmd := range []string{"ec2", "eni", "ebs", "vpc", "subnet", "s3"} {
		help := sortHelp(cmd, "things", "id")
		for _, name := range search.SortFieldNames(cmd) {
			if !strings.Contains(help, name) {
				t.Errorf("sortHelp(%q) = %q does not mention sort field %q", cmd, help, name)
			}
		}
	}
}
