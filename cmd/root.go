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

// Package cmd enables the CLI commands and flags.
//
// It is based on Cobra and Viper.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dyegoe/awss/common"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// viperOutput is the Viper key for the output used by the application. It is set by the flag --output.
	viperOutput = "output"
	// viperProfiles is the Viper key for the profiles used by the application. It is set by the flag --profiles.
	viperProfiles = "profiles"
	// viperRegions is the Viper key for the regions used by the application. It is set by the flag --regions.
	viperRegions = "regions"
	// viperShowEmpty is the Viper key for the show.empty used by the application. It is set by the flag --show-empty.
	viperShowEmpty = "show.empty"
	// viperShowTags is the Viper key for the show.tags used by the application. It is set by the flag --show-tags.
	viperShowTags = "show.tags"
	// viperAllRegions is the Viper key for the all-regions used by the application. It is set by the config file section all-regions.]
	viperAllRegions = "all-regions"
	// viperConfigName is the Viper default config file name.
	viperConfigName = "config"
	// viperConfigType is the Viper default config file type.
	viperConfigType = "yaml"
	// viperConfigPathCurrent is the Viper default config file path for the current directory.
	viperConfigPathCurrent = "."
	// viperConfigPathHome is the Viper default config file path for the home directory.
	viperConfigPathHome = "$HOME/.awss/"
)

// cfgFile is the config file used by the application. It is set by the flag --config.
var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "awss",
	Short: "AWSS is a CLI tool to make your life easier when searching AWS resources.",
	Long: `
AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.

It is a wrapper written in Go using AWS SDK Go v2.

The work is still in progress and will be updated regularly.
You can find the source code on GitHub:
https://github.com/dyegoe/awss`,
	Version: "0.7.2", // TODO: Remember to update this version when releasing a new version.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Try to read the config file.
		if err := initConfig(); err != nil {
			return err
		}

		// Check if the output is valid.
		if output := viper.GetString(viperOutput); !common.StringInSlice(output, common.ValidOutputs) {
			return fmt.Errorf("output format %s not found", output)
		}

		// Check if the profiles are valid.
		profiles, err := checkProfiles(viper.GetStringSlice(viperProfiles))
		if err != nil {
			return err
		}
		viper.Set(viperProfiles, profiles)

		// Check if the regions are valid.
		regions, err := checkRegions(viper.GetStringSlice(viperRegions))
		if err != nil {
			return err
		}
		viper.Set(viperRegions, regions)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
//
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.awss/config.yaml)")
	// Flags that can be used by all subcommands and be configured in the config file.
	rootCmd.PersistentFlags().StringSlice("profiles", []string{"default"}, "Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. `profile1,profile2`")
	rootCmd.PersistentFlags().StringSlice("regions", []string{"us-east-1"}, "Select a region to perform your API calls. You can pass multiple regions separated by comma. `region1,region2`")
	rootCmd.PersistentFlags().String("output", "table", "Select the output format. `table`, json or json-pretty")
	rootCmd.PersistentFlags().Bool("show-empty", false, "Show/hide empty results. Default is false")
	rootCmd.PersistentFlags().Bool("show-tags", false, "Show/hide Tags column on table output. Default is false")
	// Viper will bind the flags to the config file.
	// This way, you can use the flags or the config file to set the values.
	viper.BindPFlag(viperOutput, rootCmd.PersistentFlags().Lookup("profiles"))
	viper.BindPFlag(viperRegions, rootCmd.PersistentFlags().Lookup("regions"))
	viper.BindPFlag(viperOutput, rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag(viperShowEmpty, rootCmd.PersistentFlags().Lookup("show-empty"))
	viper.BindPFlag(viperShowTags, rootCmd.PersistentFlags().Lookup("show-tags"))
	// Set the default values for other config file options.
	viper.SetDefault(viperAllRegions, []string{
		"eu-central-1",
		"eu-north-1",
		"eu-west-1",
		"eu-west-2",
		"eu-west-3",
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"ca-central-1",
		"sa-east-1",
		"ap-south-1",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-northeast-3",
		"ap-northeast-2",
		"ap-northeast-1",
	})
}

// initConfig reads the config file.
//
// It will search for the config file in the following order:
// 1. The --config flag absolute path. Either a directory or a file.
// 2. The --config flag relative path. Either a directory or a file.
// 3. The --config flag file name. It will search for the file in the current directory or `$HOME/.awss/`
// 4. The config.yaml file in the current directory
// 5. The $HOME/.awss/config.yaml file
func initConfig() error {
	// If a config flag is passed
	if cfgFile != "" {
		// Use the absolute path of the config flag
		abs, err := filepath.Abs(cfgFile)
		if err != nil {
			return err
		}

		// Check if the config flag is a directory and add it to the search path
		fileInfo, err := os.Stat(abs)
		if err == nil && fileInfo.IsDir() {
			viper.AddConfigPath(abs)
		} else { // If it is not a directory, use the file name and the directory
			base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(filepath.Base(abs)))
			path := filepath.Dir(abs)
			viper.SetConfigName(base)
			viper.AddConfigPath(path)
		}
	} else { // If no config flag is passed, use the default config file name
		viper.SetConfigName(viperConfigName)
	}
	// Set the config file type and search path
	viper.SetConfigType(viperConfigType)
	viper.AddConfigPath(viperConfigPathCurrent)
	viper.AddConfigPath(viperConfigPathHome)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	return nil
}

// checkProfiles checks if the profiles are valid.
//
// If the user passes the `all` profile, it will return all the profiles.
// If the user passes a list of profiles, it will check if they are valid and return them.
// It compares the profiles passed by the user with the profiles found in the config file.
func checkProfiles(p []string) ([]string, error) {
	profiles, err := common.GetAwsProfiles()
	if err != nil {
		return nil, err
	}

	if len(p) == 1 && p[0] == "all" {
		return profiles, nil
	}

	// Check if the profiles are valid.
	for _, profile := range p {
		if !common.StringInSlice(profile, profiles) {
			return nil, fmt.Errorf("profile %s not found", profile)
		}
	}
	return p, nil
}

// checkRegions checks if the regions are valid.
//
// If the user passes the `all` region, it will return all the regions.
// If the user passes a list of regions, it will check if they are valid and return them.
// It compares the regions passwd by the user with the all-regions list in the config file.
func checkRegions(r []string) ([]string, error) {
	regions := viper.GetStringSlice(viperAllRegions)

	if len(r) == 1 && r[0] == "all" {
		return regions, nil
	}

	// Check if the regions are valid.
	for _, region := range r {
		if !common.StringInSlice(region, regions) {
			return nil, fmt.Errorf("region %s not found", region)
		}
	}
	return r, nil
}

// checkAvailabilityZones checks if the availability zones are valid.
//
// Availability zones must be only one letter that will be appended to the region name.
// Example: a,b,c,d,e,f for us-east-1
func checkAvailabilityZones(az []string) error {
	// Check if the availability zones are valid.
	for _, zone := range az {
		if len(zone) != 1 {
			return fmt.Errorf("availability zones must be just a letter that will be append to the region: %s", zone)
		}
	}
	return nil
}
