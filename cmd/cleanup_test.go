/*
Copyright © 2026 Dyego Alexandre Eugenio github@dyego.com.br

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
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/search"
)

// TestExecute_defaultVpc checks the cleanup default-vpc command: a preview by default, --apply
// to delete, and --confirm-all required with --apply over every profile or every region only.
func TestExecute_defaultVpc(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		config       string
		wantProfiles []string
		wantApply    bool
		wantErr      string
	}{
		{name: "preview", args: []string{"cleanup", "default-vpc"}, wantProfiles: []string{""}},
		{name: "preview of every profile", args: []string{"cleanup", "default-vpc", "--profiles", "all"},
			wantProfiles: []string{"default", "dev", "prod"}},
		{name: "apply to named profiles", args: []string{"cleanup", "default-vpc", "--profiles", "dev,prod", "--apply"},
			wantProfiles: []string{"dev", "prod"}, wantApply: true},
		{name: "apply to every profile, confirmed", wantProfiles: []string{"default", "dev", "prod"}, wantApply: true,
			args: []string{"cleanup", "default-vpc", "--profiles", "all", "--apply", "--confirm-all"}},
		{name: "apply to every profile, not confirmed", args: []string{"cleanup", "default-vpc", "--profiles", "all",
			"--apply"}, wantErr: "add --confirm-all"},
		{name: "apply to every profile from the config file, not confirmed", config: "profiles: all\n",
			args: []string{"cleanup", "default-vpc", "--apply"}, wantErr: "add --confirm-all"},
		{name: "apply to every region, not confirmed", args: []string{"cleanup", "default-vpc", "--profiles", "dev",
			"--regions", "all", "--apply"}, wantErr: "add --confirm-all"},
		{name: "apply to every region from the config file, not confirmed", config: "regions: all\n",
			args: []string{"cleanup", "default-vpc", "--profiles", "dev", "--apply"}, wantErr: "add --confirm-all"},
		{name: "confirm without apply", args: []string{"cleanup", "default-vpc", "--profiles", "all",
			"--confirm-all"}, wantErr: "only goes with --apply"},
		{name: "confirm with named profiles and regions", args: []string{"cleanup", "default-vpc", "--profiles", "dev",
			"--apply", "--confirm-all"}, wantErr: "only goes with --profiles all or --regions all"},
		{name: "positional argument", args: []string{"cleanup", "default-vpc", "vpc-1"}, wantErr: "unknown command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)
			useAwsConfig(t, testAwsConfig)
			args := tt.args
			if tt.config != "" {
				args = append(args, "--config", writeConfig(t, tt.config))
			}

			_, err := runCLI(t, args...)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Execute(%v) error = %v, want it to contain %q", args, err, tt.wantErr)
				}
				if len(*calls) != 0 {
					t.Errorf("Execute(%v) runs = %d, want 0", args, len(*calls))
				}
				return
			}
			checkDefaultVpcRun(t, args, err, *calls, tt.wantProfiles, tt.wantApply)
		})
	}
}

// checkDefaultVpcRun checks that a run of args succeeded with one default-vpc run over profiles.
func checkDefaultVpcRun(t *testing.T, args []string, err error, calls []searchCall, profiles []string, apply bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
	if len(calls) != 1 {
		t.Fatalf("Execute(%v) runs = %d, want 1", args, len(calls))
	}
	want := searchCall{
		cmd: "default-vpc", profiles: profiles, regions: []string{"us-east-1"}, filters: map[string][]string{},
		opts: search.Options{
			Output: "table", Concurrency: search.DefaultConcurrency, Timeout: defaultTimeout, Apply: apply,
		},
	}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("Execute(%v) = %+v, want %+v", args, calls[0], want)
	}
}

// TestExecute_defaultVpcGlobalFlags checks that the global flags reach the cleanup.
func TestExecute_defaultVpcGlobalFlags(t *testing.T) {
	calls := resetCLI(t, nil)
	_, err := runCLI(t, "cleanup", "default-vpc", "--regions", "eu-west-1,us-east-2", "--output", "json",
		"--show-empty", "--timeout", "30m", "--concurrency", "4")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := (*calls)[0]
	wantOpts := search.Options{Output: "json", ShowEmpty: true, Concurrency: 4, Timeout: 30 * time.Minute}
	if !reflect.DeepEqual(got.regions, []string{"eu-west-1", "us-east-2"}) || !reflect.DeepEqual(got.opts, wantOpts) {
		t.Errorf("Execute() = %v %+v, want [eu-west-1 us-east-2] %+v", got.regions, got.opts, wantOpts)
	}
}

// TestExecute_defaultVpcSearchError checks that an error of the run is returned.
func TestExecute_defaultVpcSearchError(t *testing.T) {
	resetCLI(t, errors.New("boom"))
	if _, err := runCLI(t, "cleanup", "default-vpc"); err == nil {
		t.Error("Execute(cleanup default-vpc) error = nil, want the run error")
	}
}

// TestCheckApply checks every combination of the safety flags.
func TestCheckApply(t *testing.T) {
	tests := []struct {
		name                       string
		apply, confirm, everywhere bool
		wantErr                    bool
	}{
		{name: "preview"},
		{name: "preview everywhere", everywhere: true},
		{name: "apply", apply: true},
		{name: "apply everywhere", apply: true, everywhere: true, wantErr: true},
		{name: "apply everywhere, confirmed", apply: true, confirm: true, everywhere: true},
		{name: "confirm alone", confirm: true, wantErr: true},
		{name: "confirm without apply", confirm: true, everywhere: true, wantErr: true},
		{name: "confirm with named profiles and regions", apply: true, confirm: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkApply(tt.apply, tt.confirm, tt.everywhere); (err != nil) != tt.wantErr {
				t.Errorf("checkApply(%v, %v, %v) = %v, want error %v", tt.apply, tt.confirm, tt.everywhere, err, tt.wantErr)
			}
		})
	}
}
