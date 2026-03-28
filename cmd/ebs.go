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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelEbsAll            = "ebs.all"
	labelEbsSort           = "ebs.sort"
	labelEbsNoInstanceName = "ebs.no-instance-name"
)

// ebsFilters represents the filters for the ebs command.
//
// The filters are used to filter the results.
// common.StructToFilters is used to convert the struct to a map[string][]string.
// The AWS filter names must be present in the struct tag `filter:"filter-name"`.
type ebsFilters struct {
	Ids               []string `filter:"volume-id"`
	Tags              []string `filter:"tag"`
	TagsKey           []string `filter:"tag-key"`
	AvailabilityZones []string `filter:"availability-zone"`
	Statuses          []string `filter:"status"`
	VolumeTypes       []string `filter:"volume-type"`
	InstanceIDs       []string `filter:"attachment.instance-id"`
	Encrypted         []string `filter:"encrypted"`
}

var ebsF = ebsFilters{}

// ebsCmd represents the ebs command.
var ebsCmd = &cobra.Command{
	Use:   "ebs",
	Short: "Search for EBS volumes.",
	Long: `
Search for EBS volumes.
You can search EBS volumes using the following filters:
  ids, tags, tags-key, availability-zones, statuses, volume-types, instance-ids, encrypted.
You can use multiple values for each filter, separated by comma.
Example: --ids vol-1230456078901,vol-1230456078902

You can use multiple filters at same time, for example:
	awss ebs -s available,in-use -z a,b

Use --all to search for all EBS volumes without any filter.
This flag cannot be combined with other filters.

(You can use the wildcard '*' to search for all values in a filter)
`,
	RunE: ebsRunE,
}

// ebsFilterFlags lists all EBS filter flag names for mutual exclusivity with --all.
var ebsFilterFlags = []string{
	"ids", "tags", "tags-key", "availability-zones",
	"statuses", "volume-types", "instance-ids", "encrypted",
}

func ebsRunE(cmd *cobra.Command, args []string) error {
	return runSearch(
		cmd, labelEbsAll, labelEbsSort, labelEbsNoInstanceName,
		ebsFilterFlags, ebsF.AvailabilityZones, ebsF.Tags, ebsF,
	)
}

func ebsInitFlags() {
	rootCmd.AddCommand(ebsCmd)

	ebsCmd.Flags().BoolP("all", "a", false,
		"Search for all EBS volumes without any filter. Cannot be combined with other filters.")
	ebsCmd.Flags().StringSliceVarP(&ebsF.Ids, "ids", "i", []string{},
		"Filter EBS volumes by IDs. `vol-1230456078901,vol-1230456078902`")
	ebsCmd.Flags().StringSliceVarP(&ebsF.Tags, "tags", "t", []string{},
		"Filter EBS volumes by tags. `'Key=Value1:Value2,Environment=Production'`")
	ebsCmd.Flags().StringSliceVarP(&ebsF.TagsKey, "tags-key", "k", []string{},
		"Filter EBS volumes by tags key. `Key,Environment`")
	ebsCmd.Flags().StringSliceVarP(&ebsF.AvailabilityZones, "availability-zones", "z", []string{},
		"Filter EBS volumes by availability zones. It will append to current region. `a,b`")
	ebsCmd.Flags().StringSliceVarP(&ebsF.Statuses, "statuses", "s", []string{},
		"Filter EBS volumes by status. `available,in-use,creating,deleting,deleted,error`")
	ebsCmd.Flags().StringSliceVarP(&ebsF.VolumeTypes, "volume-types", "T", []string{},
		"Filter EBS volumes by volume type. `gp2,gp3,io1,io2,st1,sc1,standard`")
	ebsCmd.Flags().StringSliceVarP(&ebsF.InstanceIDs, "instance-ids", "I", []string{},
		"Filter EBS volumes by attached instance IDs. `i-1230456078901,i-1230456078902`")
	ebsCmd.Flags().StringSliceVarP(&ebsF.Encrypted, "encrypted", "e", []string{},
		"Filter EBS volumes by encryption. `true,false`")
	ebsCmd.Flags().String("sort", "id",
		"Sort EBS volumes by id, size, type, state, az, encrypted, instance-id, instance-name or device. `id`")
	ebsCmd.Flags().Bool("no-instance-name", false,
		"Skip the instance name lookup to speed up the EBS volume search.")
}

func ebsInitViper() error {
	if err := viper.BindPFlag(labelEbsAll, ebsCmd.Flags().Lookup("all")); err != nil {
		return fmt.Errorf("error binding flag: %w", err)
	}
	if err := viper.BindPFlag(labelEbsSort, ebsCmd.Flags().Lookup("sort")); err != nil {
		return fmt.Errorf("error binding flag: %w", err)
	}
	if err := viper.BindPFlag(labelEbsNoInstanceName, ebsCmd.Flags().Lookup("no-instance-name")); err != nil {
		return fmt.Errorf("error binding flag: %w", err)
	}
	return nil
}
