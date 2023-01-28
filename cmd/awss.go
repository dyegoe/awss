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
//   - The root command does not execute any further action but print Help().
//   - It contains the persistent flags and persistent pre-run function.
//   - The persistent flags are used by all the subcommands.
//   - The persistent pre-run function is executed before the subcommands and does sanity checks.
//   - The subcommands are in the subdirectories of the search engines and should be imported.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	enginesEc2Cmd "github.com/dyegoe/awss/engines/ec2/cmd"
	"github.com/dyegoe/awss/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Initialize setups the cli root command.
//
// It creates the root command, initializes the persistent flags and binds the viper flags to the cobra flags.
// By the end, it executes the root command.
func Initialize() error {
	log := logger.NewLogger(logger.DefaultOutput, map[string]string{
		"pkg":  "cmd",
		"cmd":  "awss",
		"func": "execute",
	})

	awssCmd := &cobra.Command{
		Use:           "awss",
		Short:         "Search resources in AWS.",
		Long:          `AWSS (AWS Search) is a command line tool to search resources in AWS.`,
		Version:       "0.8.0",
		SilenceErrors: true,
		RunE:          func(cmd *cobra.Command, args []string) error { return ErrNoSubcommand },
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := execute(cmd, log); err != nil {
				return fmt.Errorf("failed to execute root command: %w", err)
			}
			return nil
		},
	}

	// Initialize the persistent flags
	initPersistentFlags(awssCmd)
	if err := initViperBind(awssCmd); err != nil {
		return fmt.Errorf("failed to bind viper flags: %w", err)
	}

	// Add subcommands
	ec2Cmd, err := enginesEc2Cmd.Command()
	if err != nil {
		return fmt.Errorf("failed to create ec2 command: %w", err)
	}
	awssCmd.AddCommand(ec2Cmd)

	// Execute the root command
	if err := awssCmd.Execute(); err != nil {
		return fmt.Errorf("failed to execute root command: %w", err)
	}

	return nil
}

// ErrNoSubcommand is the error returned when no subcommand is provided.
var ErrNoSubcommand = errors.New("no subcommand provided")

// execute is the persistent pre-run function of the root command.
//
// It initializes the viper config, sets the log level and prints the flags.
func execute(c *cobra.Command, log *logger.Logger) error {
	log.AddFields(map[string]string{"func": "execute"})

	cfg, err := c.Flags().GetString("config")
	if err != nil {
		return fmt.Errorf("failed to get config flag: %w", err)
	}

	if err := initViperConfig(cfg); err != nil {
		return fmt.Errorf("failed to initialize viper config: %w", err)
	}

	logLevel := viper.GetString("log.level")
	if err := logger.SetLogLevel(logLevel); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	log.Debugf("Config -> %s", cfg)
	log.Debugf("Log Level -> %s", logLevel)
	log.Debugf("Profiles -> %v", viper.GetStringSlice("profiles"))
	log.Debugf("Regions -> %v", viper.GetStringSlice("regions"))
	log.Debugf("Output -> %s", viper.GetString("output"))
	log.Debugf("Show.Empty -> %v", viper.GetBool("show.empty"))
	log.Debugf("Show.Tags -> %v", viper.GetBool("show.tags"))
	for k, v := range viper.AllSettings() {
		log.Debugf("ViperConfig: %s -> %v", k, v)
	}

	return nil
}

// initPersistentFlags initializes the persistent flags.
//
//nolint:lll
func initPersistentFlags(c *cobra.Command) {
	validOutputs := "table, json, json-pretty"
	pflags := c.PersistentFlags()
	pflags.String("config", "", "config file or directory either absolute or relative path. (default is $HOME/.awss/config.yaml)")
	pflags.StringSlice("profiles", []string{"default"}, "Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. e.g. `profile1,profile2`")
	pflags.StringSlice("regions", []string{"us-east-1"}, "Select a region to perform your API calls. You can pass multiple regions separated by comma. e.g. `region1,region2`")
	pflags.String("output", "table", fmt.Sprintf("Select the output format. Valid outputs are: %s", validOutputs))
	pflags.Bool("show-empty", false, "Show empty resources. Default is false.")
	pflags.Bool("show-tags", false, "Show tags for resources. Default is false.")
	pflags.String("log-level", "info", "Set the log level. Valid levels are: debug, info, warn, error, fatal, panic")
}

// initViperBind binds the viper flags to the cobra flags.
func initViperBind(c *cobra.Command) error {
	viperBind := []struct{ viperFlag, cobraFlag string }{
		{"profiles", "profiles"},
		{"regions", "regions"},
		{"output", "output"},
		{"show.empty", "show-empty"},
		{"show.tags", "show-tags"},
		{"log.level", "log-level"},
	}

	pflags := c.PersistentFlags()
	for _, bind := range viperBind {
		if err := viper.BindPFlag(bind.viperFlag, pflags.Lookup(bind.cobraFlag)); err != nil {
			return fmt.Errorf("failed to bind viper flag %s to cobra flag %s: %w", bind.viperFlag, bind.cobraFlag, err)
		}
	}

	viper.SetDefault("all.regions", []string{
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

	viper.SetDefault("all.profiles", []string{"default"})

	return nil
}

var (
	filepathAbs = filepath.Abs
	osStat      = os.Stat
)

// initViperConfig initializes the viper config.
//
// The viper config is initialized in the following order:
// 1. If the config flag is set, it is used as the config file or directory.
// 2. If the config flag is not set, the default config file is used.
func initViperConfig(cfg string) error {
	configName := "config"
	configType := "yaml"
	currentPath := "."
	homePath := "$HOME/.awss/"

	if cfg != "" {
		abs, err := filepathAbs(cfg)
		if err != nil {
			return err
		}

		fileInfo, err := osStat(abs)
		if err == nil && fileInfo.IsDir() {
			viper.SetConfigName(configName)
			viper.SetConfigType(configType)
			viper.AddConfigPath(abs)
		}
		if err == nil && !fileInfo.IsDir() {
			viper.SetConfigFile(abs)
		}
		if err != nil {
			viper.SetConfigName(cfg)
			viper.AddConfigPath(currentPath)
			viper.AddConfigPath(homePath)
		}
	}
	if cfg == "" {
		viper.SetConfigName(configName)
		viper.SetConfigType(configType)
		viper.AddConfigPath(currentPath)
		viper.AddConfigPath(homePath)
	}

	err := viper.ReadInConfig()
	if err == nil {
		return nil
	}
	if errors.As(err, &viper.ConfigFileNotFoundError{}) && cfg == "" {
		return nil
	}
	return fmt.Errorf("failed to read config: %w", err)
}
