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

	"github.com/spf13/cobra"
)

var ec2Tags []string

var ec2Cmd = &cobra.Command{
	Use:   "ec2",
	Short: "Search for EC2 instances.",
	Long:  `Search for EC2 instances.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("no arguments allowed")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		var values []string
		var searchBy string

		switch {
		case len(ec2Tags) > 0:
			values = append(values, ec2Tags...)
			searchBy = "tags"
		default:
			l.Errorf("no flags provided. You must provide one flag.")
			cmd.Help()
			return
		}

		if !search.Run(cmd.Name(), searchBy, profile, region, values) {
			l.Errorf("something went wrong while running %s", cmd.Name())
		}
	},
}

func init() {
	// Set flags for ec2Cmd
	ec2Cmd.Flags().StringSliceVarP(&ec2Tags, "tags", "t", []string{}, "Filter EC2 instances by tags. Example: -t `'Key=Value1:Value2'` -t 'Environment=Production'")
}
