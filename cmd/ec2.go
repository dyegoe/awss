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

var ec2Ids string
var ec2Names string
var ec2PrivateIps string
var ec2PublicIps string

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
		if ec2Ids != "" {
			s.Search("ids", ec2Ids)
			s.JSON()
			return
		}
		if ec2Names != "" {
			s.Search("names", ec2Names)
			s.JSON()
			return
		}
		if ec2PrivateIps != "" {
			s.Search("private-ips", ec2PrivateIps)
			s.JSON()
			return
		}
		if ec2PublicIps != "" {
			s.Search("public-ips", ec2PublicIps)
			s.JSON()
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(ec2Cmd)

	ec2Cmd.Flags().StringVar(&ec2Ids, "ids", "", "Provide a list of comma-separated ids. e.g. --ids `i-00000000000000001,i-00000000000000002`")
	ec2Cmd.Flags().StringVar(&ec2Names, "names", "", "Provide a list of comma-separated names. It searchs using the 'tag:Name'. e.g. --names `instance-1,instance-2`")
	ec2Cmd.Flags().StringVar(&ec2PrivateIps, "private-ips", "", "Provide a list of comma-separated private IPs. e.g. --private-ips `172.16.0.1,172.17.1.254`")
	ec2Cmd.Flags().StringVar(&ec2PublicIps, "public-ips", "", "Provide a list of comma-separated public IPs. e.g. --public-ips `52.28.19.20,52.30.31.32`")
}
