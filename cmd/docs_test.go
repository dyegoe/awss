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
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/dyegoe/awss/search"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// readme returns the text of the repository README.
func readme(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	return string(b)
}

// documentedCommands returns every user-facing command under cmd, depth first: the built-in help
// and completion commands and hidden commands are left out.
func documentedCommands(cmd *cobra.Command) []*cobra.Command {
	var cmds []*cobra.Command
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		cmds = append(cmds, c)
		cmds = append(cmds, documentedCommands(c)...)
	}
	return cmds
}

// TestSubcommands_readmeCommandsAndFlags checks README.md names every command as "awss <command>" and every
// flag as "--<flag>", global flags included, so a new command or flag cannot ship undocumented.
func TestSubcommands_readmeCommandsAndFlags(t *testing.T) {
	resetCLI(t, nil)
	text := readme(t)
	checkFlags := func(where string, fs *pflag.FlagSet) {
		fs.VisitAll(func(f *pflag.Flag) {
			if !f.Hidden && f.Name != "help" && !strings.Contains(text, "--"+f.Name) {
				t.Errorf("README.md does not mention --%s (%s)", f.Name, where)
			}
		})
	}
	checkFlags("global", rootCmd.PersistentFlags())
	for _, c := range documentedCommands(rootCmd) {
		path := c.CommandPath()
		if !strings.Contains(text, path) {
			t.Errorf("README.md does not mention %q", path)
		}
		checkFlags(path, c.LocalNonPersistentFlags())
	}
}

// TestSubcommands_readmeSortFields checks README.md has, for every command with --sort, a "Sort by" line
// listing exactly its sort fields: none missing, none removed from the code.
func TestSubcommands_readmeSortFields(t *testing.T) {
	resetCLI(t, nil)
	// The "Sort by: `--sort a|b|c`" line of a command.
	sortLine := regexp.MustCompile("Sort by: `--sort ([a-z0-9|-]+)`")
	var documented [][]string
	for _, m := range sortLine.FindAllStringSubmatch(readme(t), -1) {
		fields := strings.Split(m[1], "|")
		slices.Sort(fields)
		documented = append(documented, fields)
	}
	for _, sub := range subcommands {
		if sub.cmd.Flags().Lookup(flagSort) == nil {
			continue
		}
		want := search.SortFieldNames(engineName(sub.cmd))
		if !slices.ContainsFunc(documented, func(got []string) bool { return slices.Equal(got, want) }) {
			t.Errorf("README.md has no \"Sort by: `--sort ...`\" line with exactly the sort fields of %s: %s",
				sub.cmd.CommandPath(), strings.Join(want, "|"))
		}
	}
}
