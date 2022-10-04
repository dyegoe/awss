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
	"awss/logger"
	"os"

	"github.com/spf13/cobra"
)

var l = logger.NewLog()

var profile, region string

// parentCmd represents the base command when called without any subcommands
var parentCmd = &cobra.Command{
	Use:   "awss",
	Short: "AWSS is a CLI tool to make your life easier when searching AWS resources.",
	Long: `AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.
It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

You can find the source code on GitHub:
https://github.com/dyegoe/awss`,
	Version: "0.1",
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			l.Errorf("no command provided")
			cmd.Help()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

// init is called before the command is executed and is used to set flags
func init() {
	// Set flags for parentCmd
	parentCmd.PersistentFlags().StringVar(&profile, "profile", "default", "Select the profile from ~/.aws/config")
	parentCmd.PersistentFlags().StringVar(&region, "region", "eu-central-1", "Select a region to perform your API calls")

	// Add subcommands
	parentCmd.AddCommand(ec2Cmd)
}

// Execute calls *cobra.Command.Execute() to start the CLI
func Execute() {
	if err := parentCmd.Execute(); err != nil {
		os.Exit(0)
	}
}
