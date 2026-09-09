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
)

const (
	labelVpcAll  = "vpc.all"
	labelVpcSort = "vpc.sort"

	// flagCIDRs names the CIDR filter flag shared by the vpc, subnet and ec2 commands.
	flagCIDRs = "cidrs"
)

// vpcFilters represents the filters for the vpc command.
//
// The filters are used to filter the results.
// common.StructToFilters is used to convert the struct to a map[string][]string.
// The AWS filter names must be present in the struct tag `filter:"filter-name"`.
type vpcFilters struct {
	IDs       []string `filter:"vpc-id"`
	Names     []string `filter:"tag:Name"`
	Tags      []string `filter:"tag"`
	TagsKey   []string `filter:"tag-key"`
	CIDRs     []string `filter:"cidr-block-association.cidr-block"`
	States    []string `filter:"state"`
	IsDefault []string `filter:"is-default"`
	OwnerIDs  []string `filter:"owner-id"`
}

var vpcF = vpcFilters{}

// vpcCmd represents the vpc command.
var vpcCmd = &cobra.Command{
	Use:   "vpc",
	Short: "Search for VPCs.",
	Long: `
Search for VPCs.
You can search VPCs using the following filters:
  ids, names, tags, tags-key, cidrs, states, default, owner-ids.
You can use multiple values for each filter, separated by comma.
Example: --ids vpc-1230456078901,vpc-1230456078902

A CIDR matches when it is exactly one of the IPv4 CIDR blocks associated with the VPC,
primary or secondary. Example: --cidrs 10.0.0.0/16

You can use multiple filters at same time, for example:
	awss vpc -n 'prod-*' -s available

Use --all to search for all VPCs without any filter.
This flag cannot be combined with other filters.

(You can use the wildcard '*' to search for all values in a filter)
`,
	RunE: vpcRunE,
}

// vpcFilterFlags lists all VPC filter flag names for mutual exclusivity with --all.
var vpcFilterFlags = []string{
	flagIDs, flagNames, flagTags, flagTagsKey, flagCIDRs,
	"states", "default", "owner-ids",
}

func vpcRunE(cmd *cobra.Command, _ []string) error {
	if err := common.CheckCIDRs(vpcF.CIDRs); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		allLabel:    labelVpcAll,
		sortLabel:   labelVpcSort,
		filterFlags: vpcFilterFlags,
	}, nil, vpcF.Tags, vpcF)
}

func vpcInitFlags() {
	rootCmd.AddCommand(vpcCmd)

	vpcCmd.Flags().BoolP("all", "a", false,
		"Search for all VPCs without any filter. Cannot be combined with other filters.")
	vpcCmd.Flags().StringSliceVarP(&vpcF.IDs, flagIDs, "i", []string{},
		"Filter VPCs by IDs. `vpc-1230456078901,vpc-1230456078902`")
	vpcCmd.Flags().StringSliceVarP(&vpcF.Names, flagNames, "n", []string{},
		"Filter VPCs by names. It searches using the 'tag:Name'. `vpc-1,vpc-2`")
	vpcCmd.Flags().StringSliceVarP(&vpcF.Tags, flagTags, "t", []string{},
		"Filter VPCs by tags. `'Key=Value1:Value2,Environment=Production'`")
	vpcCmd.Flags().StringSliceVarP(&vpcF.TagsKey, flagTagsKey, "k", []string{},
		"Filter VPCs by tags key. `Key,Environment`")
	vpcCmd.Flags().StringSliceVarP(&vpcF.CIDRs, flagCIDRs, "c", []string{},
		"Filter VPCs by associated IPv4 CIDR blocks. Exact match. `10.0.0.0/16,10.1.0.0/16`")
	vpcCmd.Flags().StringSliceVarP(&vpcF.States, "states", "s", []string{},
		"Filter VPCs by state. `pending,available`")
	vpcCmd.Flags().StringSliceVarP(&vpcF.IsDefault, "default", "d", []string{},
		"Filter VPCs by whether they are the default VPC. `true,false`")
	vpcCmd.Flags().StringSliceVarP(&vpcF.OwnerIDs, "owner-ids", "o", []string{},
		"Filter VPCs by owner account IDs. `123456789012`")
	vpcCmd.Flags().String("sort", "name", sortHelp("vpc", "VPCs", "name"))
}

func vpcInitViper() error {
	return bindFlags(vpcCmd, map[string]string{
		labelVpcAll:  flagAll,
		labelVpcSort: flagSort,
	})
}
