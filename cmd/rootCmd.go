package cmd

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
)

var output string
var profile, region []string
var showEmptyResults bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "awss",
	Short: "AWSS is a CLI tool to make your life easier when searching AWS resources.",
	Long: `AWSS (stands for AWS Search) is a CLI tool to make your life easier when searching AWS resources.
It is a wrapper written in Go using AWS SDK Go v2. The work is still in progress and will be updated regularly.

This command uses the credentials stored in ~/.aws/credentials and ~/.aws/config files.

The 'default' profile is used if no profile is provided.
The provided profile must be present in ~/.aws/credentials and ~/.aws/config files.
If you would like to iterate over multiple profiles, you can pass them separated by comma. Example: --profile profile1,profile2.
You can also pass 'all' to iterate over all profiles.

The default region is 'eu-central-1'.
If you would like to iterate over multiple regions, you can pass them separated by comma. Example: --region region1,region2.
You can also pass 'all' to iterate over all regions.

You can find the source code on GitHub:
https://github.com/dyegoe/awss`,
	// Remember to update this version when releasing a new version
	Version:   "0.4.0",
	ValidArgs: []string{"ec2"},
	Args:      cobra.ExactValidArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := checkOutput(); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := cmd.Help()
		return err
	},
}

// init is called before the command is executed and is used to set flags
func init() {
	// Set flags for rootCmd
	rootCmd.PersistentFlags().StringSliceVar(&profile, "profile", []string{"default"}, "Select the profile from ~/.aws/config. You can pass multiple profiles separated by comma. `profile1,profile2`")
	rootCmd.PersistentFlags().StringSliceVar(&region, "region", []string{"eu-central-1"}, "Select a region to perform your API calls. You can pass multiple regions separated by comma. `region1,region2`")
	rootCmd.PersistentFlags().StringVar(&output, "output", "table", "Select the output format. `table`, json or json-pretty")
	rootCmd.PersistentFlags().BoolVar(&showEmptyResults, "show-empty-results", false, "Show empty result. Default is false")

}

// Execute calls *cobra.Command.Execute() to start the CLI
func Execute() error {
	return rootCmd.Execute()
}

// checkOutput checks if the output is valid
func checkOutput() error {
	if output != "table" && output != "json" && output != "json-pretty" {
		return fmt.Errorf("invalid output format. Please use 'table', 'json' or 'json-pretty'")
	}
	return nil
}

// ipToString converts a slice of net.IP to a slice of string
func ipToString(ip []net.IP) []string {
	var ips []string
	for _, i := range ip {
		ips = append(ips, i.String())
	}
	return ips
}
