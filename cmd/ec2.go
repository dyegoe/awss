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
	"github.com/dyegoe/awss/search"
	"github.com/spf13/cobra"
)

var ec2Ids, ec2Names, ec2Tags, ec2PrivateIps, ec2PublicIps []string

// ec2Cmd represents the ec2 command
var ec2Cmd = &cobra.Command{
	Use:   "ec2",
	Short: "Use it to search across EC2 instances.",
	Long: `Use it to search across EC2 instances.
	       You can search by ids, names, private IPs or public IPs.`,
	Run: func(cmd *cobra.Command, args []string) {
		s := search.Instances{
			Profile: profile,
			Region:  region,
		}
		if len(ec2Ids) > 0 {
			s.Search("ids", ec2Ids)
			s.Print(output)
			return
		}
		if len(ec2Names) > 0 {
			s.Search("names", ec2Names)
			s.Print(output)
			return
		}
		if len(ec2PrivateIps) > 0 {
			s.Search("private-ips", ec2PrivateIps)
			s.Print(output)
			return
		}
		if len(ec2PublicIps) > 0 {
			s.Search("public-ips", ec2PublicIps)
			s.Print(output)
			return
		}
		if len(ec2Tags) > 0 {
			s.Search("tags", ec2Tags)
			s.Print(output)
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(ec2Cmd)

	ec2Cmd.Flags().StringSliceVarP(&ec2Ids, "ids", "i", []string{}, "Provide a list of comma-separated ids. e.g. --ids `i-1230456078901,i-1230456078902`")
	ec2Cmd.Flags().StringSliceVarP(&ec2Names, "names", "n", []string{}, "Provide a list of comma-separated names. It searchs using the 'tag:Name'. e.g. --names `instance-1,instance-2`")
	ec2Cmd.Flags().StringSliceVarP(&ec2Tags, "tags", "t", []string{}, "Provide a list of comma-separated tags. e.g. --tags `tag1=value1:value2,tag2=value3`")
	ec2Cmd.Flags().StringSliceVarP(&ec2PrivateIps, "private-ips", "p", []string{}, "Provide a list of comma-separated private IPs. e.g. --private-ips `172.16.0.1,172.17.1.254`")
	ec2Cmd.Flags().StringSliceVarP(&ec2PublicIps, "public-ips", "P", []string{}, "Provide a list of comma-separated public IPs. e.g. --public-ips `52.28.19.20,52.30.31.32`")
}
