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
)

// TestTgwRoutesFilterFlags_coversStruct checks every tgwRoutesFilters field has a registered flag
// in tgwRoutesFilterFlags.
func TestTgwRoutesFilterFlags_coversStruct(t *testing.T) {
	resetCLI(t, nil)
	checkFilterFlags(t, "tgwRoutes", tgwRoutesCmd, reflect.TypeOf(tgwRoutesFilters{}).NumField(), tgwRoutesFilterFlags)
}

// TestExecute_tgwRoutes checks the filters every tgw routes flag sends to the search, and the
// invalid values rejected before any search.
func TestExecute_tgwRoutes(t *testing.T) {
	runFilterCases(t, "tgw-routes", "route-table", []filterCase{
		{name: "no filter lists every route", args: []string{"tgw", "routes"}, wantFilters: map[string][]string{}},
		{
			name: "every filter",
			args: []string{
				"tgw", "routes", "-c", "10.0.0.0/8", "-m", "longest", "--route-table-ids", "tgw-rtb-1",
				"--route-table-names", "prod-*", "--transit-gateway-ids", "tgw-1", "-T", "static", "--states", "active",
				"--attachment-types", "dx", "--attachment-ids", "tgw-attach-1", "--resource-ids", "vpc-1",
			},
			wantFilters: map[string][]string{
				"cidr": {"10.0.0.0/8"}, "match": {"longest"}, "route-table-id": {"tgw-rtb-1"},
				"route-table-name": {"prod-*"}, "transit-gateway-id": {"tgw-1"}, "type": {"static"},
				"state": {"active"}, "attachment-type": {"dx"}, "attachment-id": {"tgw-attach-1"},
				"resource-id": {"vpc-1"},
			},
		},
		{
			name: "--sort", args: []string{"tgw", "routes", "--sort", "cidr"}, wantFilters: map[string][]string{},
			wantSort: "cidr",
		},
		{name: "bad sort", args: []string{"tgw", "routes", "--sort", "nope"}, wantErr: "nope"},
		{name: "bad match", args: []string{"tgw", "routes", "-m", "near"}, wantErr: "invalid match"},
		{name: "bad type", args: []string{"tgw", "routes", "-T", "system"}, wantErr: "invalid route type"},
		{name: "bad state", args: []string{"tgw", "routes", "--states", "up"}, wantErr: "invalid route state"},
		{
			name: "bad attachment type", args: []string{"tgw", "routes", "--attachment-types", "tgw"},
			wantErr: "invalid attachment type",
		},
		{name: "bad cidr", args: []string{"tgw", "routes", "-c", "x"}, wantErr: "invalid CIDR"},
	})
}
