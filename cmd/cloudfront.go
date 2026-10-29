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
	searchCloudFront "github.com/dyegoe/awss/search/cloudfront"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelCloudFrontAll   = "cloudfront.all"
	labelCloudFrontSort  = "cloudfront.sort"
	labelCloudFrontRegex = "cloudfront.regex"
)

// cloudfrontFilters represents the filters for the cloudfront command.
//
// ListDistributions cannot filter, so search/cloudfront matches every one on the results.
type cloudfrontFilters struct {
	IDs     []string `filter:"id"`
	Domains []string `filter:"domain"`
	Origins []string `filter:"origin"`
	Tags    []string `filter:"tag"`
}

var cloudfrontF = cloudfrontFilters{}

// cloudfrontCmd represents the cloudfront command.
var cloudfrontCmd = &cobra.Command{
	Use:   "cloudfront",
	Short: "Search for CloudFront distributions.",
	Long: `
Search for CloudFront distributions.

CloudFront is a global service: one call lists every distribution of an account, so cloudfront
searches each profile once and ignores --regions.

AWS cannot filter distributions, so awss matches every filter on the results:
  --ids      distribution IDs, exact
  --domains  the aliases (CNAMEs) and the *.cloudfront.net domain: globs, or Go regular
             expressions with --regex
  --origins  the domain names of the origins, globs or regular expressions
  --tags     'Key=Value1:Value2,Other=Value': every key must match, the values of one key are
             alternatives, and values accept globs

Filters combine with AND. The distribution that serves www.example.com, for example:
	awss cloudfront -d www.example.com --profiles all

Tags need one API call per distribution, so they are fetched only with --show-tags,
--show-tags-keys or --tags.

Use --all to search for all distributions without any filter. This flag cannot be combined with
other filters.
`,
	Args: cobra.NoArgs,
	RunE: cloudfrontRunE,
}

// cloudfrontFilterFlags lists all cloudfront filter flag names for mutual exclusivity with --all.
var cloudfrontFilterFlags = []string{flagIDs, "domains", "origins", flagTags}

func cloudfrontRunE(cmd *cobra.Command, _ []string) error {
	if err := checkCloudFrontPatterns(viper.GetBool(labelCloudFrontRegex)); err != nil {
		return err
	}
	viper.Set(labelRegions, []string{searchCloudFront.Region})

	return runSearch(cmd, &cmdSpec{
		allLabel:    labelCloudFrontAll,
		sortLabel:   labelCloudFrontSort,
		regexLabel:  labelCloudFrontRegex,
		filterFlags: cloudfrontFilterFlags,
	}, nil, nil, cloudfrontF)
}

// checkCloudFrontPatterns validates the domain, origin and tag patterns before any AWS call.
func checkCloudFrontPatterns(regex bool) error {
	if _, err := common.NewMatcher(cloudfrontF.Domains, regex); err != nil {
		return fmt.Errorf("domains: %w", err)
	}
	if _, err := common.NewMatcher(cloudfrontF.Origins, regex); err != nil {
		return fmt.Errorf("origins: %w", err)
	}
	if _, err := common.NewTagMatcher(cloudfrontF.Tags); err != nil {
		return err
	}
	return nil
}

func cloudfrontInitFlags() {
	rootCmd.AddCommand(cloudfrontCmd)

	cloudfrontCmd.Flags().BoolP(flagAll, "a", false,
		"Search for all distributions without any filter. Cannot be combined with other filters.")
	cloudfrontCmd.Flags().StringSliceVarP(&cloudfrontF.IDs, flagIDs, "i", []string{},
		"Filter distributions by ID, exact. `E1ABCDEF,E2GHIJKL`")
	cloudfrontCmd.Flags().StringSliceVarP(&cloudfrontF.Domains, "domains", "d", []string{},
		"Filter distributions by alias or *.cloudfront.net domain. Globs by default, see --regex. "+
			"`'www.example.com,*.example.org'`")
	cloudfrontCmd.Flags().StringSliceVarP(&cloudfrontF.Origins, "origins", "O", []string{},
		"Filter distributions by origin domain name. Globs by default, see --regex. `'*.s3.amazonaws.com'`")
	cloudfrontCmd.Flags().StringSliceVarP(&cloudfrontF.Tags, flagTags, "t", []string{},
		"Filter distributions by tags; values accept globs. `'Env=prod:stg,Team=app'`")
	cloudfrontCmd.Flags().Bool(flagRegex, false,
		"Treat --domains and --origins patterns as Go regular expressions instead of globs.")
	cloudfrontCmd.Flags().String(flagSort, "id", sortHelp("cloudfront", "distributions", "id"))
}

func cloudfrontInitViper() error {
	return bindFlags(cloudfrontCmd, map[string]string{
		labelCloudFrontAll:   flagAll,
		labelCloudFrontSort:  flagSort,
		labelCloudFrontRegex: flagRegex,
	})
}
