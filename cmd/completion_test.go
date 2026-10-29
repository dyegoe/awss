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
	"strconv"
	"strings"
	"testing"

	"github.com/dyegoe/awss/search"

	"github.com/spf13/cobra"
)

// complete runs Cobra's hidden __complete command, as the shell does on <TAB>, and returns the
// values offered and the directive.
func complete(t *testing.T, args ...string) ([]string, cobra.ShellCompDirective) {
	t.Helper()
	out, err := runCLI(t, append([]string{cobra.ShellCompRequestCmd}, args...)...)
	if err != nil {
		t.Fatalf("Execute(__complete %v) error = %v", args, err)
	}
	values := []string{}
	for _, line := range strings.Split(out, "\n") {
		if d, ok := strings.CutPrefix(line, ":"); ok {
			n, err := strconv.Atoi(d)
			if err != nil {
				t.Fatalf("Execute(__complete %v) directive %q: %v", args, line, err)
			}
			return values, cobra.ShellCompDirective(n)
		}
		values = append(values, line)
	}
	t.Fatalf("Execute(__complete %v) = %q, want a :<directive> line", args, out)
	return nil, 0
}

// TestRegisterCompletions_sort checks that --sort of every search command completes its sort
// fields, the list awss validates against, without file names.
func TestRegisterCompletions_sort(t *testing.T) {
	for _, sub := range subcommands {
		name := sub.cmd.Name()
		t.Run(name, func(t *testing.T) {
			resetCLI(t, nil)
			got, directive := complete(t, name, "--sort", "")
			if want := search.SortFieldNames(name); len(want) == 0 || !reflect.DeepEqual(got, want) {
				t.Errorf("complete(%s --sort) = %v, want %v", name, got, want)
			}
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("complete(%s --sort) directive = %d, want NoFileComp", name, directive)
			}
		})
	}
}

// TestRegisterCompletions checks the completion of --output on a search command and of the org
// --statuses comma list, and that an invalid flag typed earlier does not stop it.
func TestRegisterCompletions(t *testing.T) {
	noFile := cobra.ShellCompDirectiveNoFileComp
	list := noFile | cobra.ShellCompDirectiveNoSpace
	tests := []struct {
		name          string
		args          []string
		want          []string
		wantDirective cobra.ShellCompDirective
	}{
		{
			name: "--output", args: []string{"vpc", "--output", ""},
			want: []string{"json", "json-pretty", "table"}, wantDirective: noFile,
		},
		{
			name: "--statuses", args: []string{"org", "--statuses", ""},
			want:          []string{"active", "closed", "pending-activation", "pending-closure", "suspended"},
			wantDirective: list,
		},
		{
			name: "--statuses after a comma", args: []string{"org", "--statuses", "active,suspended,p"},
			want: []string{
				"active,suspended,closed", "active,suspended,pending-activation", "active,suspended,pending-closure",
			},
			wantDirective: list,
		},
		{
			name: "an invalid region typed earlier", args: []string{"rds-cluster", "--regions", "mars-1", "--sort", ""},
			want: search.SortFieldNames("rds-cluster"), wantDirective: noFile,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLI(t, nil)
			got, directive := complete(t, tt.args...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("complete(%v) = %v, want %v", tt.args, got, tt.want)
			}
			if directive != tt.wantDirective {
				t.Errorf("complete(%v) directive = %d, want %d", tt.args, directive, tt.wantDirective)
			}
		})
	}
}

// TestListValues checks the comma-list completion: the names not given yet, each prefixed with
// the items already typed.
func TestListValues(t *testing.T) {
	names := func() []string { return []string{"a", "b", "c"} }
	tests := []struct {
		name       string
		toComplete string
		want       []string
	}{
		{name: "nothing typed", toComplete: "", want: []string{"a", "b", "c"}},
		{name: "part of the first item", toComplete: "b", want: []string{"a", "b", "c"}},
		{name: "after one item", toComplete: "a,", want: []string{"a,b", "a,c"}},
		{name: "every name given", toComplete: "a,b,c,", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := listValues(names)(nil, nil, tt.toComplete); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("listValues()(%q) = %v, want %v", tt.toComplete, got, tt.want)
			}
		})
	}
}
