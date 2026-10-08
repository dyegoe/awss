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
	"io"
	"os"
	"path/filepath"
	"time"

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
	labelAllProfiles    = "all-profiles"
	labelTimeout        = "timeout"
	labelConcurrency    = "concurrency"
	labelAccounts       = "accounts"

	// defaultTimeout is generous so --profiles all over many regions is not cut short.
	defaultTimeout = 5 * time.Minute

	// flagIDs, flagTags, flagTagsKey, and flagAvailabilityZones name the flags
	// shared by the ec2, eni, and ebs commands' sort-field lists.
	flagIDs               = "ids"
	flagNames             = "names"
	flagAll               = "all"
	flagSort              = "sort"
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
	if err := setup(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// subcommand ties a search subcommand to the functions that register its flags and bind them to viper.
type subcommand struct {
	cmd       *cobra.Command
	initFlags func()
	initViper func() error
}

// subcommands lists the search subcommands, in the order they are registered.
var subcommands = []subcommand{
	{ec2Cmd, ec2InitFlags, ec2InitViper},
	{eniCmd, eniInitFlags, eniInitViper},
	{ebsCmd, ebsInitFlags, ebsInitViper},
	{vpcCmd, vpcInitFlags, vpcInitViper},
	{subnetCmd, subnetInitFlags, subnetInitViper},
	{s3Cmd, s3InitFlags, s3InitViper},
	{s3objCmd, s3objInitFlags, s3objInitViper},
}

// setup registers the global flags and every subcommand with its flags, then binds them to viper.
func setup() error {
	initFlags()
	for _, sub := range subcommands {
		sub.initFlags()
	}

	if err := initViper(); err != nil {
		return err
	}
	for _, sub := range subcommands {
		if err := sub.initViper(); err != nil {
			return err
		}
	}
	return nil
}

// persistentPreRun is executed before any command.
func persistentPreRun(cmd *cobra.Command, _ []string) error {
	cfg, err := cmd.Flags().GetString(labelConfig)
	if err != nil {
		return fmt.Errorf("reading the --%s flag: %w", labelConfig, err)
	}

	if err := initConfig(cfg); err != nil {
		return err
	}

	profiles, err := common.CheckProfiles(viper.GetStringSlice(labelProfiles), viper.GetStringSlice(labelAllProfiles))
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

	timeout, err := parseTimeout(viper.Get(labelTimeout))
	if err != nil {
		return err
	}
	viper.Set(labelTimeout, timeout)

	if c := viper.GetInt(labelConcurrency); c < 1 {
		return fmt.Errorf("invalid concurrency: %v. Use a number of searches of 1 or more",
			viper.Get(labelConcurrency))
	}

	return nil
}

// parseTimeout reads the --timeout flag or the timeout config key.
//
// The value must be a duration with a unit, such as 90s or 5m, or 0. A bare number is rejected:
// it would be read as nanoseconds and time out every search.
func parseTimeout(v interface{}) (time.Duration, error) {
	s := fmt.Sprint(v)
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid timeout: %s. Use a duration such as 90s or 5m, or 0 to disable it", s)
	}
	return d, nil
}

// initFlags initializes cobras flags.
func initFlags() {
	validOutputs, _ := common.ValidOutputs("")

	rootCmd.PersistentFlags().String(labelConfig, "",
		"config file path (default is $HOME/.awss/config.yaml)")
	rootCmd.PersistentFlags().StringSlice(labelProfiles, []string{},
		"Select the profile from the AWS config file (AWS_CONFIG_FILE, or ~/.aws/config). You can pass "+
			"multiple profiles separated by comma. e.g. `profile1,profile2`. `all` uses the all-profiles list of "+
			"the config file, or every profile in the AWS config file when that list is not set. "+
			"If not set, falls back to the AWS SDK's default credential "+
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
	rootCmd.PersistentFlags().Duration(labelTimeout, defaultTimeout,
		"Stop waiting for the searches after this `duration` (e.g. 90s, 5m). The profiles and regions "+
			"that did not finish are reported as timed out; the others are printed. 0 disables it.")
	rootCmd.PersistentFlags().Int(labelConcurrency, search.DefaultConcurrency,
		"How many profile and region searches run at once. The others wait for a free slot.")
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
	if err := viper.BindPFlag(labelTimeout, rootCmd.PersistentFlags().Lookup(labelTimeout)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelTimeout, err)
	}
	if err := viper.BindPFlag(labelConcurrency, rootCmd.PersistentFlags().Lookup(labelConcurrency)); err != nil {
		return fmt.Errorf("error binding flag %s: %w", labelConcurrency, err)
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
			return fmt.Errorf("finding the home directory for the config file: %w", err)
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
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", f, err)
	}
	// check if the path is a directory
	if info.IsDir() {
		return fmt.Errorf("config file is a directory: %s", f)
	}

	viper.SetConfigFile(f)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("reading config file %s: %w", f, err)
	}
	return nil
}

// bindFlags binds the flags of cmd to viper keys. keys maps the viper key to the flag name.
func bindFlags(cmd *cobra.Command, keys map[string]string) error {
	for key, flag := range keys {
		if err := viper.BindPFlag(key, cmd.Flags().Lookup(flag)); err != nil {
			return fmt.Errorf("error binding flag %s: %w", flag, err)
		}
	}
	return nil
}

// sortHelp builds the --sort help text of a command from its registered sort fields,
// so the help never drifts from the struct tags.
func sortHelp(cmd, what, def string) string {
	return fmt.Sprintf("Sort %s by %s. `%s`", what, common.StringSliceToString(search.SortFieldNames(cmd), ", "), def)
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

// cmdSpec describes how a subcommand maps its viper keys and flags onto a search.
type cmdSpec struct {
	// allLabel and sortLabel are the viper keys of the --all and --sort flags.
	allLabel, sortLabel string

	// noInstanceNameLabel is the viper key of --no-instance-name, or "" when the command has none.
	noInstanceNameLabel string

	// regexLabel is the viper key of --regex, or "" when the command has none.
	regexLabel string

	// maxKeysLabel is the viper key of --max-keys, or "" when the command has none.
	maxKeysLabel string

	// maxBucketsLabel is the viper key of --max-buckets, or "" when the command has none.
	maxBucketsLabel string

	// filterFlags lists the filter flag names that cannot be combined with --all.
	filterFlags []string

	// accountNames is true for the commands with an Owner column, which get the account names.
	accountNames bool
}

// boolLabel returns the viper bool at label, or false when label is empty.
func boolLabel(label string) bool {
	return label != "" && viper.GetBool(label)
}

// intLabel returns the viper int at label, or 0 when label is empty.
func intLabel(label string) int {
	if label == "" {
		return 0
	}
	return viper.GetInt(label)
}

// executeSearch runs the search. It is a variable so tests can replace the AWS calls.
var executeSearch = search.Execute

// runSearch is the common RunE body of every search subcommand.
//
// It validates the sort field, builds filters, and executes the search.
func runSearch(cmd *cobra.Command, spec *cmdSpec, azs, tags []string, filterStruct interface{}) error {
	if err := search.CheckSortField(cmd.Name(), viper.GetString(spec.sortLabel)); err != nil {
		return err
	}

	filters, err := buildFilters(
		cmd, viper.GetBool(spec.allLabel), spec.filterFlags,
		azs, tags, filterStruct,
	)
	if err != nil {
		return err
	}

	tagsKeys := viper.GetStringSlice(labelTagsKeys)

	var names map[string]string
	if spec.accountNames {
		names = accountNames(cmd.ErrOrStderr())
	}

	return executeSearch(
		cmd.Name(),
		viper.GetStringSlice(labelProfiles),
		viper.GetStringSlice(labelRegions),
		filters,
		&search.Options{
			SortField:      viper.GetString(spec.sortLabel),
			Output:         viper.GetString(labelOutput),
			ShowEmpty:      viper.GetBool(labelShowEmpty),
			ShowTags:       viper.GetBool(labelShowTags) || len(tagsKeys) > 0,
			TagsKeys:       tagsKeys,
			NoInstanceName: boolLabel(spec.noInstanceNameLabel),
			Regex:          boolLabel(spec.regexLabel),
			MaxKeys:        intLabel(spec.maxKeysLabel),
			MaxBuckets:     intLabel(spec.maxBucketsLabel),
			AccountNames:   names,
			Concurrency:    viper.GetInt(labelConcurrency),
			Timeout:        viper.GetDuration(labelTimeout),
		},
	)
}

// accountNames returns the account names for the Owner column, keyed by account ID, from the
// accounts map of the awss config file.
//
// Account names are optional, so nothing here fails the run: an unusable entry is one warning on
// w. It returns nil when no name was found.
func accountNames(w io.Writer) map[string]string {
	names, warnings := common.ConfiguredAccountNames(viper.Get(labelAccounts))
	for _, warning := range warnings {
		fmt.Fprintf(w, "awss: warning: %s\n", warning)
	}
	if len(names) == 0 {
		return nil
	}
	return names
}
