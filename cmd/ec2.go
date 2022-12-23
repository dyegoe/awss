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

const (
	// viperEC2Sort is the Viper key for the ec2.sort used by the application. It is set by the flag --sort.
	viperEC2Sort = "ec2.sort"
)

// ec2Filters represents the filters for the eni command.
//
// The filters are used to filter the results.
// common.StructToFilters is used to convert the struct to a map[string][]string.
// The AWS filter names must be present in the struct tag `filter:"filter-name"`.
type ec2Filters struct {
	Ids               []string `filter:"instance-id"`
	Names             []string `filter:"tag:Name"`
	Tags              []string `filter:"tag"`
	TagsKey           []string `filter:"tag-key"`
	InstanceTypes     []string `filter:"instance-type"`
	InstanceStates    []string `filter:"instance-state-name"`
	AvailabilityZones []string `filter:"availability-zone"`
	PrivateIPs        []net.IP `filter:"network-interface.addresses.private-ip-address"`
	PublicIPs         []net.IP `filter:"network-interface.addresses.association.public-ip"`
}

var ec2F = ec2Filters{}

// ec2Cmd represents the ec2 command
var ec2Cmd = &cobra.Command{
	Use:   "ec2",
	Short: "Search for EC2 instances.",
	Long: `
Search for EC2 instances.
You can search EC2 instances using the following filters: ids, names, tags, instance-types, availability-zones, instance-states, private-ips and public-ips.
You can use multiple values for each filter, separated by comma. Example: --names 'Name1,Name2'

You can use multiple filters at same time, for example:
	awss ec2 -n '*' -t 'Key=Value1:Value2,Environment=Production' -T t2.micro,t2.small -z a,b -s running,stopped

(You can use the wildcard '*' to search for all values in a filter)
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if the availability zones are valid
		if err := checkAvailabilityZones(ec2F.AvailabilityZones); err != nil {
			return err
		}

		// Check if the tags are valid
		if _, err := common.ParseTags(ec2F.Tags); err != nil {
			return err
		}

		// Check if the sort is valid
		if err := search.CheckSortField(cmd.Name(), viper.GetString(viperEC2Sort)); err != nil {
			return err
		}

		// Convert the struct to a map[string][]string to be used as filters
		filters, err := common.StructToFilters(ec2F)
		if err != nil {
			return err
		}

		// Execute the search
		err = search.Execute(
			cmd.Name(),
			viper.GetStringSlice(viperProfiles),
			viper.GetStringSlice(viperRegions),
			filters,
			viper.GetString(viperEC2Sort),
			viper.GetString(viperOutput),
			viper.GetBool(viperShowEmpty),
			viper.GetBool(viperShowTags),
		)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ec2Cmd)

	ec2Cmd.Flags().StringSliceVarP(&ec2F.Ids, "ids", "i", []string{}, "Filter EC2 instances by ids. `i-1230456078901,i-1230456078902`")
	ec2Cmd.Flags().StringSliceVarP(&ec2F.Names, "names", "n", []string{}, "Filter EC2 instances by names. It searchs using the 'tag:Name'. `instance-1,instance-2`")
	ec2Cmd.Flags().StringSliceVarP(&ec2F.Tags, "tags", "t", []string{}, "Filter EC2 instances by tags. `'Key=Value1:Value2,Environment=Production'`")
	ec2Cmd.Flags().StringSliceVarP(&ec2F.TagsKey, "tags-key", "k", []string{}, "Filter EC2 instances by tags key. `Key,Environment`")
	ec2Cmd.Flags().StringSliceVarP(&ec2F.InstanceTypes, "instance-types", "T", []string{}, "Filter EC2 instances by instance type. `t2.micro,t2.small`")
	ec2Cmd.Flags().StringSliceVarP(&ec2F.AvailabilityZones, "availability-zones", "z", []string{}, "Filter EC2 instances by availability zones. It will append to current region. `a,b`")
	ec2Cmd.Flags().StringSliceVarP(&ec2F.InstanceStates, "instance-states", "s", []string{}, "Filter EC2 instances by instance state. `running,stopped`")
	ec2Cmd.Flags().IPSliceVarP(&ec2F.PrivateIPs, "private-ips", "p", []net.IP{}, "Filter EC2 instances by private IPs. `172.16.0.1,172.17.1.254`")
	ec2Cmd.Flags().IPSliceVarP(&ec2F.PublicIPs, "public-ips", "P", []net.IP{}, "Filter EC2 instances by public IPs. `52.28.19.20,52.30.31.32`")
	ec2Cmd.Flags().String("sort", "name", "Sort EC2 instances by id, name, type, az, state, private-ip or public-ip. `name`")
	// Bind flags to viper
	viper.BindPFlag(viperEC2Sort, ec2Cmd.Flags().Lookup("sort"))
}
