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
	labelSubnetAll  = "subnet.all"
	labelSubnetSort = "subnet.sort"
)

// subnetFilters represents the filters for the subnet command.
//
// The filters are used to filter the results.
// common.StructToFilters is used to convert the struct to a map[string][]string.
// The AWS filter names must be present in the struct tag `filter:"filter-name"`.
type subnetFilters struct {
	IDs               []string `filter:"subnet-id"`
	Names             []string `filter:"tag:Name"`
	Tags              []string `filter:"tag"`
	TagsKey           []string `filter:"tag-key"`
	VpcIDs            []string `filter:"vpc-id"`
	CIDRs             []string `filter:"cidr-block"`
	AvailabilityZones []string `filter:"availability-zone"`
	States            []string `filter:"state"`
	DefaultForAz      []string `filter:"default-for-az"`
	MapPublicIP       []string `filter:"map-public-ip-on-launch"`
}

var subnetF = subnetFilters{}

// subnetCmd represents the subnet command.
var subnetCmd = &cobra.Command{
	Use:   "subnet",
	Short: "Search for subnets.",
	Long: `
Search for subnets.
You can search subnets using the following filters:
  ids, names, tags, tags-key, vpc-ids, cidrs, availability-zones, states, default-for-az, public-ip-on-launch.
You can use multiple values for each filter, separated by comma.
Example: --ids subnet-1230456078901,subnet-1230456078902

A CIDR matches when it is exactly the IPv4 CIDR block of the subnet. Example: --cidrs 10.0.1.0/24

You can use multiple filters at same time, for example:
	awss subnet -V vpc-1230456078901 -z a,b

Use --all to search for all subnets without any filter.
This flag cannot be combined with other filters.

(You can use the wildcard '*' to search for all values in a filter)
`,
	Args: cobra.NoArgs,
	RunE: subnetRunE,
}

// subnetFilterFlags lists all subnet filter flag names for mutual exclusivity with --all.
var subnetFilterFlags = []string{
	flagIDs, flagNames, flagTags, flagTagsKey, "vpc-ids", flagCIDRs,
	flagAvailabilityZones, "states", "default-for-az", "public-ip-on-launch",
}

func subnetRunE(cmd *cobra.Command, _ []string) error {
	if err := common.CheckCIDRs(subnetF.CIDRs); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		allLabel:    labelSubnetAll,
		sortLabel:   labelSubnetSort,
		filterFlags: subnetFilterFlags,
	}, subnetF.AvailabilityZones, subnetF.Tags, subnetF)
}

func subnetInitFlags() {
	rootCmd.AddCommand(subnetCmd)

	subnetCmd.Flags().BoolP("all", "a", false,
		"Search for all subnets without any filter. Cannot be combined with other filters.")
	subnetCmd.Flags().StringSliceVarP(&subnetF.IDs, flagIDs, "i", []string{},
		"Filter subnets by IDs. `subnet-1230456078901,subnet-1230456078902`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.Names, flagNames, "n", []string{},
		"Filter subnets by names. It searches using the 'tag:Name'. `subnet-1,subnet-2`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.Tags, flagTags, "t", []string{},
		"Filter subnets by tags. `'Key=Value1:Value2,Environment=Production'`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.TagsKey, flagTagsKey, "k", []string{},
		"Filter subnets by tags key. `Key,Environment`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.VpcIDs, "vpc-ids", "V", []string{},
		"Filter subnets by VPC IDs. `vpc-1230456078901,vpc-1230456078902`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.CIDRs, flagCIDRs, "c", []string{},
		"Filter subnets by IPv4 CIDR block. Exact match. `10.0.1.0/24,10.0.2.0/24`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.AvailabilityZones, flagAvailabilityZones, "z", []string{},
		"Filter subnets by availability zones. It will append to current region. `a,b`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.States, "states", "s", []string{},
		"Filter subnets by state. `pending,available`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.DefaultForAz, "default-for-az", "d", []string{},
		"Filter subnets by whether they are the default subnet of their AZ. `true,false`")
	subnetCmd.Flags().StringSliceVarP(&subnetF.MapPublicIP, "public-ip-on-launch", "p", []string{},
		"Filter subnets by whether instances get a public IP on launch. `true,false`")
	subnetCmd.Flags().String("sort", "name", sortHelp("subnet", "subnets", "name"))
}

func subnetInitViper() error {
	return bindFlags(subnetCmd, map[string]string{
		labelSubnetAll:  flagAll,
		labelSubnetSort: flagSort,
	})
}
