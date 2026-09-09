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

	"github.com/dyegoe/awss/common"
)

// Test_subnetFilters_mapping checks the struct tags map flags to the AWS filter names.
func Test_subnetFilters_mapping(t *testing.T) {
	f := subnetFilters{
		CIDRs:       []string{"10.0.1.0/24"},
		VpcIDs:      []string{"vpc-1"},
		MapPublicIP: []string{"true"},
	}
	got, err := common.StructToFilters(f)
	if err != nil {
		t.Fatalf("StructToFilters() error = %v", err)
	}
	want := map[string][]string{
		"cidr-block":              {"10.0.1.0/24"},
		"vpc-id":                  {"vpc-1"},
		"map-public-ip-on-launch": {"true"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StructToFilters() = %#v, want %#v", got, want)
	}
}

// Test_subnetFilterFlags_coversStruct checks every subnetFilters field has a registered flag in the --all list.
func Test_subnetFilterFlags_coversStruct(t *testing.T) {
	if subnetCmd.Flags().Lookup("all") == nil {
		subnetInitFlags()
	}
	checkFilterFlags(t, "subnet", subnetCmd, reflect.TypeOf(subnetFilters{}).NumField(), subnetFilterFlags)
}

// Test_subnetRunE_invalidCIDR checks that a malformed --cidrs value is rejected before searching.
func Test_subnetRunE_invalidCIDR(t *testing.T) {
	old := subnetF
	t.Cleanup(func() { subnetF = old })
	subnetF = subnetFilters{CIDRs: []string{"10.0.1.0/33"}}

	if err := subnetRunE(subnetCmd, nil); err == nil {
		t.Error("subnetRunE() error = nil, want invalid CIDR error")
	}
}
