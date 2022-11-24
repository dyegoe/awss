package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dyegoe/awss/search"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/ini.v1"
)

var config string

// awssCmd represents the base command when called without any subcommands
var awssCmd = &cobra.Command{
	Use:   "awss",
	Short: "AWSS is a CLI tool to make your life easier when searching AWS resources.",
	Long: `AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.
It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

This command uses the credentials stored in ~/.aws/credentials and ~/.aws/config files.

The 'default' profile is used if no profile is provided.
The provided profile must be present in ~/.aws/credentials and ~/.aws/config files.
If you would like to iterate over multiple profiles, you can pass them separated by comma. Example: --profile profile1,profile2.
You can also pass 'all' to iterate over all profiles.

The default region is 'us-east-1'.
If you would like to iterate over multiple regions, you can pass them separated by comma. Example: --region region1,region2.
You can also pass 'all' to iterate over all regions.

You can find the source code on GitHub:
https://github.com/dyegoe/awss`,
	// Remember to update this version when releasing a new version
	Version: "0.5.5",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := initConfig()
		if err != nil {
			return err
		}

		// Check if the provided output is valid
		err = search.ValidateOutput(viper.GetString("output"))
		if err != nil {
			return err
		}

		// Send current profiles to getProfiles which will check if they are valid or if 'all' was passed
		profiles, err := getProfiles(viper.GetStringSlice("profiles"))
		if err != nil {
			return err
		}
		viper.Set("profiles", profiles)

		// Send current regions to getRegions which will check if they are valid or if 'all' was passed
		regions, err := getRegions(viper.GetStringSlice("regions"))
		if err != nil {
			return err
		}
		viper.Set("regions", regions)

		return nil
	},
}

// init is called before the command is executed and is used to set flags
func init() {
	// Set flags for awssCmd
	awssCmd.PersistentFlags().StringVarP(&config, "config", "c", "", "config file (default is $HOME/.awss.yaml)")
	// Flags that can be configured in the config file
	awssCmd.PersistentFlags().StringSlice("profiles", []string{"default"}, "Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. `profile1,profile2`")
	awssCmd.PersistentFlags().StringSlice("regions", []string{"us-east-1"}, "Select a region to perform your API calls. You can pass multiple regions separated by comma. `region1,region2`")
	awssCmd.PersistentFlags().String("output", "table", "Select the output format. `table`, json or json-pretty")
	awssCmd.PersistentFlags().Bool("show-empty", false, "Show empty results. Default is false")
	// Add ec2Cmd to awssCmd
	awssCmd.AddCommand(ec2Cmd)
	// Bind flags to viper
	viper.BindPFlag("profiles", awssCmd.PersistentFlags().Lookup("profiles"))
	viper.BindPFlag("regions", awssCmd.PersistentFlags().Lookup("regions"))
	viper.BindPFlag("output", awssCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("show-empty", awssCmd.PersistentFlags().Lookup("show-empty"))
	// Set default values for configuration
	viper.SetDefault("table.style", "uc")
	viper.SetDefault("all-regions", []string{
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
	viper.SetDefault("separators.kv", ": ")
	viper.SetDefault("separators.list", "\n")
}

func initConfig() error {
	if config != "" {
		abs, err := filepath.Abs(config)
		if err != nil {
			return err
		}
		base := filepath.Base(abs)
		path := filepath.Dir(abs)
		viper.SetConfigName(strings.TrimSuffix(base, filepath.Ext(base)))
		viper.AddConfigPath(path)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("$HOME/.awss/")
	}
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %s", err)
		}
	}
	return nil
}

// getProfiles returns the profiles
func getProfiles(p []string) ([]string, error) {
	profiles, err := getProfilesFromConfig()
	if err != nil {
		return nil, err
	}
	if p[0] == "all" {
		return profiles, nil
	}
	for _, profile := range p {
		if !stringInSlice(profile, profiles) {
			return nil, fmt.Errorf("profile %s not found", profile)
		}
	}
	return p, nil
}

// getProfilesFromConfig returns the profiles from the config file
func getProfilesFromConfig() ([]string, error) {
	f, err := ini.Load(search.DefaultSharedConfigFilename())
	if err != nil {
		return nil, fmt.Errorf("fail to read file: %v", err)
	}
	arr := []string{}
	for _, v := range f.Sections() {
		if strings.HasPrefix(v.Name(), "profile ") {
			arr = append(arr, strings.TrimPrefix(v.Name(), "profile "))
		}
	}
	return arr, nil
}

// getRegions returns the regions
func getRegions(r []string) ([]string, error) {
	regions := viper.GetStringSlice("all-regions")
	if r[0] == "all" {
		return regions, nil
	}
	for _, region := range r {
		if !stringInSlice(region, regions) {
			return nil, fmt.Errorf("region %s not found", region)
		}
	}
	return r, nil
}

// stringInSlice returns true if the string is in the slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Execute calls *cobra.Command.Execute() to start the CLI
func Execute() error {
	return awssCmd.Execute()
}
