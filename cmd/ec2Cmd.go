/*
Copyright © 2022 Dyego Alexandre Eugenio dyegoe@gmail.com

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
package cmd

import (
	"awss/search"
	"fmt"
	"net"

	"github.com/spf13/cobra"
)

var ec2Ids, ec2Names, ec2Tags []string
var ec2PrivateIps, ec2PublicIps []net.IP

var ec2Cmd = &cobra.Command{
	Use:   "ec2",
	Short: "Search for EC2 instances.",
	Long: `Search for EC2 instances.
You can search EC2 instances using the following filters: ids, names, private-ips, public-ips and tags.
You can use multiple values for each filter, separated by comma, but you can specify just one filter at time.

For example, if you want to search for EC2 instances with the ids i-1230456078901 and i-1230456078902, you can use:
	awss ec2 -i i-1230456078901,i-1230456078902
If you want to search for EC2 instances with the names instance-1 and instance-2, you can use:
	awss ec2 -n instance-1,instance-2
If you want to search for EC2 instances with the private IPs 172.16.0.1 and 172.17.1.254, you can use:
	awss ec2 -p 172.16.0.1,172.17.1.254
If you want to search for EC2 instances with the public IPs 52.28.19.20 and 52.30.31.32, you can use:
	awss ec2 -P 52.28.19.20,52.30.31.32
If you want to search for EC2 instances with the tag Key and the values Value1 and Value2, you can use:
	awss ec2 -t 'Key=Value1:Value2'
If you want to search for EC2 instances with the tag Environment and the value Production, you can use:
	awss ec2 -t 'Environment=Production'
If you want to search for EC2 instances with the tags Key=Value1:Value2 and Environment=Production, you can use:
	awss ec2 -t 'Key=Value1:Value2,Environment=Production'`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var values []string
		var searchBy string

		switch {
		case len(ec2Ids) > 0:
			values = ec2Ids
			searchBy = "ids"
		case len(ec2Names) > 0:
			values = ec2Names
			searchBy = "names"
		case len(ec2PrivateIps) > 0:
			values = ipToString(ec2PrivateIps)
			searchBy = "private-ips"
		case len(ec2PublicIps) > 0:
			values = ipToString(ec2PublicIps)
			searchBy = "public-ips"
		case len(ec2Tags) > 0:
			values = ec2Tags
			searchBy = "tags"
		default:
			return fmt.Errorf("no flags provided. You must provide one of flags listed below")
		}

		err := search.Run(profile, region, output, cmd.Name(), searchBy, values)
		if err != nil {
			return fmt.Errorf("something went wrong while running %s. error: %s", cmd.Name(), err)
		}
		return nil
	},
}

func init() {
	// Set flags for ec2Cmd
	ec2Cmd.Flags().StringSliceVarP(&ec2Ids, "ids", "i", []string{}, "Filter EC2 instances by ids. `i-1230456078901,i-1230456078902`")
	ec2Cmd.Flags().StringSliceVarP(&ec2Names, "names", "n", []string{}, "Filter EC2 instances by names. It searchs using the 'tag:Name'. `instance-1,instance-2`")
	ec2Cmd.Flags().StringSliceVarP(&ec2Tags, "tags", "t", []string{}, "Filter EC2 instances by tags. `'Key=Value1:Value2,Environment=Production'`")
	ec2Cmd.Flags().IPSliceVarP(&ec2PrivateIps, "private-ips", "p", []net.IP{}, "Filter EC2 instances by private IPs. `172.16.0.1,172.17.1.254`")
	ec2Cmd.Flags().IPSliceVarP(&ec2PublicIps, "public-ips", "P", []net.IP{}, "Filter EC2 instances by public IPs. `52.28.19.20,52.30.31.32`")
	// Mark set of flags that can't be used together
	ec2Cmd.MarkFlagsMutuallyExclusive("ids", "names", "private-ips", "public-ips", "tags")
}
