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
	"context"
	"slices"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"
	"github.com/dyegoe/awss/tui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// tuiDefault is the resource the TUI shows when none is given.
const tuiDefault = "ec2"

// sortID and sortName are the default sort fields of the TUI resources.
const (
	sortID   = "id"
	sortName = "name"
)

// tuiSorts maps each resource the TUI browses to the sort field of its rows: the default --sort
// of its command. s3obj and org are left out: s3obj needs buckets and org one profile.
var tuiSorts = map[string]string{
	tuiDefault: sortName, "eni": sortID, "ebs": sortID, "vpc": sortName, "subnet": sortName,
	"s3": sortName, "rds": sortID, "rds-cluster": sortID,
}

// runTUI and streamSearch run the TUI and the searches. They are variables so tests can replace
// the terminal and the AWS calls.
var (
	runTUI       = tui.Run
	streamSearch = search.Stream
)

// tuiCmd represents the experimental tui command.
var tuiCmd = &cobra.Command{
	Use:   "tui [resource]",
	Short: "Browse resources in an interactive terminal UI (experimental).",
	Long: `
Browse resources in an interactive terminal UI, in the style of k9s (experimental).

It loads every resource of the type across the profiles and regions, with no filter, keeps them in
memory and filters them as you type. The resource defaults to ec2.

Keys: / filter (words that must all appear; ~word matches fuzzily), : switch resource (e.g. :eni),
d details, e errors, r refresh, esc clear the filter, q quit.
`,
	Hidden:    true,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tuiResources(),
	RunE:      tuiRunE,
}

// tuiResources returns the resources the TUI browses, sorted.
func tuiResources() []string {
	names := make([]string, 0, len(tuiSorts))
	for name := range tuiSorts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func tuiRunE(cmd *cobra.Command, args []string) error {
	resource := tuiDefault
	if len(args) == 1 {
		resource = args[0]
	}
	profiles := viper.GetStringSlice(labelProfiles)
	regions := viper.GetStringSlice(labelRegions)
	// Read before the TUI takes the screen: it may write warnings on stderr.
	names := accountNames(cmd.Context(), cmd.ErrOrStderr())

	return runTUI(cmd.Context(), tui.Config{
		Resource:  resource,
		Resources: tuiResources(),
		Searches:  len(profiles) * len(regions),
		Load:      tuiLoader(profiles, regions, names),
	})
}

// tuiLoader returns the loader of the TUI: an unfiltered search of a resource, sorted by its
// default sort field.
func tuiLoader(profiles, regions []string, names map[string]string) tui.Loader {
	return func(ctx context.Context, resource string, out chan<- common.Results) error {
		_, err := streamSearch(ctx, resource, profiles, regions, map[string][]string{}, &search.Options{
			SortField:    tuiSorts[resource],
			AccountNames: names,
			Concurrency:  viper.GetInt(labelConcurrency),
			Timeout:      viper.GetDuration(labelTimeout),
		}, out)
		return err
	}
}

func tuiInitFlags() {
	rootCmd.AddCommand(tuiCmd)
}

func tuiInitViper() error {
	return nil
}
