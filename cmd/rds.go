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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelRdsAll   = "rds.all"
	labelRdsSort  = "rds.sort"
	labelRdsRegex = "rds.regex"
)

// rdsFilters represents the filters for the rds command.
//
// db-instance-id, engine and db-cluster-id are DescribeDBInstances filters. DescribeDBInstances
// cannot filter on the others, so search/rds matches them on the results.
type rdsFilters struct {
	IDs            []string `filter:"db-instance-id"`
	Engines        []string `filter:"engine"`
	Clusters       []string `filter:"db-cluster-id"`
	Names          []string `filter:"name"`
	EngineVersions []string `filter:"engine-version"`
	Tags           []string `filter:"tag"`
}

var rdsF = rdsFilters{}

// rdsCmd represents the rds command.
var rdsCmd = &cobra.Command{
	Use:   "rds",
	Short: "Search for RDS DB instances.",
	Long: `
Search for RDS DB instances, including the instances of Aurora clusters.

AWS filters the request by --ids, --engines and --clusters. AWS cannot filter on the others, so
awss matches them on the results:
  --names            identifier patterns: globs, or Go regular expressions with --regex
  --engine-versions  globs on the engine version: '13*' finds every 13.x
  --tags             'Key=Value1:Value2,Other=Value': every key must match, the values of one
                     key are alternatives, and values accept globs

Filters combine with AND. PostgreSQL 13, for example:
	awss rds -e postgres -V '13*'

Use --all to search for all DB instances without any filter. This flag cannot be combined with
other filters.
`,
	Args: cobra.NoArgs,
	RunE: rdsRunE,
}

// rdsFilterFlags lists all rds filter flag names for mutual exclusivity with --all.
var rdsFilterFlags = []string{flagIDs, "engines", "clusters", flagNames, "engine-versions", flagTags}

func rdsRunE(cmd *cobra.Command, _ []string) error {
	if err := checkRDSPatterns(rdsF.Names, rdsF.EngineVersions, rdsF.Tags, viper.GetBool(labelRdsRegex)); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		allLabel:    labelRdsAll,
		sortLabel:   labelRdsSort,
		regexLabel:  labelRdsRegex,
		filterFlags: rdsFilterFlags,
	}, nil, nil, rdsF)
}

// checkRDSPatterns validates the patterns awss matches in the rds and rds-cluster searches, before
// any AWS call.
func checkRDSPatterns(names, versions, tags []string, regex bool) error {
	if _, err := common.NewMatcher(names, regex); err != nil {
		return err
	}
	if _, err := common.NewMatcher(versions, false); err != nil {
		return fmt.Errorf("engine versions: %w", err)
	}
	if _, err := common.NewTagMatcher(tags); err != nil {
		return err
	}
	return nil
}

func rdsInitFlags() {
	rootCmd.AddCommand(rdsCmd)

	rdsCmd.Flags().BoolP(flagAll, "a", false,
		"Search for all DB instances without any filter. Cannot be combined with other filters.")
	rdsCmd.Flags().StringSliceVarP(&rdsF.IDs, flagIDs, "i", []string{},
		"Filter DB instances by identifier (AWS). `db-1,db-2`")
	rdsCmd.Flags().StringSliceVarP(&rdsF.Engines, "engines", "e", []string{},
		"Filter DB instances by engine (AWS). `postgres,aurora-postgresql,mysql`")
	rdsCmd.Flags().StringSliceVarP(&rdsF.Clusters, "clusters", "c", []string{},
		"Filter DB instances by the cluster they belong to (AWS). `cluster-1,cluster-2`")
	rdsCmd.Flags().StringSliceVarP(&rdsF.Names, flagNames, "n", []string{},
		"Filter DB instances by identifier patterns (awss). Globs by default, see --regex. `'app-*,*-prd'`")
	rdsCmd.Flags().StringSliceVarP(&rdsF.EngineVersions, "engine-versions", "V", []string{},
		"Filter DB instances by engine version globs (awss). `'13*,16.4'`")
	rdsCmd.Flags().StringSliceVarP(&rdsF.Tags, flagTags, "t", []string{},
		"Filter DB instances by tags (awss); values accept globs. `'Env=prod:stg,Team=app'`")
	rdsCmd.Flags().Bool(flagRegex, false,
		"Treat --names patterns as Go regular expressions instead of globs.")
	rdsCmd.Flags().String(flagSort, "id", sortHelp("rds", "DB instances", "id"))
}

func rdsInitViper() error {
	return bindFlags(rdsCmd, map[string]string{
		labelRdsAll:   flagAll,
		labelRdsSort:  flagSort,
		labelRdsRegex: flagRegex,
	})
}
