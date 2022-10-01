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
	"fmt"

	"github.com/spf13/cobra"
)

var elbArns, elbNames, elbDnsNames []string

// elbCmd represents the elb command
var elbCmd = &cobra.Command{
	Use:   "elb",
	Short: "Use it to search across ELBv2.",
	Long: `Use it to search across ELBv2 (Elastic Load Balancer).
	       You can search by ARNs, by names or by DNS names.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not implemented yet.")
		if len(elbArns) > 0 {
			fmt.Println("elbArns:", elbArns)
			return
		}
		if len(elbNames) > 0 {
			fmt.Println("elbNames:", elbNames)
			return
		}
		if len(elbDnsNames) > 0 {
			fmt.Println("elbDnsNames:", elbDnsNames)
			return
		}
		cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(elbCmd)

	elbCmd.Flags().StringSliceVarP(&elbNames, "names", "n", []string{}, "Provide a list of comma-separated names. It searchs using the 'tag:Name'. e.g. --names `instance-1,instance-2`")
	elbCmd.Flags().StringSliceVarP(&elbArns, "privateIps", "p", []string{}, "Provide a list of comma-separated private IPs. e.g. --privateIps `172.16.0.1,172.17.1.254`")
	elbCmd.Flags().StringSliceVarP(&elbDnsNames, "DnsNames", "d", []string{}, "Provide a list of comma-separated public IPs. e.g. --DnsNames `52.28.19.20,52.30.31.32`")
}
