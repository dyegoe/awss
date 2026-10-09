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
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	searchOrg "github.com/dyegoe/awss/search/org"
)

// orgAwsConfig is an AWS config file with two profiles.
const orgAwsConfig = "[profile org]\nregion = us-east-1\n[profile member]\nregion = us-east-1\n"

// orgCase is one table entry of TestExecute_org.
type orgCase struct {
	name         string
	args         []string
	wantProfiles []string
	wantSort     string
	wantFilters  map[string][]string
	wantShowTags bool
	wantErr      string
}

// TestExecute_org checks that org searches one profile in the global region, whatever --regions
// says, passes the status filter and the tags flags, and refuses several profiles or an unknown
// status before any search.
func TestExecute_org(t *testing.T) {
	tests := []orgCase{
		{name: "default profile", args: []string{"org"}, wantProfiles: []string{""}, wantSort: "name"},
		{
			name: "statuses", args: []string{"org", "-s", "active,pending-closure"}, wantProfiles: []string{""},
			wantSort: "name", wantFilters: map[string][]string{"status": {"active", "pending-closure"}},
		},
		{
			name: "tags keys imply tags", args: []string{"org", "--show-tags-keys", "Env"}, wantProfiles: []string{""},
			wantSort: "name", wantShowTags: true,
		},
		{
			name: "one profile, regions ignored, sort", args: []string{"org", "--profiles", "org", "--regions",
				"eu-west-1,us-east-2", "--sort", "joined"},
			wantProfiles: []string{"org"}, wantSort: "joined",
		},
		{name: "unknown status", args: []string{"org", "--statuses", "deleted"}, wantErr: "invalid status: deleted"},
		{name: "two profiles", args: []string{"org", "--profiles", "org,member"}, wantErr: "not 2 (org,member)"},
		{name: "all profiles", args: []string{"org", "--profiles", "all"}, wantErr: "pass one profile"},
		{name: "bad sort field", args: []string{"org", "--sort", "owner"}, wantErr: "owner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)
			useAwsConfig(t, orgAwsConfig)

			_, err := runCLI(t, tt.args...)

			if tt.wantErr == "" {
				checkOrgSearch(t, &tt, err, *calls)
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Execute(%v) error = %v, want it to contain %q", tt.args, err, tt.wantErr)
			}
			if len(*calls) != 0 {
				t.Errorf("Execute(%v) searches run = %d, want 0", tt.args, len(*calls))
			}
		})
	}
}

// checkOrgSearch checks that a run of tt succeeded with one org search as tt wants.
func checkOrgSearch(t *testing.T, tt *orgCase, err error, calls []searchCall) {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute(%v) error = %v", tt.args, err)
	}
	if len(calls) != 1 {
		t.Fatalf("Execute(%v) searches run = %d, want 1", tt.args, len(calls))
	}
	got := calls[0]
	wantFilters := tt.wantFilters
	if wantFilters == nil {
		wantFilters = map[string][]string{}
	}
	if !reflect.DeepEqual(got.filters, wantFilters) || got.opts.ShowTags != tt.wantShowTags {
		t.Errorf("Execute(%v) filters %v show tags %v, want %v %v", tt.args,
			got.filters, got.opts.ShowTags, wantFilters, tt.wantShowTags)
	}
	if got.cmd != "org" || !reflect.DeepEqual(got.profiles, tt.wantProfiles) ||
		!reflect.DeepEqual(got.regions, []string{searchOrg.Region}) || got.opts.SortField != tt.wantSort {
		t.Errorf("Execute(%v) searched %s %v %v sort %q, want org %v [%s] sort %q", tt.args,
			got.cmd, got.profiles, got.regions, got.opts.SortField, tt.wantProfiles, searchOrg.Region, tt.wantSort)
	}
}

// orgNamesCall records one call of orgAccountNames.
type orgNamesCall struct {
	profile     string
	hasDeadline bool
}

// mockOrgAccountNames replaces orgAccountNames with one returning names, or err, and records the
// calls.
func mockOrgAccountNames(t *testing.T, names map[string]string, err error) *[]orgNamesCall {
	t.Helper()
	calls := &[]orgNamesCall{}
	old := orgAccountNames
	t.Cleanup(func() { orgAccountNames = old })
	orgAccountNames = func(ctx context.Context, profile string) (map[string]string, error) {
		_, ok := ctx.Deadline()
		*calls = append(*calls, orgNamesCall{profile: profile, hasDeadline: ok})
		return names, err
	}
	return calls
}

// TestExecute_orgProfile checks the opt-in account names from the organization: the accounts map
// wins, a failed call falls back to the map with a warning, and nothing is called without the
// flag or for a command without an Owner column.
func TestExecute_orgProfile(t *testing.T) {
	org := map[string]string{"111111111111": "org-network", "222222222222": "org-tools"}
	tests := []struct {
		name      string
		args      []string
		config    string
		orgErr    error
		wantCalls []orgNamesCall
		want      map[string]string
		wantOut   string
	}{
		{
			name: "names from the organization", args: []string{"subnet", "--all", "--org-profile", "org"},
			wantCalls: []orgNamesCall{{profile: "org", hasDeadline: true}}, want: org,
		},
		{
			name: "the accounts map wins", args: []string{"vpc", "--all", "--org-profile", "org", "--timeout", "0"},
			config:    "accounts:\n  \"111111111111\": network\n",
			wantCalls: []orgNamesCall{{profile: "org"}},
			want:      map[string]string{"111111111111": "network", "222222222222": "org-tools"},
		},
		{
			name: "org-profile config key", args: []string{"eni", "--all"}, config: "org-profile: org\n",
			wantCalls: []orgNamesCall{{profile: "org", hasDeadline: true}}, want: org,
		},
		{
			name: "a failed call keeps the accounts map", args: []string{"subnet", "--all", "--org-profile", "member"},
			config: "accounts:\n  \"111111111111\": network\n", orgErr: errors.New("AccessDeniedException"),
			wantCalls: []orgNamesCall{{profile: "member", hasDeadline: true}},
			want:      map[string]string{"111111111111": "network"},
			wantOut:   "awss: warning: no account names from the organization (--org-profile member): AccessDeniedException",
		},
		{name: "off by default", args: []string{"subnet", "--all"}},
		{name: "no Owner column, no call", args: []string{"ec2", "--all", "--org-profile", "org"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searches := resetCLI(t, nil)
			calls := mockOrgAccountNames(t, org, tt.orgErr)
			args := tt.args
			if tt.config != "" {
				args = append([]string{"--config", writeConfig(t, tt.config)}, args...)
			}

			out, err := runCLI(t, args...)
			if err != nil {
				t.Fatalf("Execute(%v) error = %v, want nil", args, err)
			}
			if len(*calls) != len(tt.wantCalls) || len(tt.wantCalls) > 0 && !reflect.DeepEqual(*calls, tt.wantCalls) {
				t.Errorf("Execute(%v) organization calls = %+v, want %+v", args, *calls, tt.wantCalls)
			}
			if got := (*searches)[0].opts.AccountNames; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Execute(%v) AccountNames = %v, want %v", args, got, tt.want)
			}
			if !strings.Contains(out, tt.wantOut) {
				t.Errorf("Execute(%v) output = %q, want it to contain %q", args, out, tt.wantOut)
			}
		})
	}
}
