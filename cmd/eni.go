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

	"github.com/dyegoe/awss/aws"
	"github.com/spf13/cobra"
)

var eniPrivateIps string
var eniPublicIps string

// eniCmd represents the eni command
var eniCmd = &cobra.Command{
	Use:   "eni",
	Short: "Use it to search across ENIs.",
	Long: `Use it to search across ENIs (Elastic Network Interfaces).
	       You can search by private IPs or public IPs.`,
	Run: func(cmd *cobra.Command, args []string) {
		if eniPrivateIps != "" {
			eniPrivateIpsSearch()
			return
		}
		if eniPublicIps != "" {
			eniPublicIpsSearch()
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(eniCmd)

	eniCmd.Flags().StringVar(&eniPrivateIps, "privateIps", "", "Provide a list of comma-separated private IPs. e.g. --privateIps `172.16.0.1,172.17.1.254`")
	eniCmd.Flags().StringVar(&eniPublicIps, "publicIps", "", "Provide a list of comma-separated public IPs. e.g. --publicIps `52.28.19.20,52.30.31.32`")
}

func eniPrivateIpsSearch() {
	fmt.Println("---- eni ----")
	aws.Print("Profile:", profile)
	aws.Print("Region:", region)
	aws.Print("Private IPs:", eniPrivateIps)
}

func eniPublicIpsSearch() {
	fmt.Println("---- eni ----")
	aws.Print("Profile:", profile)
	aws.Print("Region:", region)
	aws.Print("Public IPs:", eniPublicIps)
}
