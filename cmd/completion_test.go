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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/dyegoe/awss/search"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		{
			name: "a free-text flag", args: []string{"rds", "--engines", ""},
			want: []string{}, wantDirective: noFile,
		},
		{
			name: "--config", args: []string{"vpc", "--config", ""},
			want: []string{"yaml", "yml"}, wantDirective: cobra.ShellCompDirectiveFilterFileExt,
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

// TestFixedLists checks the comma-list completion of the flags of a fixed list, from the AWS SDK enums
// or the lists awss validates against.
func TestFixedLists(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "ec2 --instance-states", args: []string{"ec2", "-s", "running,"},
			want: []string{
				"running,pending", "running,shutting-down", "running,terminated", "running,stopping", "running,stopped",
			},
		},
		{
			name: "ebs --statuses", args: []string{"ebs", "--statuses", ""},
			want: []string{"creating", "available", "in-use", "deleting", "deleted", "error"},
		},
		{
			name: "ebs --volume-types", args: []string{"ebs", "--volume-types", ""},
			want: []string{"standard", "io1", "io2", "gp2", "sc1", "st1", "gp3"},
		},
		{
			name: "subnet --states", args: []string{"subnet", "--states", ""},
			want: []string{"pending", "available", "unavailable", "failed", "failed-insufficient-capacity"},
		},
		{
			name: "vpc --default", args: []string{"vpc", "--default", ""},
			want: []string{"true", "false"},
		},
		{
			name: "eni --availability-zones", args: []string{"eni", "-z", "a,"},
			want: []string{"a,b", "a,c", "a,d", "a,e", "a,f"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLI(t, nil)
			got, directive := complete(t, tt.args...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("complete(%v) = %v, want %v", tt.args, got, tt.want)
			}
			if want := cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace; directive != want {
				t.Errorf("complete(%v) directive = %d, want %d", tt.args, directive, want)
			}
		})
	}
}

// TestRegionValues checks the --regions completion: the all-regions list of the config file, or the
// default list when it cannot be read, with "all" as the first item only.
func TestRegionValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "--regions from the config file", args: []string{"vpc", "--config", "testdata/config.yaml", "--regions", ""},
			want: []string{"all", "eu-central-1", "eu-north-1"},
		},
		{
			name: "--regions offers all first only",
			args: []string{"vpc", "--config", "testdata/config.yaml", "--regions", "eu-north-1,"},
			want: []string{"eu-north-1,eu-central-1"},
		},
		{
			name: "--regions with a missing config file", args: []string{"vpc", "--config", "missing.yaml", "--regions", "eu-c"},
			want: []string{"all", "eu-central-1", "eu-north-1", "eu-west-1", "eu-west-2", "eu-west-3", "us-east-1",
				"us-east-2", "us-west-1", "us-west-2", "ca-central-1", "sa-east-1", "ap-south-1", "ap-southeast-1",
				"ap-southeast-2", "ap-northeast-3", "ap-northeast-2", "ap-northeast-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLI(t, nil)
			got, directive := complete(t, tt.args...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("complete(%v) = %v, want %v", tt.args, got, tt.want)
			}
			if want := cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace; directive != want {
				t.Errorf("complete(%v) directive = %d, want %d", tt.args, directive, want)
			}
		})
	}
}

// TestRegisterCompletions_everyFlag checks that no flag that takes a value falls back to file
// names, so a flag added later is covered too. --config is the one flag that takes a file.
func TestRegisterCompletions_everyFlag(t *testing.T) {
	type flagOf struct{ cmd, flag string }
	resetCLI(t, nil)
	var flags []flagOf
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { flags = append(flags, flagOf{"vpc", f.Name}) })
	for _, sub := range subcommands {
		sub.cmd.NonInheritedFlags().VisitAll(func(f *pflag.Flag) { flags = append(flags, flagOf{sub.cmd.Name(), f.Name}) })
	}
	for _, f := range flags {
		t.Run(f.cmd+" --"+f.flag, func(t *testing.T) {
			resetCLI(t, nil)
			flag := rootCmd.PersistentFlags().Lookup(f.flag)
			if flag == nil {
				flag = mustCommand(t, f.cmd).Flags().Lookup(f.flag)
			}
			if flag.Value.Type() == "bool" {
				return
			}
			want := cobra.ShellCompDirectiveNoFileComp
			if f.flag == labelConfig {
				want = cobra.ShellCompDirectiveFilterFileExt
			}
			if _, directive := complete(t, f.cmd, "--"+f.flag, ""); directive&want == 0 {
				t.Errorf("complete(%s --%s) directive = %d, want %d set", f.cmd, f.flag, directive, want)
			}
		})
	}
}

// mustCommand returns the search command called name.
func mustCommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, sub := range subcommands {
		if sub.cmd.Name() == name {
			return sub.cmd
		}
	}
	t.Fatalf("no search command %q", name)
	return nil
}

// TestAwsProfiles checks the --profiles and --org-profile completion: the profiles of the AWS
// config file, "all" first for --profiles only, and nothing when the file cannot be read.
func TestAwsProfiles(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		content string
		want    []string
	}{
		{
			name: "--profiles", args: []string{"vpc", "--profiles", ""},
			content: "[default]\n[profile dev]\n[profile prod]\n", want: []string{"all", "default", "dev", "prod"},
		},
		{
			name: "--profiles after a comma", args: []string{"vpc", "--profiles", "dev,"},
			content: "[profile dev]\n[profile prod]\n", want: []string{"dev,prod"},
		},
		{
			name: "--org-profile", args: []string{"vpc", "--org-profile", ""},
			content: "[profile dev]\n[profile prod]\n", want: []string{"dev", "prod"},
		},
		{
			name: "an unreadable AWS config file", args: []string{"vpc", "--org-profile", ""},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLI(t, nil)
			file := filepath.Join(t.TempDir(), "config")
			if tt.content != "" {
				if err := os.WriteFile(file, []byte(tt.content), 0o600); err != nil {
					t.Fatalf("WriteFile(%s) error = %v", file, err)
				}
			}
			t.Setenv("AWS_CONFIG_FILE", file)
			got, directive := complete(t, tt.args...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("complete(%v) = %v, want %v", tt.args, got, tt.want)
			}
			if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
				t.Errorf("complete(%v) directive = %d, want NoFileComp set", tt.args, directive)
			}
		})
	}
}

// TestNoArgs checks that <TAB> after every search command offers no file names and shows the
// Active Help hint to the flags, unless AWSS_ACTIVE_HELP or COBRA_ACTIVE_HELP turns it off.
func TestNoArgs(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		wantHint bool
	}{
		{name: "hint on", envVar: "AWSS_ACTIVE_HELP", envValue: "", wantHint: true},
		{name: "hint off for awss", envVar: "AWSS_ACTIVE_HELP", envValue: "0", wantHint: false},
		{name: "hint off for every cobra tool", envVar: "COBRA_ACTIVE_HELP", envValue: "0", wantHint: false},
	}
	for _, tt := range tests {
		for _, sub := range subcommands {
			name := sub.cmd.Name()
			t.Run(tt.name+"/"+name, func(t *testing.T) {
				t.Setenv("AWSS_ACTIVE_HELP", "")
				t.Setenv("COBRA_ACTIVE_HELP", "")
				t.Setenv(tt.envVar, tt.envValue)
				resetCLI(t, nil)
				got, directive := complete(t, name, "")
				want := []string{}
				if tt.wantHint {
					want = []string{"_activeHelp_ " + fmt.Sprintf(noArgsHint, name)}
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("complete(%s \"\") = %q, want %q", name, got, want)
				}
				if directive != cobra.ShellCompDirectiveNoFileComp {
					t.Errorf("complete(%s \"\") directive = %d, want NoFileComp", name, directive)
				}
			})
		}
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
