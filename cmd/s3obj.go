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

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search/s3obj"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelS3objSort    = "s3obj.sort"
	labelS3objRegex   = "s3obj.regex"
	labelS3objMaxKeys = "s3obj.max-keys"

	flagBuckets = "buckets"
	flagKeys    = "keys"
	flagMaxKeys = "max-keys"
)

// s3objFilters represents the filters for the s3obj command.
//
// "bucket" names the buckets to scan (exact names) and "key" the key patterns, matched
// client-side in search/s3obj.
type s3objFilters struct {
	Buckets []string `filter:"bucket"`
	Keys    []string `filter:"key"`
}

var s3objF = s3objFilters{}

// s3objCmd represents the s3obj command.
var s3objCmd = &cobra.Command{
	Use:   "s3obj",
	Short: "Search for S3 objects (keys) inside buckets.",
	Long: `
Search for S3 objects (keys) inside the given buckets.
--buckets is required and takes exact bucket names. Each region only scans the buckets that
live in it, so use --regions all when you do not know the bucket's region.

Search keys by glob pattern ('*' matches anything, including '/', '?' one character):
	awss s3obj -b my-bucket -K 'logs/2024/*.gz'

Use --regex to treat the patterns as Go regular expressions instead. It is a switch, the
patterns still go in --keys:
	awss s3obj -b my-bucket -K '^logs/2024-0[1-3]/.*\.gz$' --regex

Without --keys every key of the bucket is listed. Scanning stops after --max-keys keys per
bucket (default 10000) and reports it; refine the pattern or raise the limit.
`,
	Args: cobra.NoArgs,
	RunE: s3objRunE,
}

// s3objFilterFlags lists the s3obj filter flag names. The command has no --all.
var s3objFilterFlags = []string{flagBuckets, flagKeys}

func s3objRunE(cmd *cobra.Command, _ []string) error {
	if len(s3objF.Buckets) == 0 {
		return fmt.Errorf("--%s is required", flagBuckets)
	}
	// Validate the patterns early, before any AWS call.
	if _, err := common.NewMatcher(s3objF.Keys, viper.GetBool(labelS3objRegex)); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		sortLabel:    labelS3objSort,
		regexLabel:   labelS3objRegex,
		maxKeysLabel: labelS3objMaxKeys,
		filterFlags:  s3objFilterFlags,
	}, nil, nil, s3objF)
}

func s3objInitFlags() {
	rootCmd.AddCommand(s3objCmd)

	s3objCmd.Flags().StringSliceVarP(&s3objF.Buckets, flagBuckets, "b", []string{},
		"Buckets to scan, exact names. Required. `my-bucket,other-bucket`")
	s3objCmd.Flags().StringSliceVarP(&s3objF.Keys, flagKeys, "K", []string{},
		"Filter objects by key patterns. Globs by default, see --regex. `'logs/2024/*.gz'`")
	s3objCmd.Flags().Bool(flagRegex, false,
		"Treat --keys patterns as Go regular expressions instead of globs.")
	s3objCmd.Flags().Int(flagMaxKeys, s3obj.DefaultMaxKeys,
		"Maximum number of keys scanned per bucket before stopping.")
	s3objCmd.Flags().String(flagSort, "key", sortHelp("s3obj", "objects", "key"))
}

func s3objInitViper() error {
	return bindFlags(s3objCmd, map[string]string{
		labelS3objSort:    flagSort,
		labelS3objRegex:   flagRegex,
		labelS3objMaxKeys: flagMaxKeys,
	})
}
