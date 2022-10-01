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
	"log"

	"github.com/spf13/cobra"
)

var profile, region, output string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "awss",
	Short: "CLI command to make the resource search easier on AWS.",
	Long:  `CLI command to make the resource search easier on AWS.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln("[ERROR] ", err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "default", "Select the profile from ~/.aws/config")
	rootCmd.PersistentFlags().StringVar(&region, "region", "eu-central-1", "Select a region to perform your API calls")
	rootCmd.PersistentFlags().StringVar(&output, "output", "json", "Select the output format. Options: table, json")
}
