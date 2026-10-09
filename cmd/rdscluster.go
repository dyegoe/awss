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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelRdsClusterAll   = "rds-cluster.all"
	labelRdsClusterSort  = "rds-cluster.sort"
	labelRdsClusterRegex = "rds-cluster.regex"
)

// rdsClusterFilters represents the filters for the rds-cluster command.
//
// db-cluster-id and engine are DescribeDBClusters filters. DescribeDBClusters cannot filter on
// the others, so search/rdscluster matches them on the results.
type rdsClusterFilters struct {
	IDs            []string `filter:"db-cluster-id"`
	Engines        []string `filter:"engine"`
	Names          []string `filter:"name"`
	EngineVersions []string `filter:"engine-version"`
	Tags           []string `filter:"tag"`
}

var rdsClusterF = rdsClusterFilters{}

// rdsClusterCmd represents the rds-cluster command.
var rdsClusterCmd = &cobra.Command{
	Use:   "rds-cluster",
	Short: "Search for RDS DB clusters (Aurora and Multi-AZ DB clusters).",
	Long: `
Search for RDS DB clusters: Aurora and Multi-AZ DB clusters. List the instances of a cluster
with 'awss rds --clusters <cluster>'.

AWS filters the request by --ids and --engines. AWS cannot filter on the others, so awss matches
them on the results:
  --names            identifier patterns: globs, or Go regular expressions with --regex
  --engine-versions  globs on the engine version: '8.0.*' finds every 8.0.x
  --tags             'Key=Value1:Value2,Other=Value': every key must match, the values of one
                     key are alternatives, and values accept globs

The table shows the identifier, engine, version, status, members and VPC; the endpoints, port,
members list and the other details are in --output json.

Use --all to search for all DB clusters without any filter. This flag cannot be combined with
other filters.
`,
	Args: cobra.NoArgs,
	RunE: rdsClusterRunE,
}

// rdsClusterFilterFlags lists all rds-cluster filter flag names for mutual exclusivity with --all.
var rdsClusterFilterFlags = []string{flagIDs, "engines", flagNames, "engine-versions", flagTags}

func rdsClusterRunE(cmd *cobra.Command, _ []string) error {
	f := rdsClusterF
	if err := checkRDSPatterns(f.Names, f.EngineVersions, f.Tags, viper.GetBool(labelRdsClusterRegex)); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		allLabel:    labelRdsClusterAll,
		sortLabel:   labelRdsClusterSort,
		regexLabel:  labelRdsClusterRegex,
		filterFlags: rdsClusterFilterFlags,
	}, nil, nil, rdsClusterF)
}

func rdsClusterInitFlags() {
	rootCmd.AddCommand(rdsClusterCmd)

	rdsClusterCmd.Flags().BoolP(flagAll, "a", false,
		"Search for all DB clusters without any filter. Cannot be combined with other filters.")
	rdsClusterCmd.Flags().StringSliceVarP(&rdsClusterF.IDs, flagIDs, "i", []string{},
		"Filter DB clusters by identifier (AWS). `cluster-1,cluster-2`")
	rdsClusterCmd.Flags().StringSliceVarP(&rdsClusterF.Engines, "engines", "e", []string{},
		"Filter DB clusters by engine (AWS). `aurora-postgresql,aurora-mysql`")
	rdsClusterCmd.Flags().StringSliceVarP(&rdsClusterF.Names, flagNames, "n", []string{},
		"Filter DB clusters by identifier patterns (awss). Globs by default, see --regex. `'app-*,*-prd'`")
	rdsClusterCmd.Flags().StringSliceVarP(&rdsClusterF.EngineVersions, "engine-versions", "V", []string{},
		"Filter DB clusters by engine version globs (awss). `'13*,8.0.*'`")
	rdsClusterCmd.Flags().StringSliceVarP(&rdsClusterF.Tags, flagTags, "t", []string{},
		"Filter DB clusters by tags (awss); values accept globs. `'Env=prod:stg,Team=app'`")
	rdsClusterCmd.Flags().Bool(flagRegex, false,
		"Treat --names patterns as Go regular expressions instead of globs.")
	rdsClusterCmd.Flags().String(flagSort, "id", sortHelp("rds-cluster", "DB clusters", "id"))
}

func rdsClusterInitViper() error {
	return bindFlags(rdsClusterCmd, map[string]string{
		labelRdsClusterAll:   flagAll,
		labelRdsClusterSort:  flagSort,
		labelRdsClusterRegex: flagRegex,
	})
}
