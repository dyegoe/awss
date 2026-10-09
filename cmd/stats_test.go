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
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"
)

// TestWriteFailures checks the failure line: nothing for a clean run, and the counts of errors
// and timeouts otherwise.
func TestWriteFailures(t *testing.T) {
	tests := []struct {
		name    string
		summary search.Summary
		want    string
	}{
		{name: "no failure", summary: search.Summary{Searches: 2550, WithResults: 2550}, want: ""},
		{
			name:    "errors only",
			summary: search.Summary{Searches: 2550, Errors: 2},
			want:    "awss: 2 of 2,550 searches failed (2 errors); see the result sets marked with errors\n",
		},
		{
			name:    "timeouts only",
			summary: search.Summary{Searches: 2550, TimedOut: 1},
			want:    "awss: 1 of 2,550 searches failed (1 timed out); see the result sets marked with errors\n",
		},
		{
			name:    "errors and timeouts",
			summary: search.Summary{Searches: 2550, Errors: 2, TimedOut: 1},
			want:    "awss: 3 of 2,550 searches failed (2 errors, 1 timed out); see the result sets marked with errors\n",
		},
		{
			name:    "one search, one error",
			summary: search.Summary{Searches: 1, Errors: 1},
			want:    "awss: 1 of 1 search failed (1 error); see the result sets marked with errors\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &bytes.Buffer{}
			writeFailures(got, &tt.summary)
			if got.String() != tt.want {
				t.Errorf("writeFailures(%+v) = %q, want %q", tt.summary, got.String(), tt.want)
			}
		})
	}
}

// TestWriteStats checks the --stats lines for given counters and elapsed time, with and without
// a peak memory.
func TestWriteStats(t *testing.T) {
	summary := search.Summary{
		Profiles: 150, Regions: 17, Searches: 2550, Concurrency: 32,
		WithResults: 2512, Empty: 35, Errors: 2, TimedOut: 1, Resources: 48213,
	}
	api := common.APICallCounts{Calls: 2904, Retries: 61, Throttled: 12}
	rest := "  profiles       150   regions 17   searches 2,550 (concurrency 32)\n" +
		"  searches       2,512 with results, 35 empty, 2 failed, 1 timed out\n" +
		"  resources      48,213\n" +
		"  API calls      2,904 (61 retries, 12 throttled)\n"
	tests := []struct {
		name  string
		stats runStats
		want  string
	}{
		{
			name: "with peak memory",
			stats: runStats{
				elapsed: 1974 * time.Millisecond, peakMemory: 146_200_000, hasPeakMemory: true,
				summary: summary, api: api,
			},
			want: "awss stats\n  elapsed        1.97s\n  peak memory    146 MB\n" + rest,
		},
		{
			name:  "without peak memory",
			stats: runStats{elapsed: 2*time.Minute + 3456*time.Millisecond, summary: summary, api: api},
			want:  "awss stats\n  elapsed        2m3.46s\n  peak memory    n/a\n" + rest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &bytes.Buffer{}
			writeStats(got, &tt.stats)
			if got.String() != tt.want {
				t.Errorf("writeStats() =\n%s\nwant\n%s", got.String(), tt.want)
			}
		})
	}
}

// TestCollectStats checks that the stats carry the summary, an elapsed time from start and the
// peak memory of the process.
func TestCollectStats(t *testing.T) {
	summary := search.Summary{Searches: 3}
	got := collectStats(time.Now().Add(-time.Second), &summary)
	if got.summary != summary {
		t.Errorf("collectStats() summary = %+v, want %+v", got.summary, summary)
	}
	if got.elapsed < time.Second {
		t.Errorf("collectStats() elapsed = %s, want at least 1s", got.elapsed)
	}
	if peak, ok := common.PeakMemory(); ok && got.peakMemory < 1 {
		t.Errorf("collectStats() peakMemory = %d, want the process peak (%d)", got.peakMemory, peak)
	}
}

// TestFormatElapsed checks the rounding of the elapsed time.
func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "under a second", d: 3456 * time.Microsecond, want: "3ms"},
		{name: "seconds", d: 1974 * time.Millisecond, want: "1.97s"},
		{name: "minutes", d: 2*time.Minute + 3456*time.Millisecond, want: "2m3.46s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatElapsed(tt.d); got != tt.want {
				t.Errorf("formatElapsed(%s) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

// TestThousands checks the digit grouping of counts.
func TestThousands(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{name: "zero", n: 0, want: "0"},
		{name: "three digits", n: 999, want: "999"},
		{name: "four digits", n: 2550, want: "2,550"},
		{name: "seven digits", n: 1234567, want: "1,234,567"},
		{name: "negative", n: -48213, want: "-48,213"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := thousands(tt.n); got != tt.want {
				t.Errorf("thousands(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// TestFormatBytes checks that sizes get a decimal unit, with one decimal below 100.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		b    int64
		want string
	}{
		{name: "bytes", b: 512, want: "512 B"},
		{name: "kilobytes", b: 1500, want: "1.5 KB"},
		{name: "megabytes", b: 146_200_000, want: "146 MB"},
		{name: "gigabytes", b: 1_400_000_000, want: "1.4 GB"},
		{name: "beyond the last unit", b: 5_000_000_000_000_000, want: "5000 TB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBytes(tt.b); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.b, got, tt.want)
			}
		})
	}
}

// TestExecute_stats checks, through the command tree, that the failure line and the stats go to
// stderr, so stdout is the same with and without --stats, and that the stats config key works
// like the flag.
func TestExecute_stats(t *testing.T) {
	failed := search.Summary{Profiles: 1, Regions: 1, Searches: 1, Concurrency: 1, Errors: 1}
	tests := []struct {
		name      string
		args      []string
		config    string
		wantStats bool
	}{
		{name: "without --stats", args: []string{"ec2", "--all"}},
		{name: "with --stats", args: []string{"ec2", "--all", "--stats"}, wantStats: true},
		{name: "stats config key", args: []string{"ec2", "--all"}, config: "stats: true\n", wantStats: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLIWithSummary(t, nil, failed)
			args := tt.args
			if tt.config != "" {
				args = append([]string{"--config", writeConfig(t, tt.config)}, args...)
			}
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			rootCmd.SetArgs(args)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			if _, err := rootCmd.ExecuteC(); err != nil {
				t.Fatalf("Execute(%v) error = %v, want nil: a failed search does not fail the command", args, err)
			}

			if stdout.Len() != 0 {
				t.Errorf("Execute(%v) stdout = %q, want nothing: only the results go to stdout", args, stdout.String())
			}
			wantLine := "awss: 1 of 1 search failed (1 error)"
			if !strings.HasPrefix(stderr.String(), wantLine) {
				t.Errorf("Execute(%v) stderr = %q, want it to start with %q", args, stderr.String(), wantLine)
			}
			if got := strings.Contains(stderr.String(), "awss stats\n"); got != tt.wantStats {
				t.Errorf("Execute(%v) stderr has stats = %v, want %v:\n%s", args, got, tt.wantStats, stderr.String())
			}
		})
	}
}
