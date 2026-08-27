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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"

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
	labelTagsKeysCobra  = "show-tags-keys"
	labelTagsKeys       = "show.tags.keys"
	labelAllRegions     = "all-regions"

	// flagIDs, flagTags, flagTagsKey, and flagAvailabilityZones name the flags
	// shared by the ec2, eni, and ebs commands' sort-field lists.
	flagIDs               = "ids"
	flagTags              = "tags"
	flagTagsKey           = "tags-key"
	flagAvailabilityZones = "availability-zones"
)

// version is overridden at build time via -ldflags.
var version = "dev"

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
	Version:           version,
	PersistentPreRunE: persistentPreRun,
}

// Execute adds all child commands to the root command and sets flags appropriately.
//
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	initFlags()
	ec2InitFlags()
	eniInitFlags()
	ebsInitFlags()

	if err := initViper(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := ec2InitViper(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := eniInitViper(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := ebsInitViper(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// persistentPreRun is executed before any command.
func persistentPreRun(cmd *cobra.Command, _ []string) error {
	cfg, err := cmd.Flags().GetString(labelConfig)
	if err != nil {
		return err
	}

	if err := initConfig(cfg); err != nil {
		return err
	}

	profiles, err := common.CheckProfiles(viper.GetStringSlice(labelProfiles))
	if err != nil {
		return err
	}
	viper.Set(labelProfiles, profiles)

	regions, err := common.CheckRegions(viper.GetStringSlice(labelRegions), viper.GetStringSlice(labelAllRegions))
	if err != nil {
		return err
	}
	viper.Set(labelRegions, regions)

	if _, valid := common.ValidOutputs(viper.GetString(labelOutput)); !valid {
		validList, _ := common.ValidOutputs("")
		return fmt.Errorf("invalid output format: %s. Valid outputs are: %s",
			viper.GetString(labelOutput), validList)
	}

	return nil
}

// initFlags initializes cobras flags.
func initFlags() {
	validOutputs, _ := common.ValidOutputs("")

	rootCmd.PersistentFlags().String(labelConfig, "",
		"config file path (default is $HOME/.awss/config.yaml)")
	rootCmd.PersistentFlags().StringSlice(labelProfiles, []string{},
		"Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. "+
			"e.g. `profile1,profile2`. If not set, falls back to the AWS SDK's default credential "+
			"resolution (AWS_PROFILE, static env credentials, or the `default` profile).")
	rootCmd.PersistentFlags().StringSlice(labelRegions, []string{},
		fmt.Sprintf(
			"Select a region to perform your API calls. You can pass multiple regions separated by comma. "+
				"e.g. `region1,region2`. If not set, falls back to AWS_REGION/AWS_DEFAULT_REGION, or %s.",
			common.DefaultRegion,
		))
	rootCmd.PersistentFlags().String(labelOutput, "table",
		fmt.Sprintf("Select the output format. Valid outputs are: %s", validOutputs))
	rootCmd.PersistentFlags().Bool(labelShowEmptyCobra, false,
		"Show empty resources. Default is false.")
	rootCmd.PersistentFlags().Bool(labelShowTagsCobra, false,
		"Show tags for resources. Default is false.")
	rootCmd.PersistentFlags().StringSlice(labelTagsKeysCobra, []string{},
		"Restrict the tags shown to these keys. Implies --show-tags. e.g. `Name,Environment`")
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
	if err := viper.BindPFlag(labelTagsKeys, rootCmd.PersistentFlags().Lookup(labelTagsKeysCobra)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelTagsKeys, err)
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

	info, err := os.Stat(f)
	if os.IsNotExist(err) && cfg == "" {
		return nil
	}
	if os.IsNotExist(err) && cfg != "" {
		return fmt.Errorf("config file not found: %s", f)
	}
	// check if the path is a directory
	if info.IsDir() {
		return fmt.Errorf("config file is a directory: %s", f)
	}

	viper.SetConfigFile(f)

	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	return nil
}

// buildFilters validates and builds the filter map for a subcommand.
//
// When allFlag is true, it checks that no filter flags were set and returns an empty map.
// Otherwise, it validates availability zones and tags, then converts the filter struct.
func buildFilters(
	cmd *cobra.Command,
	allFlag bool,
	filterFlags []string,
	azs, tags []string,
	filterStruct interface{},
) (map[string][]string, error) {
	if allFlag {
		for _, f := range filterFlags {
			if cmd.Flags().Changed(f) {
				return nil, fmt.Errorf("--all cannot be combined with --%s", f)
			}
		}
		return map[string][]string{}, nil
	}

	if err := common.CheckAvailabilityZones(azs); err != nil && !errors.Is(err, common.ErrNoAZSelected) {
		return nil, err
	}

	if _, err := common.ParseTags(tags); err != nil {
		return nil, err
	}

	return common.StructToFilters(filterStruct)
}

// runSearch is the common RunE body for ec2, eni, and ebs commands.
//
// It validates the sort field, builds filters, and executes the search.
func runSearch(
	cmd *cobra.Command,
	allLabel, sortLabel, noInstanceNameLabel string,
	filterFlags []string,
	azs, tags []string,
	filterStruct interface{},
) error {
	if err := search.CheckSortField(cmd.Name(), viper.GetString(sortLabel)); err != nil {
		return err
	}

	filters, err := buildFilters(
		cmd, viper.GetBool(allLabel), filterFlags,
		azs, tags, filterStruct,
	)
	if err != nil {
		return err
	}

	tagsKeys := viper.GetStringSlice(labelTagsKeys)

	return search.Execute(
		cmd.Name(),
		viper.GetStringSlice(labelProfiles),
		viper.GetStringSlice(labelRegions),
		filters,
		viper.GetString(sortLabel),
		viper.GetString(labelOutput),
		viper.GetBool(labelShowEmpty),
		viper.GetBool(labelShowTags) || len(tagsKeys) > 0,
		tagsKeys,
		noInstanceNameLabel != "" && viper.GetBool(noInstanceNameLabel),
	)
}
