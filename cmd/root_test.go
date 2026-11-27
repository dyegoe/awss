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

// Package cmd enables the CLI commands and flags.
//
// It is based on Cobra and Viper.
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/search"

	"github.com/spf13/cobra"
)

// checkFilterFlags checks that a command's --all exclusivity list has one entry per filter struct
// field and that every listed flag is registered on the command.
func checkFilterFlags(t *testing.T, name string, cmd *cobra.Command, numFields int, flags []string) {
	t.Helper()
	if numFields != len(flags) {
		t.Errorf("%sFilters has %d fields but %sFilterFlags lists %d flags", name, numFields, name, len(flags))
	}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("%sFilterFlags lists %q but the %s command has no such flag", name, flag, name)
		}
	}
}

// TestFiltersHelp checks the filter list of the long help: every flag with its dashes, a comma
// between them, a period at the end, and lines wrapped at helpWidth.
func TestFiltersHelp(t *testing.T) {
	long := []string{
		"availability-zones", "instance-states", "private-ips", "public-ips", "volume-ids",
		"tags-key", "instance-types", "default-for-az", "public-ip-on-launch", "owner-ids",
	}
	tests := []struct {
		name  string
		flags []string
		want  string
	}{
		{
			name:  "one flag",
			flags: []string{"ids"},
			want:  "You can search things using the following filters:\n  --ids.",
		},
		{
			name:  "two flags",
			flags: []string{"ids", "tags"},
			want:  "You can search things using the following filters:\n  --ids, --tags.",
		},
		{
			name:  "wrapped",
			flags: long,
			want: "You can search things using the following filters:\n" +
				"  --availability-zones, --instance-states, --private-ips, --public-ips, --volume-ids, --tags-key,\n" +
				"  --instance-types, --default-for-az, --public-ip-on-launch, --owner-ids.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filtersHelp("things", tt.flags)
			if got != tt.want {
				t.Errorf("filtersHelp(things, %v) = %q, want %q", tt.flags, got, tt.want)
			}
			for _, line := range strings.Split(got, "\n") {
				if len(line) > helpWidth {
					t.Errorf("filtersHelp(things, %v) has a line of %d characters, want at most %d",
						tt.flags, len(line), helpWidth)
				}
			}
		})
	}
}

// TestFiltersHelp_commands checks the long help of the commands that list their filters names
// every filter flag, so the list follows the flags.
func TestFiltersHelp_commands(t *testing.T) {
	tests := []struct {
		name  string
		cmd   *cobra.Command
		flags []string
	}{
		{name: "ec2", cmd: ec2Cmd, flags: ec2FilterFlags},
		{name: "eni", cmd: eniCmd, flags: eniFilterFlags},
		{name: "ebs", cmd: ebsCmd, flags: ebsFilterFlags},
		{name: "vpc", cmd: vpcCmd, flags: vpcFilterFlags},
		{name: "subnet", cmd: subnetCmd, flags: subnetFilterFlags},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			long := strings.Join(strings.Fields(tt.cmd.Long), " ")
			for _, flag := range tt.flags {
				if !strings.Contains(long, "--"+flag+",") && !strings.Contains(long, "--"+flag+".") {
					t.Errorf("%s long help does not list --%s", tt.name, flag)
				}
			}
		})
	}
}

// TestInitConfig tests the initConfig function.
func TestInitConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     string
		wantErr bool
	}{
		{
			name:    "empty",
			cfg:     "",
			wantErr: false,
		},
		{
			name:    "non-existent",
			cfg:     "non-existent",
			wantErr: true,
		},
		{
			name:    "existent",
			cfg:     "testdata/config.yaml",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv(envConfig, "")
			if err := initConfig(tt.cfg); (err != nil) != tt.wantErr {
				t.Errorf("initConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNormalizeFlagName checks that the singular --profile and --region read as the plural, and
// that other names, --org-profile among them, stay as they are.
func TestNormalizeFlagName(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want string
	}{
		{name: "profile", flag: "profile", want: "profiles"},
		{name: "region", flag: "region", want: "regions"},
		{name: "plural unchanged", flag: "profiles", want: "profiles"},
		{name: "org-profile unchanged", flag: "org-profile", want: "org-profile"},
		{name: "other flag unchanged", flag: "output", want: "output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeFlagName(nil, tt.flag); string(got) != tt.want {
				t.Errorf("normalizeFlagName(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

// TestParseTimeout tests parseTimeout with flag and config file values.
func TestParseTimeout(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    time.Duration
		wantErr bool
	}{
		{name: "flag default", value: "5m0s", want: 5 * time.Minute},
		{name: "seconds", value: "90s", want: 90 * time.Second},
		{name: "duration value", value: 2 * time.Minute, want: 2 * time.Minute},
		{name: "zero disables", value: "0", want: 0},
		{name: "zero from yaml int", value: 0, want: 0},
		{name: "bare number from yaml", value: 300, wantErr: true},
		{name: "negative", value: "-1s", wantErr: true},
		{name: "garbage", value: "soon", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeout(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTimeout(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseTimeout(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestSubcommands_engines checks that every subcommand has its own engine, registered for its
// command by setup, with a constructor and sort fields that agree, and that the constructor of
// every search passes SortField through. A cleanup, such as default-vpc, has no --sort and is
// known by its package under cleanup/.
func TestSubcommands_engines(t *testing.T) {
	resetCLI(t, nil)
	seen := map[*search.Engine]string{}
	for _, sub := range subcommands {
		t.Run(sub.cmd.CommandPath(), func(t *testing.T) {
			eng := sub.engine
			if eng == nil || eng.New == nil || eng.SortFields == nil || eng.SortFieldNames == nil {
				t.Fatalf("%s: the engine must define New, SortFields and SortFieldNames", sub.cmd.CommandPath())
			}
			if other, ok := seen[eng]; ok {
				t.Errorf("%s shares its engine with %s", sub.cmd.CommandPath(), other)
			}
			seen[eng] = sub.cmd.CommandPath()
			if got := engineOf(sub.cmd); got != eng {
				t.Errorf("engineOf(%s) = %p, want its engine %p", sub.cmd.CommandPath(), got, eng)
			}
			for _, name := range eng.SortFieldNames() {
				if err := eng.CheckSortField(name); err != nil {
					t.Errorf("%s: SortFieldNames lists %q but CheckSortField rejects it: %v", sub.cmd.CommandPath(), name, err)
				}
			}
			r := eng.New("default", "us-east-1", map[string][]string{}, &search.Options{SortField: "id", NoInstanceName: true})
			if r == nil {
				t.Fatalf("%s: New returned nil", sub.cmd.CommandPath())
			}
			cleanup := strings.HasPrefix(reflect.TypeOf(r).Elem().PkgPath(), "github.com/dyegoe/awss/cleanup/")
			if got := r.GetSortField(); !cleanup && got != "id" {
				t.Errorf("%s: New SortField = %q, want %q", sub.cmd.CommandPath(), got, "id")
			}
		})
	}
}

// TestEngineOf checks that a command that is not a registered subcommand has no engine.
func TestEngineOf(t *testing.T) {
	resetCLI(t, nil)
	for _, cmd := range []*cobra.Command{rootCmd, cleanupCmd, {Use: "nope"}} {
		if got := engineOf(cmd); got != nil {
			t.Errorf("engineOf(%s) = %p, want nil", cmd.Name(), got)
		}
	}
}

// TestSubcommands_searchKeepsFilters checks that no search changes the filters map it is given: one
// map is shared by every profile x region goroutine of a run (AGENTS.md). The profile does not
// exist, so each search builds its filters and stops at the AWS config, with no AWS call.
func TestSubcommands_searchKeepsFilters(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	// One key of every kind the commands send: IDs, names, tags, zones, CIDRs and S3 patterns.
	sent := func() map[string][]string {
		return map[string][]string{
			"instance-id": {"i-1"}, "tag:Name": {"web-*"}, "tag": {"Env=prod:dev"},
			"availability-zone": {"a", "b"}, "cidr": {"10.0.0.0/16"}, "vpc-id": {"vpc-1"},
			"name": {"logs-*"}, "bucket": {"b1"}, "key": {"app/*"},
			"engine": {"postgres"}, "engine-version": {"13*"}, "db-cluster-id": {"c1"}, "db-instance-id": {"db-1"},
		}
	}
	for _, sub := range subcommands {
		t.Run(sub.cmd.CommandPath(), func(t *testing.T) {
			filters := sent()
			r := sub.engine.New("awss-test-missing-profile", "us-east-1", filters, &search.Options{NoInstanceName: true})

			r.Search(context.Background())

			if !reflect.DeepEqual(filters, sent()) {
				t.Errorf("%s Search() changed its filters to %v, want %v", sub.cmd.CommandPath(), filters, sent())
			}
		})
	}
}

// TestSubcommands_accountNames checks that every search with an AccountNames field, the ones with
// an Owner column, is marked AccountNames and receives the account names of the run, and that no
// other search is marked.
func TestSubcommands_accountNames(t *testing.T) {
	names := map[string]string{"111111111111": "network"}
	opts := &search.Options{AccountNames: names}
	for _, sub := range subcommands {
		t.Run(sub.cmd.CommandPath(), func(t *testing.T) {
			r := sub.engine.New("default", "us-east-1", map[string][]string{}, opts)
			field := reflect.ValueOf(r).Elem().FieldByName("AccountNames")
			if got, want := sub.engine.AccountNames, field.IsValid(); got != want {
				t.Fatalf("%s: AccountNames = %v, want %v: true exactly when the result has an AccountNames field",
					sub.cmd.CommandPath(), got, want)
			}
			if !field.IsValid() {
				return
			}
			if got := field.Interface(); !reflect.DeepEqual(got, names) {
				t.Errorf("%s: New AccountNames = %v, want %v", sub.cmd.CommandPath(), got, names)
			}
		})
	}
}
