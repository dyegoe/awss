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
	"net"
	"os"

	"github.com/spf13/cobra"
)

var output string
var profile, region []string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "awss",
	Short: "AWSS is a CLI tool to make your life easier when searching AWS resources.",
	Long: `AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.
It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

This command uses the credentials stored in ~/.aws/credentials and ~/.aws/config files.

The 'default' profile is used if no profile is provided.
The provided profile must be present in ~/.aws/credentials and ~/.aws/config files.
If you would like to iterate over multiple profiles, you can pass them separated by comma. Example: --profile profile1,profile2.
You can also pass 'all' to iterate over all profiles.

The default region is 'eu-central-1'.
If you would like to iterate over multiple regions, you can pass them separated by comma. Example: --region region1,region2.
You can also pass 'all' to iterate over all regions.

You can find the source code on GitHub:
https://github.com/dyegoe/awss`,
	Version:   "0.2.1",
	ValidArgs: []string{"ec2"},
	Args:      cobra.ExactValidArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := checkOutput(); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Help()
		return nil
	},
}

// init is called before the command is executed and is used to set flags
func init() {
	// Set flags for rootCmd
	rootCmd.PersistentFlags().StringSliceVar(&profile, "profile", []string{"default"}, "Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. `profile1,profile2`")
	rootCmd.PersistentFlags().StringSliceVar(&region, "region", []string{"eu-central-1"}, "Select a region to perform your API calls. You can pass multiple regions separated by comma. `region1,region2`")
	rootCmd.PersistentFlags().StringVar(&output, "output", "table", "Select the output format. `table`, json or json-pretty")

}

// checkOutput checks if the output is valid
func checkOutput() error {
	if output != "table" && output != "json" && output != "json-pretty" {
		return fmt.Errorf("invalid output format. Please use 'table', 'json' or 'json-pretty'")
	}
	return nil
}

// ipToString converts a slice of net.IP to a slice of string
func ipToString(ip []net.IP) []string {
	var ips []string
	for _, i := range ip {
		ips = append(ips, i.String())
	}
	return ips
}

// Execute calls *cobra.Command.Execute() to start the CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
