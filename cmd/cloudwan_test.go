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

	searchCloudWAN "github.com/dyegoe/awss/search/cloudwan"

	"github.com/spf13/cobra"
)

// TestCloudwanRoutesFilterFlags_coversStruct checks every cloudwanRoutesFilters field has a
// registered flag in the filter flag list.
func TestCloudwanRoutesFilterFlags_coversStruct(t *testing.T) {
	resetCLI(t, nil)
	checkFilterFlags(t, "cloudwanRoutes", cloudwanRoutesCmd,
		reflect.TypeOf(cloudwanRoutesFilters{}).NumField(), cloudwanRoutesFilterFlags)
}

// TestCloudwanAttachmentsFilterFlags_coversStruct checks every cloudwanAttachmentsFilters field
// has a registered flag in the filter flag list.
func TestCloudwanAttachmentsFilterFlags_coversStruct(t *testing.T) {
	resetCLI(t, nil)
	checkFilterFlags(t, "cloudwanAttachments", cloudwanAttachmentsCmd,
		reflect.TypeOf(cloudwanAttachmentsFilters{}).NumField(), cloudwanAttachmentsFilterFlags)
}

// TestCloudwanBGPFilterFlags_coversStruct checks every cloudwanBGPFilters field has a registered
// flag in the filter flag list.
func TestCloudwanBGPFilterFlags_coversStruct(t *testing.T) {
	resetCLI(t, nil)
	checkFilterFlags(t, "cloudwanBGP", cloudwanBGPCmd, reflect.TypeOf(cloudwanBGPFilters{}).NumField(),
		cloudwanBGPFilterFlags)
}

// TestExecute_cloudwanBGP checks the filters every cloudwan bgp flag sends to the search, and the
// invalid values rejected before any search.
func TestExecute_cloudwanBGP(t *testing.T) {
	runFilterCases(t, "cloudwan-bgp", "segment", []filterCase{
		{name: "no filter lists every route", args: []string{"cloudwan", "bgp"}, wantFilters: map[string][]string{}},
		{
			name: "every filter",
			args: []string{
				"cloudwan", "bgp", "-c", "10.0.0.0/8", "-m", "exact", "-s", "prod", "-e", "eu-*",
				"--next-hop-types", "dx", "--communities", "65000:100", "--asns", "65010,65020",
				"--installed", "none", "--core-network-ids", "cn-1",
			},
			wantFilters: map[string][]string{
				"cidr": {"10.0.0.0/8"}, "match": {"exact"}, "segment": {"prod"}, "edge": {"eu-*"},
				"target-type": {"dx"}, "community": {"65000:100"}, "asn": {"65010", "65020"},
				"installed": {"none"}, "core-network-id": {"cn-1"},
			},
		},
		{
			name: "--sort", args: []string{"cloudwan", "bgp", "--sort", "as-path"}, wantFilters: map[string][]string{},
			wantSort: "as-path",
		},
		{name: "bad asn", args: []string{"cloudwan", "bgp", "--asns", "AS65010"}, wantErr: "invalid AS number"},
		{name: "bad installed", args: []string{"cloudwan", "bgp", "--installed", "maybe"}, wantErr: "invalid installed"},
		{name: "bad cidr", args: []string{"cloudwan", "bgp", "-c", "x"}, wantErr: "invalid CIDR"},
		{name: "no function groups", args: []string{"cloudwan", "bgp", "--function-groups", "x"}, wantErr: "unknown flag"},
	})
}

// TestExecute_cloudwanRoutes checks the filters every cloudwan routes flag sends to the search,
// and the invalid values rejected before any search.
func TestExecute_cloudwanRoutes(t *testing.T) {
	runFilterCases(t, "cloudwan-routes", "segment", []filterCase{
		{name: "no filter lists every route", args: []string{"cloudwan", "routes"}, wantFilters: map[string][]string{}},
		{
			name: "--cidrs --match", args: []string{"cloudwan", "routes", "-c", "10.0.0.0/8,10.1.1.1", "-m", "longest"},
			wantFilters: map[string][]string{"cidr": {"10.0.0.0/8", "10.1.1.1"}, "match": {"longest"}},
		},
		{
			name: "route tables",
			args: []string{
				"cloudwan", "routes", "-s", "prod", "--function-groups", "insp", "-e", "eu-*", "--core-network-ids", "cn-1",
			},
			wantFilters: map[string][]string{
				"segment": {"prod"}, "function-group": {"insp"}, "edge": {"eu-*"}, "core-network-id": {"cn-1"},
			},
		},
		{
			name:        "types, states, targets",
			args:        []string{"cloudwan", "routes", "-T", "propagated", "--states", "active", "--target-types", "vpc"},
			wantFilters: map[string][]string{"type": {"propagated"}, "state": {"active"}, "target-type": {"vpc"}},
		},
		{
			name: "--sort", args: []string{"cloudwan", "routes", "--sort", "cidr"}, wantFilters: map[string][]string{},
			wantSort: "cidr",
		},
		{name: "bad cidr", args: []string{"cloudwan", "routes", "-c", "10.0.0.0/33"}, wantErr: "invalid CIDR"},
		{
			name: "two modes", args: []string{"cloudwan", "routes", "-m", "exact,longest"},
			wantErr: "--match takes one mode",
		},
		{name: "bad type", args: []string{"cloudwan", "routes", "-T", "dynamic"}, wantErr: "invalid route type"},
		{name: "bad sort field", args: []string{"cloudwan", "routes", "--sort", "target-edge"}, wantErr: "target-edge"},
		{name: "positional argument", args: []string{"cloudwan", "routes", "10.0.0.0/8"}, wantErr: "unknown command"},
	})
}

// TestExecute_cloudwanAttachments checks the filters every cloudwan attachments flag sends to the
// search, and the invalid values rejected before any search.
func TestExecute_cloudwanAttachments(t *testing.T) {
	runFilterCases(t, "cloudwan-attachments", "segment", []filterCase{
		{
			name: "no filter lists every attachment", args: []string{"cloudwan", "attachments"},
			wantFilters: map[string][]string{},
		},
		{
			name: "every filter",
			args: []string{
				"cloudwan", "attachments", "-i", "a1", "-n", "dc-*", "-T", "direct-connect-gateway", "-s", "dc",
				"-e", "eu-west-1", "--states", "available", "--core-network-ids", "cn-1",
			},
			wantFilters: map[string][]string{
				"id": {"a1"}, "name": {"dc-*"}, "type": {"direct-connect-gateway"}, "segment": {"dc"},
				"edge": {"eu-west-1"}, "state": {"available"}, "core-network-id": {"cn-1"},
			},
		},
		{name: "bad type", args: []string{"cloudwan", "attachments", "-T", "tgw"}, wantErr: "invalid attachment type"},
		{name: "bad name glob", args: []string{"cloudwan", "attachments", "-n", "[x"}, wantErr: "names"},
	})
}

// TestExecute_cloudwanRegions checks that the cloudwan commands search every profile once, in the
// global region whatever --regions says, with the account names for the Owner column.
func TestExecute_cloudwanRegions(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "routes",
			args: []string{"cloudwan", "routes", "--profiles", "dev,prod", "--regions", "eu-west-1,us-east-1"},
		},
		{
			name: "attachments",
			args: []string{"cloudwan", "attachments", "--profiles", "dev,prod", "--regions", "eu-west-1"},
		},
		{name: "bgp", args: []string{"cloudwan", "bgp", "--profiles", "dev,prod", "--regions", "eu-west-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)
			useAwsConfig(t, testAwsConfig)
			args := append([]string{"--config", writeConfig(t, "accounts:\n  \"111111111111\": network\n")}, tt.args...)

			if _, err := runCLI(t, args...); err != nil || len(*calls) != 1 {
				t.Fatalf("Execute(%v) error = %v, searches %d, want one search", args, err, len(*calls))
			}
			got := (*calls)[0]
			if !reflect.DeepEqual(got.profiles, []string{"dev", "prod"}) {
				t.Errorf("Execute(%v) profiles = %v, want [dev prod]", tt.args, got.profiles)
			}
			if want := []string{searchCloudWAN.Region}; !reflect.DeepEqual(got.regions, want) {
				t.Errorf("Execute(%v) regions = %v, want %v", tt.args, got.regions, want)
			}
			if got.opts.AccountNames["111111111111"] != "network" {
				t.Errorf("Execute(%v) AccountNames = %v, want 111111111111 named network", tt.args, got.opts.AccountNames)
			}
		})
	}
}

// TestEngineName checks the engine key of a command with and without the engine annotation.
func TestEngineName(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		want string
	}{
		{
			name: "annotated",
			cmd:  &cobra.Command{Use: "routes", Annotations: map[string]string{annotationEngine: "cloudwan-routes"}},
			want: "cloudwan-routes",
		},
		{name: "plain", cmd: &cobra.Command{Use: "vpc"}, want: "vpc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engineName(tt.cmd); got != tt.want {
				t.Errorf("engineName(%s) = %q, want %q", tt.cmd.Use, got, tt.want)
			}
		})
	}
}
