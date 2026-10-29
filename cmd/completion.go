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
	"fmt"
	"slices"
	"strings"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"
	searchOrg "github.com/dyegoe/awss/search/org"

	"github.com/spf13/cobra"
)

// registerCompletions adds the shell completion of the flags whose values are a fixed list. The
// values come from the lists awss validates against, so completion and validation cannot drift.
//
// Cobra keeps the completions by flag, and setup creates new flags every time, so every setup
// registers them again.
func registerCompletions() error {
	if err := rootCmd.RegisterFlagCompletionFunc(labelOutput, fixedValues(common.OutputNames)); err != nil {
		return fmt.Errorf("registering the --%s completion: %w", labelOutput, err)
	}
	for _, sub := range subcommands {
		name := sub.cmd.Name()
		sortNames := func() []string { return search.SortFieldNames(name) }
		if err := sub.cmd.RegisterFlagCompletionFunc(flagSort, fixedValues(sortNames)); err != nil {
			return fmt.Errorf("registering the %s --%s completion: %w", name, flagSort, err)
		}
	}
	if err := orgCmd.RegisterFlagCompletionFunc(flagStatuses, listValues(searchOrg.StatusNames)); err != nil {
		return fmt.Errorf("registering the org --%s completion: %w", flagStatuses, err)
	}
	return nil
}

// fixedValues completes a flag that takes one value of names. The shell keeps the values that
// start with what is typed.
func fixedValues(names func() []string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return names(), cobra.ShellCompDirectiveNoFileComp
	}
}

// listValues completes a flag that takes a comma list of names. After "active," it offers the
// names not given yet, each prefixed with "active," so the shell completes the last item, and
// leaves no space so another comma can follow.
func listValues(names func() []string) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		prefix := toComplete[:strings.LastIndex(toComplete, ",")+1]
		given := strings.Split(prefix, ",")
		var out []string
		for _, n := range names() {
			if !slices.Contains(given, n) {
				out = append(out, prefix+n)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
}
