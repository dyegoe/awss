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
)

// TestRdsFilterFlags_coversStruct checks every rdsFilters field has a registered flag in the --all list.
func TestRdsFilterFlags_coversStruct(t *testing.T) {
	if rdsCmd.Flags().Lookup(flagAll) == nil {
		rdsInitFlags()
	}
	checkFilterFlags(t, "rds", rdsCmd, reflect.TypeOf(rdsFilters{}).NumField(), rdsFilterFlags)
}

// filterCase is one run of a search command: the filters and options it must send, or the error
// that stops it before any search.
type filterCase struct {
	name        string
	args        []string
	wantFilters map[string][]string
	wantSort    string
	wantRegex   bool
	wantErr     string
}

// runFilterCases runs each case through the command tree and checks what reached the search.
// defaultSort is the sort field a case gets when it sets none.
func runFilterCases(t *testing.T, command, defaultSort string, tests []filterCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)

			_, err := runCLI(t, tt.args...)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || len(*calls) != 0 {
					t.Errorf("Execute(%v) error = %v, searches %d, want an error containing %q and no search",
						tt.args, err, len(*calls), tt.wantErr)
				}
				return
			}
			if err != nil || len(*calls) != 1 {
				t.Fatalf("Execute(%v) error = %v, searches %d, want one search", tt.args, err, len(*calls))
			}
			got := (*calls)[0]
			wantSort := tt.wantSort
			if wantSort == "" {
				wantSort = defaultSort
			}
			if got.cmd != command || !reflect.DeepEqual(got.filters, tt.wantFilters) ||
				got.opts.SortField != wantSort || got.opts.Regex != tt.wantRegex {
				t.Errorf("Execute(%v) = %s %v sort %q regex %v, want %s %v sort %q regex %v", tt.args,
					got.cmd, got.filters, got.opts.SortField, got.opts.Regex,
					command, tt.wantFilters, wantSort, tt.wantRegex)
			}
		})
	}
}

// TestExecute_rds checks the filters and options every rds flag sends to the search, and the
// invalid values rejected before any search.
func TestExecute_rds(t *testing.T) {
	runFilterCases(t, "rds", "id", []filterCase{
		{name: "--all", args: []string{"rds", "-a"}, wantFilters: map[string][]string{}},
		{name: "--ids", args: []string{"rds", "-i", "db-1,db-2"},
			wantFilters: map[string][]string{"db-instance-id": {"db-1", "db-2"}}},
		{name: "--engines", args: []string{"rds", "-e", "postgres"},
			wantFilters: map[string][]string{"engine": {"postgres"}}},
		{name: "--clusters", args: []string{"rds", "-c", "aurora-1"},
			wantFilters: map[string][]string{"db-cluster-id": {"aurora-1"}}},
		{name: "--names with --regex", args: []string{"rds", "-n", "^app-", "--regex"},
			wantFilters: map[string][]string{"name": {"^app-"}}, wantRegex: true},
		{name: "--engine-versions with --engines", args: []string{"rds", "-e", "postgres", "-V", "13*"},
			wantFilters: map[string][]string{"engine": {"postgres"}, "engine-version": {"13*"}}},
		{name: "--tags", args: []string{"rds", "-t", "Env=prod:stg,Team=app"},
			wantFilters: map[string][]string{"tag": {"Env=prod:stg", "Team=app"}}},
		{name: "--sort", args: []string{"rds", "-a", "--sort", "version"}, wantFilters: map[string][]string{},
			wantSort: "version"},
		{name: "no filter", args: []string{"rds"}, wantErr: "at least one filter"},
		{name: "--all with a filter", args: []string{"rds", "-a", "-e", "mysql"}, wantErr: "--all cannot be combined"},
		{name: "bad name glob", args: []string{"rds", "-n", "[a"}, wantErr: "invalid pattern"},
		{name: "bad version glob", args: []string{"rds", "-V", "[1"}, wantErr: "engine versions: invalid pattern"},
		{name: "bad tag", args: []string{"rds", "-t", "Env"}, wantErr: "invalid tag format: Env"},
		{name: "bad tag glob", args: []string{"rds", "-t", "Env=[p"}, wantErr: "tag Env: invalid pattern"},
		{name: "bad sort field", args: []string{"rds", "-a", "--sort", "size"}, wantErr: "size"},
	})
}
