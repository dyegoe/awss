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

	"github.com/dyegoe/awss/common"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelConfig         = "config"
	labelProfiles       = "profiles"
	labelRegions        = "regions"
	labelOutput         = "output"
	labelShowEmptyCobra = "show-empty"
	labelShowEmpty      = "show.empty"
	labelShowTagsCobra  = "show-tags"
	labelShowTags       = "show.tags"
	labelAllProfiles    = "all-profiles"
	labelAllRegions     = "all-regions"
)

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
	Version:           "0.7.2", // TODO: Remember to update this version when releasing a new version.
	PersistentPreRunE: persistentPreRun,
}

// Execute adds all child commands to the root command and sets flags appropriately.
//
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	initFlags()
	ec2InitFlags()
	eniInitFlags()

	if err := initViper(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := ec2InitViper(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// persistentPreRun is executed before any command.
func persistentPreRun(cmd *cobra.Command, args []string) error {
	cfg, err := cmd.Flags().GetString(labelConfig)
	if err != nil {
		return err
	}

	if err := initConfig(cfg); err != nil {
		return err
	}

	profiles, err := checkProfiles(viper.GetStringSlice(labelProfiles))
	if err != nil {
		return err
	}
	viper.Set(labelProfiles, profiles)

	regions, err := checkRegions(viper.GetStringSlice(labelRegions), viper.GetStringSlice(labelAllRegions))
	if err != nil {
		return err
	}
	viper.Set(labelRegions, regions)

	return nil
}

// initFlags initializes cobras flags.
func initFlags() {
	validOutputs, _ := common.ValidOutputs("")

	rootCmd.PersistentFlags().String(labelConfig, "",
		"config file path (default is $HOME/.awss/config.yaml)")
	rootCmd.PersistentFlags().StringSlice(labelProfiles, []string{"default"},
		"Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. e.g. `profile1,profile2`")
	rootCmd.PersistentFlags().StringSlice(labelRegions, []string{"us-east-1"},
		"Select a region to perform your API calls. You can pass multiple regions separated by comma. e.g. `region1,region2`")
	rootCmd.PersistentFlags().String(labelOutput, "table",
		fmt.Sprintf("Select the output format. Valid outputs are: %s", validOutputs))
	rootCmd.PersistentFlags().Bool(labelShowEmptyCobra, false,
		"Show empty resources. Default is false.")
	rootCmd.PersistentFlags().Bool(labelShowTagsCobra, false,
		"Show tags for resources. Default is false.")
}

// initViper binds the flags to viper.
func initViper() error {
	allRegionsDefault := []string{
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
	}

	if err := viper.BindPFlag(labelProfiles, rootCmd.PersistentFlags().Lookup(labelProfiles)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelProfiles, err)
	}
	if err := viper.BindPFlag(labelRegions, rootCmd.PersistentFlags().Lookup(labelRegions)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelRegions, err)
	}
	if err := viper.BindPFlag(labelOutput, rootCmd.PersistentFlags().Lookup(labelOutput)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelOutput, err)
	}
	if err := viper.BindPFlag(labelShowEmpty, rootCmd.PersistentFlags().Lookup(labelShowEmptyCobra)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelShowEmpty, err)
	}
	if err := viper.BindPFlag(labelShowTags, rootCmd.PersistentFlags().Lookup(labelShowTagsCobra)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelShowTags, err)
	}
	viper.SetDefault(labelAllRegions, allRegionsDefault)

	return nil
}

// initConfig reads the config file.
//
// It will search for the config file in the following order:
// 1. --config flag absolute/relative path to a file.
// 2. $HOME/.awss/config.yaml file
func initConfig(cfg string) error {
	var f string

	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		f = filepath.Join(home, ".awss", "config.yaml")
	}
	if cfg != "" {
		f = cfg
	}

	_, err := os.Stat(f)
	if os.IsNotExist(err) && cfg == "" {
		return nil
	}
	if os.IsNotExist(err) && cfg != "" {
		return fmt.Errorf("config file not found: %s", f)
	}

	viper.SetConfigFile(f)

	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	return nil
}

// getAwsProfiles returns the profiles found in the config file.
//
// We use a variable to mock the function in the tests.
var getAwsProfiles = common.GetAwsProfiles

// checkProfiles checks if the profiles are valid.
//
// If the user passes the `all` profile, it will return all the profiles.
// If the user passes a list of profiles, it will check if they are valid and return them.
// It compares the profiles passed by the user with the profiles found in the config file.
func checkProfiles(profiles []string) ([]string, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profile selected")
	}

	awsProfiles, err := getAwsProfiles()
	if err != nil {
		return nil, err
	}

	if len(profiles) == 1 && profiles[0] == "all" {
		return awsProfiles, nil
	}

	for _, profile := range profiles {
		if !common.StringInSlice(profile, awsProfiles) {
			return nil, fmt.Errorf("profile %s not found", profile)
		}
	}
	return profiles, nil
}

// checkRegions checks if the regions are valid.
//
// If the user passes the `all` region, it will return all the regions.
// If the user passes a list of regions, it will check if they are valid and return them.
// It compares the regions passwd by the user with the all-regions list in the config file.
func checkRegions(regions, allRegions []string) ([]string, error) {
	if len(regions) == 0 {
		return nil, fmt.Errorf("no region selected")
	}

	if len(regions) == 1 && regions[0] == "all" {
		return allRegions, nil
	}

	for _, region := range regions {
		if !common.StringInSlice(region, allRegions) {
			return nil, fmt.Errorf("region %s not found", region)
		}
	}
	return regions, nil
}

// checkAvailabilityZones checks if the availability zones are valid.
//
// Availability zones must be only one letter that will be appended to the region name.
// Example: a,b,c,d,e,f for us-east-1
func checkAvailabilityZones(az []string) error {
	if len(az) == 0 {
		return fmt.Errorf("no availability zone selected")
	}

	for _, zone := range az {
		validAvailabilityZones := []string{"a", "b", "c", "d", "e", "f"}
		if len(zone) != 1 {
			return fmt.Errorf("availability zones must be just a letter. It will be append to the region: %s", zone)
		}
		if !common.StringInSlice(zone, validAvailabilityZones) {
			return fmt.Errorf("availability zone %s not found. Valid options are: %s",
				zone, common.StringSliceToString(validAvailabilityZones, ", "))
		}
	}
	return nil
}
