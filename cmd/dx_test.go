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

// TestDxGatewaysFilterFlags_coversStruct checks every dxGatewaysFilters field has a registered
// flag in dxGatewaysFilterFlags.
func TestDxGatewaysFilterFlags_coversStruct(t *testing.T) {
	resetCLI(t, nil)
	checkFilterFlags(t, "dxGateways", dxGatewaysCmd, reflect.TypeOf(dxGatewaysFilters{}).NumField(),
		dxGatewaysFilterFlags)
}

// TestExecute_dxGateways checks the filters every dx gateways flag sends to the search, and the
// invalid values rejected before any search.
func TestExecute_dxGateways(t *testing.T) {
	runFilterCases(t, "dx-gateways", "gateway", []filterCase{
		{name: "no filter lists every gateway", args: []string{"dx", "gateways"}, wantFilters: map[string][]string{}},
		{
			name: "every filter",
			args: []string{
				"dx", "gateways", "-i", "gw-1", "-n", "dc-*", "-c", "10.0.0.0/8", "-m", "supernet-of",
				"--associated-types", "transit-gateway", "--associated-ids", "tgw-1", "--states", "associated",
			},
			wantFilters: map[string][]string{
				"id": {"gw-1"}, "name": {"dc-*"}, "cidr": {"10.0.0.0/8"}, "match": {"supernet-of"},
				"associated-type": {"transit-gateway"}, "associated-id": {"tgw-1"}, "state": {"associated"},
			},
		},
		{
			name: "--sort", args: []string{"dx", "gateways", "--sort", "prefix"}, wantFilters: map[string][]string{},
			wantSort: "prefix",
		},
		{name: "bad match", args: []string{"dx", "gateways", "-m", "longest"}, wantErr: "invalid match"},
		{name: "bad type", args: []string{"dx", "gateways", "--associated-types", "tgw"}, wantErr: "invalid associated type"},
		{name: "bad state", args: []string{"dx", "gateways", "--states", "up"}, wantErr: "invalid association state"},
		{name: "bad cidr", args: []string{"dx", "gateways", "-c", "x"}, wantErr: "invalid CIDR"},
	})
}
