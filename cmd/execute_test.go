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
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/search"

	"github.com/spf13/viper"
)

// The tests in this file run the real command tree with rootCmd.ExecuteC. Cobra and Viper hold
// global state, so they never call t.Parallel(), and every case starts from resetCLI.

// searchCall records the arguments one run passed to executeSearch.
type searchCall struct {
	cmd      string
	profiles []string
	regions  []string
	filters  map[string][]string
	opts     search.Options
}

// resetCLI rebuilds the command tree and viper as Execute does, in a clean environment, and
// replaces executeSearch with a recorder. The returned slice holds the recorded calls.
//
// Rebuilding, rather than resetting flag values, is what keeps cases apart: a pflag slice that
// was set once appends on the next parse, and Changed stays true.
func resetCLI(t *testing.T, searchErr error) *[]searchCall {
	t.Helper()
	return resetCLIWithSummary(t, searchErr, search.Summary{})
}

// resetCLIWithSummary is resetCLI with a recorder that returns summary as the outcome of the run.
func resetCLIWithSummary(t *testing.T, searchErr error, summary search.Summary) *[]searchCall {
	t.Helper()

	// No config file, profile or region from the machine running the tests.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_CONFIG_FILE", "")

	rootCmd.ResetCommands()
	rootCmd.ResetFlags()
	for _, sub := range subcommands {
		sub.cmd.ResetFlags()
	}
	viper.Reset()
	if err := setup(); err != nil {
		t.Fatalf("setup() error = %v", err)
	}

	calls := &[]searchCall{}
	old := executeSearch
	executeSearch = func(
		cmd string, profiles, regions []string, filters map[string][]string, opts *search.Options,
	) (search.Summary, error) {
		*calls = append(*calls, searchCall{cmd: cmd, profiles: profiles, regions: regions, filters: filters, opts: *opts})
		return summary, searchErr
	}
	t.Cleanup(func() {
		executeSearch = old
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	return calls
}

// runCLI runs the root command with args and returns the error and what it printed.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	rootCmd.SetArgs(args)
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	_, err := rootCmd.ExecuteC()
	return out.String(), err
}

// writeConfig writes an awss config file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// defaultOpts returns the options a run gets with no global flag, for the given sort field.
func defaultOpts(sortField string) search.Options {
	return search.Options{
		SortField: sortField, Output: "table", Concurrency: search.DefaultConcurrency, Timeout: defaultTimeout,
	}
}

// allCase is the case of "<cmd> --all" with every default: no filter, the default profile and
// region, and the command's default sort field.
func allCase(cmd, sortField string) executeCase {
	return executeCase{
		name: cmd + " --all",
		args: []string{cmd, "--all"},
		want: searchCall{
			cmd: cmd, profiles: []string{""}, regions: []string{"us-east-1"},
			filters: map[string][]string{}, opts: defaultOpts(sortField),
		},
	}
}

// executeCase is one table entry of TestExecute_search.
type executeCase struct {
	name string
	args []string
	// config, when set, is written to a file passed with --config.
	config string
	// regionEnv, when set, is the AWS_REGION of the run.
	regionEnv string
	// awsConfig, when set, is written to a file AWS_CONFIG_FILE points to.
	awsConfig string
	want      searchCall
}

func executeCases() []executeCase {
	cases := append(subcommandCases(), globalCases()...)
	return append(cases, profileCases()...)
}

// subcommandCases runs each search subcommand with its own flags.
func subcommandCases() []executeCase {
	maxKeysOpts := defaultOpts("key")
	maxKeysOpts.MaxKeys = 50
	maxKeysOpts.Regex = true
	maxKeysOpts.MaxBuckets = 5
	noNameOpts := defaultOpts("id")
	noNameOpts.NoInstanceName = true
	regexOpts := defaultOpts("name")
	regexOpts.Regex = true

	return []executeCase{
		allCase("ec2", "name"),
		{
			name: "ec2 filters and global flags",
			args: []string{
				"ec2", "-n", "web-*", "-s", "running,stopped", "-z", "a,b", "--cidrs", "10.0.0.0/16",
				"--regions", "us-east-1,eu-west-1", "--output", "json", "--sort", "id",
				"--show-empty", "--timeout", "90s", "--concurrency", "8",
			},
			want: searchCall{
				cmd: "ec2", profiles: []string{""}, regions: []string{"us-east-1", "eu-west-1"},
				filters: map[string][]string{
					"tag:Name": {"web-*"}, "instance-state-name": {"running", "stopped"},
					"availability-zone": {"a", "b"}, "cidr": {"10.0.0.0/16"},
				},
				opts: search.Options{
					SortField: "id", Output: "json", ShowEmpty: true, Concurrency: 8, Timeout: 90 * time.Second,
				},
			},
		},
		{
			name: "eni --no-instance-name",
			args: []string{"eni", "--all", "--no-instance-name"},
			want: searchCall{
				cmd: "eni", profiles: []string{""}, regions: []string{"us-east-1"}, filters: map[string][]string{},
				opts: noNameOpts,
			},
		},
		allCase("ebs", "id"),
		allCase("subnet", "name"),
		{
			name: "s3 name patterns with --regex",
			args: []string{"s3", "-n", "^prod-", "--regex"},
			want: searchCall{
				cmd: "s3", profiles: []string{""}, regions: []string{"us-east-1"},
				filters: map[string][]string{"name": {"^prod-"}},
				opts:    regexOpts,
			},
		},
		{
			name: "s3obj bucket pattern, keys, --max-keys and --max-buckets",
			args: []string{
				"s3obj", "-b", "logs-*", "-K", "^app/", "--regex", "--max-keys", "50", "--max-buckets", "5",
			},
			want: searchCall{
				cmd: "s3obj", profiles: []string{""}, regions: []string{"us-east-1"},
				filters: map[string][]string{"bucket": {"logs-*"}, "key": {"^app/"}}, opts: maxKeysOpts,
			},
		},
	}
}

// testAwsConfig is an AWS config file with three profiles, for the --profiles cases.
const testAwsConfig = "[default]\nregion = us-east-1\n" +
	"[profile dev]\nregion = us-east-1\n" +
	"[profile prod]\nregion = us-east-1\n"

// profileCases checks that --profiles and all-profiles are validated against the file
// AWS_CONFIG_FILE points to, the one the AWS SDK reads (#168).
func profileCases() []executeCase {
	profilesCall := func(profiles ...string) searchCall {
		return searchCall{
			cmd: "ec2", profiles: profiles, regions: []string{"us-east-1"},
			filters: map[string][]string{}, opts: defaultOpts("name"),
		}
	}
	return []executeCase{
		{
			name: "--profiles from AWS_CONFIG_FILE", args: []string{"ec2", "--all", "--profiles", "dev,prod"},
			awsConfig: testAwsConfig, want: profilesCall("dev", "prod"),
		},
		{
			name: "--profiles all lists AWS_CONFIG_FILE", args: []string{"ec2", "--all", "--profiles", "all"},
			awsConfig: testAwsConfig, want: profilesCall("default", "dev", "prod"),
		},
		{
			name: "all-profiles of the awss config, checked against AWS_CONFIG_FILE",
			args: []string{"ec2", "--all", "--profiles", "all"}, config: "all-profiles: [prod]\n",
			awsConfig: testAwsConfig, want: profilesCall("prod"),
		},
	}
}

// useAwsConfig writes an AWS config file and points AWS_CONFIG_FILE to it for the test.
func useAwsConfig(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aws-config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", path)
}

// globalCases covers the global flags, the environment and the config file.
func globalCases() []executeCase {
	return []executeCase{
		{
			name: "--show-tags-keys implies --show-tags",
			args: []string{"vpc", "--all", "--show-tags-keys", "Name,Environment"},
			want: searchCall{
				cmd: "vpc", profiles: []string{""}, regions: []string{"us-east-1"}, filters: map[string][]string{},
				opts: search.Options{
					SortField: "name", Output: "table", ShowTags: true,
					TagsKeys: []string{"Name", "Environment"}, Concurrency: search.DefaultConcurrency, Timeout: defaultTimeout,
				},
			},
		},
		{
			name:      "region from AWS_REGION when --regions is not set",
			args:      []string{"vpc", "--all"},
			regionEnv: "eu-west-1",
			want: searchCall{
				cmd: "vpc", profiles: []string{""}, regions: []string{"eu-west-1"},
				filters: map[string][]string{}, opts: defaultOpts("name"),
			},
		},
		{
			name: "config file sets the defaults",
			args: []string{"ec2", "--all"},
			config: "regions: [eu-west-1]\noutput: json\ntimeout: 2m\nconcurrency: 4\n" +
				"show:\n  empty: true\nec2:\n  sort: type\n",
			want: searchCall{
				cmd: "ec2", profiles: []string{""}, regions: []string{"eu-west-1"}, filters: map[string][]string{},
				opts: search.Options{
					SortField: "type", Output: "json", ShowEmpty: true, Concurrency: 4, Timeout: 2 * time.Minute,
				},
			},
		},
		{
			name:   "a flag wins over the config file",
			args:   []string{"ec2", "--all", "--output", "table", "--timeout", "0", "--sort", "id", "--concurrency", "2"},
			config: "output: json\ntimeout: 2m\nconcurrency: 4\nec2:\n  sort: type\n",
			want: searchCall{
				cmd: "ec2", profiles: []string{""}, regions: []string{"us-east-1"}, filters: map[string][]string{},
				opts: search.Options{SortField: "id", Output: "table", Concurrency: 2},
			},
		},
	}
}

// TestExecute_search runs every search subcommand through the command tree and checks what
// reaches the search: the command, profiles, regions, filters and options.
func TestExecute_search(t *testing.T) {
	for _, tt := range executeCases() {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)
			if tt.regionEnv != "" {
				t.Setenv("AWS_REGION", tt.regionEnv)
			}
			if tt.awsConfig != "" {
				useAwsConfig(t, tt.awsConfig)
			}
			args := tt.args
			if tt.config != "" {
				args = append([]string{"--config", writeConfig(t, tt.config)}, args...)
			}

			out, err := runCLI(t, args...)

			if err != nil {
				t.Fatalf("Execute(%v) error = %v, output:\n%s", args, err, out)
			}
			if len(*calls) != 1 {
				t.Fatalf("searches run = %d, want 1", len(*calls))
			}
			if got := (*calls)[0]; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("search called with\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// errorCase is one table entry of TestExecute_errors.
type errorCase struct {
	name string
	args []string
	// config and awsConfig are as in executeCase.
	config    string
	awsConfig string
	wantErr   string
}

// errorCases lists argument and validation failures.
func errorCases() []errorCase {
	return []errorCase{
		{name: "unknown subcommand", args: []string{"nope"}, wantErr: `unknown command "nope"`},
		{name: "positional argument", args: []string{"ec2", "--all", "extra"}, wantErr: `unknown command "extra"`},
		{name: "unknown flag", args: []string{"ec2", "--nope"}, wantErr: "unknown flag: --nope"},
		{
			name: "--all with a filter", args: []string{"ec2", "--all", "-n", "web"},
			wantErr: "--all cannot be combined with --names",
		},
		{name: "invalid availability zone", args: []string{"ec2", "-z", "1a"}, wantErr: "must be just a letter"},
		{name: "invalid sort field", args: []string{"vpc", "--all", "--sort", "nope"}, wantErr: "nope"},
		{name: "invalid output", args: []string{"ec2", "--all", "--output", "xml"}, wantErr: "invalid output format: xml"},
		{name: "unknown region", args: []string{"ec2", "--all", "--regions", "mars-1"}, wantErr: "region mars-1 not found"},
		{name: "timeout without unit", args: []string{"ec2", "--all", "--timeout", "300"}, wantErr: `invalid argument "300"`},
		{
			name: "timeout without unit in the config file", args: []string{"ec2", "--all"},
			config: "timeout: 300\n", wantErr: "invalid timeout: 300",
		},
		{
			name: "account ID listed twice in the config file", args: []string{"vpc", "--all"},
			config:  "accounts:\n  \"111111111111\": a\n  \"111111111111\": b\n",
			wantErr: `mapping key "111111111111" already defined`,
		},
		{name: "concurrency of 0", args: []string{"ec2", "--all", "--concurrency", "0"}, wantErr: "invalid concurrency: 0"},
		{
			name: "concurrency not a number in the config file", args: []string{"ec2", "--all"},
			config: "concurrency: many\n", wantErr: "invalid concurrency: many",
		},
		{name: "malformed tag", args: []string{"ec2", "-t", "NoEquals"}, wantErr: "invalid tag format: NoEquals"},
		{name: "invalid CIDR", args: []string{"ec2", "--cidrs", "10.0.0.0"}, wantErr: "10.0.0.0"},
		{name: "s3obj without --buckets", args: []string{"s3obj", "-K", "app/*"}, wantErr: "--buckets is required"},
		{
			name: "s3obj invalid bucket pattern", args: []string{"s3obj", "-b", "logs-[x"},
			wantErr: "invalid --buckets pattern",
		},
		{name: "missing config file", args: []string{"--config", "/nonexistent/awss.yaml", "ec2", "--all"},
			wantErr: "config file not found: /nonexistent/awss.yaml"},
		{name: "config path is a directory", args: []string{"--config", os.TempDir(), "ec2", "--all"},
			wantErr: "config file is a directory"},
		{name: "config path through a file", args: []string{"--config", "execute_test.go/config.yaml", "ec2", "--all"},
			wantErr: "reading config file execute_test.go/config.yaml"},
		{
			name: "profile not in AWS_CONFIG_FILE", args: []string{"ec2", "--all", "--profiles", "dev,staging"},
			awsConfig: testAwsConfig, wantErr: "profile staging not found",
		},
		{
			name: "all-profiles entry not in AWS_CONFIG_FILE", args: []string{"ec2", "--all", "--profiles", "all"},
			config: "all-profiles: [prod, staging]\n", awsConfig: testAwsConfig,
			wantErr: "checking all-profiles: profile staging not found",
		},
		{
			name: "no AWS config file at all", args: []string{"ec2", "--all", "--profiles", "dev"},
			wantErr: "no such file or directory",
		},
	}
}

// TestExecute_errors checks the argument and validation failures: each one fails the command
// with a message naming the problem, and no search runs.
func TestExecute_errors(t *testing.T) {
	for _, tt := range errorCases() {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)
			if tt.awsConfig != "" {
				useAwsConfig(t, tt.awsConfig)
			}
			args := tt.args
			if tt.config != "" {
				args = append([]string{"--config", writeConfig(t, tt.config)}, args...)
			}

			out, err := runCLI(t, args...)

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Execute(%v) error = %v, want it to contain %q", args, err, tt.wantErr)
			}
			if !strings.Contains(out, "Error: ") {
				t.Errorf("output = %q, want the error printed", out)
			}
			if len(*calls) != 0 {
				t.Errorf("searches run = %d, want 0", len(*calls))
			}
		})
	}
}

// TestExecute_searchError checks that an error returned by the search fails the command.
func TestExecute_searchError(t *testing.T) {
	calls := resetCLI(t, errors.New("command ec2 not found"))

	_, err := runCLI(t, "ec2", "--all")

	if err == nil || err.Error() != "command ec2 not found" {
		t.Errorf("Execute() error = %v, want the search error", err)
	}
	if len(*calls) != 1 {
		t.Errorf("searches run = %d, want 1", len(*calls))
	}
}

// TestExecute_resetBetweenRuns checks that resetCLI isolates runs: values set by one run do not
// leak into the next, including slice flags that pflag would otherwise append to.
func TestExecute_resetBetweenRuns(t *testing.T) {
	resetCLI(t, nil)
	if _, err := runCLI(t, "ec2", "-n", "first", "--regions", "eu-west-1", "--output", "json"); err != nil {
		t.Fatalf("first run error = %v", err)
	}

	calls := resetCLI(t, nil)
	if _, err := runCLI(t, "ec2", "-n", "second"); err != nil {
		t.Fatalf("second run error = %v", err)
	}

	want := searchCall{
		cmd: "ec2", profiles: []string{""}, regions: []string{"us-east-1"},
		filters: map[string][]string{"tag:Name": {"second"}}, opts: defaultOpts("name"),
	}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Errorf("second run searched with %+v, want %+v", *calls, want)
	}
}

// TestExecute_version checks the --version output.
func TestExecute_version(t *testing.T) {
	resetCLI(t, nil)

	out, err := runCLI(t, "--version")

	if err != nil || !strings.Contains(out, "awss version "+version) {
		t.Errorf("--version = %q, %v; want the version", out, err)
	}
}

// accountNamesCase is one table entry of TestExecute_accountNames.
type accountNamesCase struct {
	name      string
	args      []string
	awsConfig string
	config    string
	want      map[string]string
	wantOut   string
}

// accountNamesCases are the runs of TestExecute_accountNames.
func accountNamesCases() []accountNamesCase {
	// A profile of the AWS config file names no account: names come only from accounts:.
	const awsConfig = "[profile admins-network-prd]\nsso_account_id = 111111111111\n"
	return []accountNamesCase{
		{
			name: "names from the accounts map, case kept", args: []string{"subnet", "--all"}, awsConfig: awsConfig,
			config: "accounts:\n  \"111111111111\": Network-Prd\n  222222222222: tools\n",
			want:   map[string]string{"111111111111": "Network-Prd", "222222222222": "tools"},
		},
		{
			name: "AWS config profiles give no names", args: []string{"vpc", "--all"}, awsConfig: awsConfig,
			want: nil,
		},
		{
			name: "an unquoted ID gets its leading zero back", args: []string{"eni", "--all"},
			config: "accounts:\n  012345678901: zero\n",
			want:   map[string]string{"012345678901": "zero"},
		},
		{
			name: "an unusable entry is skipped with a warning", args: []string{"vpc", "--all"},
			config:  "accounts:\n  prod: \"123456789012\"\n",
			want:    nil,
			wantOut: `awss: warning: accounts: "prod" is not a 12-digit account ID; ignored`,
		},
		{
			name: "commands without an Owner column get no names", args: []string{"ec2", "--all"},
			config: "accounts:\n  \"333333333333\": tools\n",
			want:   nil,
		},
	}
}

// TestExecute_accountNames checks the account names the vpc, subnet and eni commands pass to the
// search: only from the accounts map of the awss config file, none for the other commands, and
// the warnings printed for unusable entries.
func TestExecute_accountNames(t *testing.T) {
	for _, tt := range accountNamesCases() {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)
			if tt.awsConfig != "" {
				useAwsConfig(t, tt.awsConfig)
			}
			args := tt.args
			if tt.config != "" {
				args = append([]string{"--config", writeConfig(t, tt.config)}, args...)
			}

			out, err := runCLI(t, args...)
			if err != nil {
				t.Fatalf("Execute(%v) error = %v, want nil", args, err)
			}
			if len(*calls) != 1 {
				t.Fatalf("searches run = %d, want 1", len(*calls))
			}
			if got := (*calls)[0].opts.AccountNames; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Execute(%v) AccountNames = %v, want %v", args, got, tt.want)
			}
			if tt.wantOut == "" && out != "" || !strings.Contains(out, tt.wantOut) {
				t.Errorf("Execute(%v) output = %q, want %q", args, out, tt.wantOut)
			}
		})
	}
}
