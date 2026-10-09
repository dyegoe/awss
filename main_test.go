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

// Package main contains the main function.
package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// mainArgsEnv carries the CLI arguments to the child process of TestMain_cli, one per line.
// When it is set, the test binary runs main() with them instead of the test.
const mainArgsEnv = "AWSS_TEST_MAIN_ARGS"

// TestMain_cli runs the real binary entry point in a child process and checks its output and exit
// code. A child process gives every run fresh Cobra and Viper globals, and lets the os.Exit
// calls of cmd.Execute happen without ending the test.
func TestMain_cli(t *testing.T) {
	if args, ok := os.LookupEnv(mainArgsEnv); ok {
		os.Args = append([]string{"awss"}, strings.Split(args, "\n")...)
		main()
		return
	}

	tests := []struct {
		name     string
		args     []string
		wantOut  string
		wantExit int
	}{
		{name: "version", args: []string{"--version"}, wantOut: "awss version ", wantExit: 0},
		{name: "help", args: []string{"--help"}, wantOut: "Available Commands:", wantExit: 0},
		{
			name: "validation error exits 1", args: []string{"ec2", "--all", "--output", "xml"},
			wantOut: "Error: invalid output format: xml", wantExit: 1,
		},
		{name: "unknown command exits 1", args: []string{"nope"}, wantOut: `Error: unknown command "nope"`, wantExit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The child is this same test binary, run again.
			self := os.Args[0]
			child := exec.CommandContext(t.Context(), self, "-test.run=^TestMain_cli$") //nolint:gosec // G204: self re-run
			// No config file, profile or region from the machine running the tests.
			child.Env = append(os.Environ(),
				mainArgsEnv+"="+strings.Join(tt.args, "\n"),
				"HOME="+t.TempDir(), "AWS_CONFIG_FILE=", "AWS_PROFILE=", "AWS_REGION=", "AWS_DEFAULT_REGION=",
				// The race detector otherwise sleeps one second when the child exits.
				"GORACE=atexit_sleep_ms=0",
			)

			out, err := child.CombinedOutput()

			exit := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exit = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("running the child process: %v", err)
			}
			if exit != tt.wantExit {
				t.Errorf("awss %v exit code = %d, want %d; output:\n%s", tt.args, exit, tt.wantExit, out)
			}
			if !strings.Contains(string(out), tt.wantOut) {
				t.Errorf("awss %v output = %q, want it to contain %q", tt.args, out, tt.wantOut)
			}
		})
	}
}
