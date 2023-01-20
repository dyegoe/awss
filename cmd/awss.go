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

// Package cmd contains the persistent flags and the root command.
//
// The root command does not execute any further action but print Help().
// It contains the persistent flags and persistent pre-run function.
// The persistent flags are used by all the subcommands.
// The persistent pre-run function is executed before the subcommands and does sanity checks.
// The subcommands are in the subdirectories of the search engines and should be imported.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Initialize() error {
	awssCmd := &cobra.Command{
		Use:               "awss",
		Short:             "Search resources in AWS.",
		Long:              `AWSS (AWS Search) is a command line tool to search resources in AWS.`,
		Version:           "0.8.0",
		RunE:              func(c *cobra.Command, args []string) error { return c.Help() },
		PersistentPreRunE: func(c *cobra.Command, args []string) error { return nil },
	}

	initPersistentFlags(awssCmd)

	return awssCmd.Execute()
}

func initPersistentFlags(c *cobra.Command) {
	validOutputs := "table, json, json-pretty"
	pflags := c.PersistentFlags()
	pflags.String("config", "", "config file (default is $HOME/.awss/config.yaml)")
	pflags.StringSlice("profiles", []string{"default"}, "Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. e.g. `profile1,profile2`")   //nolint:lll
	pflags.StringSlice("regions", []string{"us-east-1"}, "Select a region to perform your API calls. You can pass multiple regions separated by comma. e.g. `region1,region2`") //nolint:lll
	pflags.String("output", "table", fmt.Sprintf("Select the output format. Valid outputs are: %s", validOutputs))
	pflags.Bool("show-empty", false, "Show empty resources. Default is false.")
	pflags.Bool("show-tags", false, "Show tags for resources. Default is false.")
}
