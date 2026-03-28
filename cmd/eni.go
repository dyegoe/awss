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
	"net"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelEniAll            = "eni.all"
	labelEniSort           = "eni.sort"
	labelEniNoInstanceName = "eni.no-instance-name"
)

// eniFilters represents the filters for the eni command.
//
// The filters are used to filter the results.
// common.StructToFilters is used to convert the struct to a map[string][]string.
// The AWS filter names must be present in the struct tag `filter:"filter-name"`.
type eniFilters struct {
	Ids               []string `filter:"network-interface-id"`
	Tags              []string `filter:"tag"`
	TagsKey           []string `filter:"tag-key"`
	InstanceIDs       []string `filter:"attachment.instance-id"`
	AvailabilityZones []string `filter:"availability-zone"`
	PrivateIPs        []net.IP `filter:"addresses.private-ip-address"`
	PublicIPs         []net.IP `filter:"association.public-ip"`
}

var eniF = eniFilters{}

// eniCmd represents the eni command
var eniCmd = &cobra.Command{
	Use:   "eni",
	Short: "Search for ENIs (Elastic Network Interfaces).",
	Long: `
Search for ENIs (Elastic Network Interfaces).
You can search ENIs using the following filters: ids, tags, instance-ids, availability-zones, private-ips, public-ips.
You can use multiple values for each filter, separated by comma. Example: --ids eni-1230456078901,eni-1230456078902

You can use multiple filters at same time, for example:
	awss eni -I i-1230456078901,i-1230456078902 -z a,b

Use --all to search for all ENIs without any filter. This flag cannot be combined with other filters.

(You can use the wildcard '*' to search for all values in a filter)
`,
	RunE: eniRunE,
}

func eniRunE(cmd *cobra.Command, args []string) error {
	return runSearch(
		cmd, labelEniAll, labelEniSort, labelEniNoInstanceName,
		eniFilterFlags, eniF.AvailabilityZones, eniF.Tags, eniF,
	)
}

// eniFilterFlags lists all ENI filter flag names for mutual exclusivity with --all.
var eniFilterFlags = []string{
	"ids", "tags", "tags-key", "instance-ids",
	"availability-zones", "private-ips", "public-ips",
}

func eniInitFlags() {
	rootCmd.AddCommand(eniCmd)

	eniCmd.Flags().BoolP("all", "a", false,
		"Search for all ENIs without any filter. Cannot be combined with other filters.")
	eniCmd.Flags().StringSliceVarP(&eniF.Ids, "ids", "i", []string{},
		"Filter ENIs by ids. `eni-1230456078901,eni-1230456078902`")
	eniCmd.Flags().StringSliceVarP(&eniF.Tags, "tags", "t", []string{},
		"Filter ENIs by tags. `'Key=Value1:Value2,Environment=Production'`")
	eniCmd.Flags().StringSliceVarP(&eniF.TagsKey, "tags-key", "k", []string{},
		"Filter ENIs by tags key. `Key,Environment`")
	eniCmd.Flags().StringSliceVarP(&eniF.InstanceIDs, "instance-ids", "I", []string{},
		"Filter ENIs by instance IDs. `i-1230456078901,i-1230456078902`")
	eniCmd.Flags().StringSliceVarP(&eniF.AvailabilityZones, "availability-zones", "z", []string{},
		"Filter ENIs by availability zones. It will append to current region. `a,b`")
	eniCmd.Flags().IPSliceVarP(&eniF.PrivateIPs, "private-ips", "p", []net.IP{},
		"Filter ENIs by private IPs. `172.16.0.1,172.17.1.254`")
	eniCmd.Flags().IPSliceVarP(&eniF.PublicIPs, "public-ips", "P", []net.IP{},
		"Filter ENIs by public IPs. `52.28.19.20,52.30.31.32`")
	eniCmd.Flags().String("sort", "id",
		"Sort ENIs by id, type, az, status, subnet-id, instance-id or instance-name. `id`")
	eniCmd.Flags().Bool("no-instance-name", false,
		"Skip the instance name lookup to speed up the ENI search.")
}

func eniInitViper() error {
	if err := viper.BindPFlag(labelEniAll, eniCmd.Flags().Lookup("all")); err != nil {
		return fmt.Errorf("error binding flag: %w", err)
	}
	if err := viper.BindPFlag(labelEniSort, eniCmd.Flags().Lookup("sort")); err != nil {
		return fmt.Errorf("error binding flag: %w", err)
	}
	if err := viper.BindPFlag(labelEniNoInstanceName, eniCmd.Flags().Lookup("no-instance-name")); err != nil {
		return fmt.Errorf("error binding flag: %w", err)
	}
	return nil
}
