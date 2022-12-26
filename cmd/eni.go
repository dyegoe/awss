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
	"net"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	
(You can use the wildcard '*' to search for all values in a filter)
`,
	RunE: eniRunE,
}

func eniRunE(cmd *cobra.Command, args []string) error {
	if err := checkAvailabilityZones(eniF.AvailabilityZones); err != nil {
		return err
	}

	if _, err := common.ParseTags(eniF.Tags); err != nil {
		return err
	}

	filters, err := common.StructToFilters(eniF)
	if err != nil {
		return err
	}

	err = search.Execute(
		cmd.Name(),
		viper.GetStringSlice(labelProfiles),
		viper.GetStringSlice(labelRegions),
		filters,
		"",
		viper.GetString(labelOutput),
		viper.GetBool(labelShowEmpty),
		viper.GetBool(labelShowTags),
	)
	if err != nil {
		return err
	}

	return nil
}

func eniInitFlags() {
	rootCmd.AddCommand(eniCmd)

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
}
