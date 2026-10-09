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

// TestRdsClusterFilterFlags_coversStruct checks every rdsClusterFilters field has a registered
// flag in the --all list.
func TestRdsClusterFilterFlags_coversStruct(t *testing.T) {
	if rdsClusterCmd.Flags().Lookup(flagAll) == nil {
		rdsClusterInitFlags()
	}
	checkFilterFlags(t, "rds-cluster", rdsClusterCmd, reflect.TypeOf(rdsClusterFilters{}).NumField(),
		rdsClusterFilterFlags)
}

// TestExecute_rdsCluster checks the filters and options every rds-cluster flag sends to the
// search, and the invalid values rejected before any search.
func TestExecute_rdsCluster(t *testing.T) {
	runFilterCases(t, "rds-cluster", "id", []filterCase{
		{name: "--all", args: []string{"rds-cluster", "-a"}, wantFilters: map[string][]string{}},
		{name: "--ids", args: []string{"rds-cluster", "-i", "c1,c2"},
			wantFilters: map[string][]string{"db-cluster-id": {"c1", "c2"}}},
		{name: "--engines", args: []string{"rds-cluster", "-e", "aurora-mysql"},
			wantFilters: map[string][]string{"engine": {"aurora-mysql"}}},
		{name: "--names with --regex", args: []string{"rds-cluster", "-n", "-prd$", "--regex"},
			wantFilters: map[string][]string{"name": {"-prd$"}}, wantRegex: true},
		{name: "--engine-versions", args: []string{"rds-cluster", "-V", "8.0.*"},
			wantFilters: map[string][]string{"engine-version": {"8.0.*"}}},
		{name: "--tags", args: []string{"rds-cluster", "-t", "Env=prod"},
			wantFilters: map[string][]string{"tag": {"Env=prod"}}},
		{name: "--sort", args: []string{"rds-cluster", "-a", "--sort", "vpc"}, wantFilters: map[string][]string{},
			wantSort: "vpc"},
		{name: "no filter", args: []string{"rds-cluster"}, wantErr: "at least one filter"},
		{name: "--all with a filter", args: []string{"rds-cluster", "-a", "-i", "c1"}, wantErr: "--all cannot be combined"},
		{name: "no --clusters flag", args: []string{"rds-cluster", "-c", "c1"}, wantErr: "unknown shorthand flag: 'c'"},
		{name: "bad name regex", args: []string{"rds-cluster", "-n", "(", "--regex"}, wantErr: "invalid regular expression"},
		{name: "bad version glob", args: []string{"rds-cluster", "-V", "[8"}, wantErr: "engine versions: invalid pattern"},
		{name: "bad tag", args: []string{"rds-cluster", "-t", "Env"}, wantErr: "invalid tag format: Env"},
		{name: "bad sort field", args: []string{"rds-cluster", "-a", "--sort", "port"}, wantErr: "port"},
	})
}
