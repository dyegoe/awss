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
	"github.com/dyegoe/awss/common"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelS3All   = "s3.all"
	labelS3Sort  = "s3.sort"
	labelS3Regex = "s3.regex"

	// flagRegex names the flag that switches name patterns from globs to regular expressions.
	flagRegex = "regex"
)

// s3Filters represents the filters for the s3 command.
//
// S3 has no server-side name filter, so "name" is matched client-side in search/s3.
type s3Filters struct {
	Names []string `filter:"name"`
}

var s3F = s3Filters{}

// s3Cmd represents the s3 command.
var s3Cmd = &cobra.Command{
	Use:   "s3",
	Short: "Search for S3 buckets.",
	Long: `
Search for S3 buckets.
Buckets are listed per region, so use --regions all to search every region.

You can search buckets by name using glob patterns ('*' matches anything, '?' one character):
	awss s3 -n 'prod-*,*-logs'

Use --regex to treat the patterns as Go regular expressions instead:
	awss s3 -n '^prod-.*-(logs|backups)$' --regex

Use --all to list all buckets without any filter. This flag cannot be combined with --names.
`,
	RunE: s3RunE,
}

// s3FilterFlags lists all s3 filter flag names for mutual exclusivity with --all.
var s3FilterFlags = []string{flagNames}

func s3RunE(cmd *cobra.Command, _ []string) error {
	// Validate the patterns early, before any AWS call.
	if _, err := common.NewMatcher(s3F.Names, viper.GetBool(labelS3Regex)); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		allLabel:    labelS3All,
		sortLabel:   labelS3Sort,
		regexLabel:  labelS3Regex,
		filterFlags: s3FilterFlags,
	}, nil, nil, s3F)
}

func s3InitFlags() {
	rootCmd.AddCommand(s3Cmd)

	s3Cmd.Flags().BoolP("all", "a", false,
		"List all buckets without any filter. Cannot be combined with --names.")
	s3Cmd.Flags().StringSliceVarP(&s3F.Names, flagNames, "n", []string{},
		"Filter buckets by name patterns. Globs by default, see --regex. `'prod-*,*-logs'`")
	s3Cmd.Flags().Bool(flagRegex, false,
		"Treat --names patterns as Go regular expressions instead of globs.")
	s3Cmd.Flags().String("sort", "name", sortHelp("s3", "buckets", "name"))
}

func s3InitViper() error {
	return bindFlags(s3Cmd, map[string]string{
		labelS3All:   flagAll,
		labelS3Sort:  flagSort,
		labelS3Regex: flagRegex,
	})
}
