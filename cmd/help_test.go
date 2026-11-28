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
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestTrailingExample checks only a help text that ends with its one back-quoted word is split.
func TestTrailingExample(t *testing.T) {
	tests := []struct {
		name        string
		usage       string
		wantExample string
		wantText    string
		wantOK      bool
	}{
		{
			name: "trailing example", usage: "Search these route tables only, exact. `tgw-rtb-0123`",
			wantExample: "tgw-rtb-0123", wantText: "Search these route tables only, exact.", wantOK: true,
		},
		{name: "word in the sentence", usage: "Config file: a `name` such as devops-lz2."},
		{name: "two back-quoted words", usage: "`all` uses every profile. `p1,p2`"},
		{name: "no back quote", usage: "Show empty resources."},
		{name: "one back quote", usage: "Odd text`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			example, text, ok := trailingExample(tt.usage)
			if example != tt.wantExample || text != tt.wantText || ok != tt.wantOK {
				t.Errorf("trailingExample(%q) = %q, %q, %v, want %q, %q, %v",
					tt.usage, example, text, ok, tt.wantExample, tt.wantText, tt.wantOK)
			}
		})
	}
}

// TestFlagUsages checks a trailing example is shown once, as the placeholder, and the default
// keeps the form pflag gives it.
func TestFlagUsages(t *testing.T) {
	tests := []struct {
		name   string
		define func(fs *pflag.FlagSet)
		want   string
		absent string
	}{
		{
			name:   "slice without default",
			define: func(fs *pflag.FlagSet) { fs.StringSlice("ids", nil, "Keep these IDs, exact. `tgw-0123`") },
			want:   "--ids tgw-0123   Keep these IDs, exact.\n",
			absent: "exact. tgw-0123",
		},
		{
			name:   "string default is quoted",
			define: func(fs *pflag.FlagSet) { fs.String("sort", "id", "Sort by id, name. `name`") },
			want:   "--sort name   Sort by id, name. (default \"id\")\n",
		},
		{
			name:   "other default is not quoted",
			define: func(fs *pflag.FlagSet) { fs.Duration("wait", time.Minute, "Wait this long. `90s`") },
			want:   "--wait 90s   Wait this long. (default 1m0s)\n",
		},
		{
			name:   "word in the sentence is left to pflag",
			define: func(fs *pflag.FlagSet) { fs.String("config", "", "A `name` to read.") },
			want:   "--config name   A name to read.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			tt.define(fs)
			got := flagUsages(fs)
			if !strings.Contains(got, tt.want) || (tt.absent != "" && strings.Contains(got, tt.absent)) {
				t.Errorf("flagUsages() = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

// TestFlagUsages_everyCommand checks no flag of any command repeats its example at the end of its
// help text (#279).
func TestFlagUsages_everyCommand(t *testing.T) {
	resetCLI(t, nil)
	for _, cmd := range append([]*cobra.Command{rootCmd}, documentedCommands(rootCmd)...) {
		usages := flagUsages(cmd.LocalFlags()) + flagUsages(cmd.InheritedFlags())
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			example, text, ok := trailingExample(f.Usage)
			if !ok {
				return
			}
			if !strings.Contains(usages, "--"+f.Name+" "+example+" ") || strings.Contains(usages, text+" "+example) {
				t.Errorf("%s --help shows --%s as %q, want the example %q once, as the placeholder",
					cmd.CommandPath(), f.Name, lineOf(usages, "--"+f.Name+" "), example)
			}
		})
	}
}

// lineOf returns the line of text that contains s, or "" when none does.
func lineOf(text, s string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, s) {
			return line
		}
	}
	return ""
}

// TestSetUsageTemplate checks --help goes through flagUsages: the help of a real command shows the
// example once, and cobra's template still has the calls setUsageTemplate replaces.
func TestSetUsageTemplate(t *testing.T) {
	resetCLI(t, nil)
	tmpl := rootCmd.UsageTemplate()
	if strings.Contains(tmpl, ".FlagUsages") || !strings.Contains(tmpl, awssLocalFlags) ||
		!strings.Contains(tmpl, awssInheritedFlags) {
		t.Fatalf("usage template does not list the flags with flagUsages: cobra's template changed?\n%s", tmpl)
	}

	got, err := runCLI(t, "tgw", "routes", "--help")
	if err != nil {
		t.Fatalf("awss tgw routes --help error = %v", err)
	}
	want := "--route-table-ids tgw-rtb-0123"
	if !strings.Contains(got, want) || strings.Contains(got, "exact. tgw-rtb-0123") {
		t.Errorf("awss tgw routes --help = %q, want %q once, not repeated after the text", got, want)
	}
}
