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

// Test_vpcFilters_cidrs checks that --cidrs maps to the association CIDR filter.
func Test_vpcFilters_cidrs(t *testing.T) {
	f := vpcFilters{CIDRs: []string{"10.0.0.0/16"}, IsDefault: []string{"false"}}
	got, err := common.StructToFilters(f)
	if err != nil {
		t.Fatalf("StructToFilters() error = %v", err)
	}
	want := map[string][]string{
		"cidr-block-association.cidr-block": {"10.0.0.0/16"},
		"is-default":                        {"false"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StructToFilters() = %#v, want %#v", got, want)
	}
}

// ensureVpcFlags registers the vpc flags once, since Execute() is not called in tests.
func ensureVpcFlags() {
	if vpcCmd.Flags().Lookup("all") == nil {
		vpcInitFlags()
	}
}

// Test_vpcFilterFlags_coversStruct checks every vpcFilters field has a flag listed for --all exclusivity.
func Test_vpcFilterFlags_coversStruct(t *testing.T) {
	ensureVpcFlags()
	checkFilterFlags(t, "vpc", vpcCmd, reflect.TypeOf(vpcFilters{}).NumField(), vpcFilterFlags)
}

// Test_vpcRunE_invalidCIDR checks that a malformed --cidrs value is rejected before searching.
func Test_vpcRunE_invalidCIDR(t *testing.T) {
	ensureVpcFlags()
	old := vpcF
	t.Cleanup(func() { vpcF = old })
	vpcF = vpcFilters{CIDRs: []string{"10.0.0.0"}}

	if err := vpcRunE(vpcCmd, nil); err == nil {
		t.Error("vpcRunE() error = nil, want invalid CIDR error")
	}
}
