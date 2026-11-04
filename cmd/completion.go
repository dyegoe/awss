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
	"slices"
	"strings"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"
	searchOrg "github.com/dyegoe/awss/search/org"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// noArgsHint is the Active Help hint shown on <TAB> after a search command, which takes no
// arguments: Cobra lists flag names only once a "-" is typed.
const noArgsHint = "%s takes no arguments; type - and press TAB to see its flags"

// flagCompletions maps a flag name to the completion of its value.
type flagCompletions map[string]cobra.CompletionFunc

// registerCompletions adds the shell completion of every flag and the no-arguments completion
// of every search command:
//   - the flags of a known list complete its values. The values come from the lists awss
//     validates against, or from the AWS SDK enums, so completion cannot drift;
//   - --config completes YAML file names;
//   - every other flag that takes a value completes nothing, so the shell offers no file names.
//
// Cobra keeps the completions by flag, and setup creates new flags every time, so every setup
// registers them again.
func registerCompletions() error {
	if err := rootCmd.MarkPersistentFlagFilename(labelConfig, "yaml", "yml"); err != nil {
		return fmt.Errorf("registering the --%s completion: %w", labelConfig, err)
	}
	if err := register(rootCmd, globalCompletions()); err != nil {
		return err
	}
	if err := registerNoValues(rootCmd, rootCmd.PersistentFlags()); err != nil {
		return err
	}
	for _, sub := range subcommands {
		sub.cmd.ValidArgsFunction = noArgs
		if err := register(sub.cmd, commandCompletions(sub.cmd)); err != nil {
			return err
		}
		if err := registerNoValues(sub.cmd, sub.cmd.NonInheritedFlags()); err != nil {
			return err
		}
	}
	return nil
}

// register adds the completions of comps to the flags of cmd.
func register(cmd *cobra.Command, comps flagCompletions) error {
	for name, fn := range comps {
		if err := cmd.RegisterFlagCompletionFunc(name, fn); err != nil {
			return fmt.Errorf("registering the %s --%s completion: %w", cmd.Name(), name, err)
		}
	}
	return nil
}

// registerNoValues adds a completion with no values, and no file names, to every flag of flags
// that takes a value and has no completion yet. --config keeps its file name completion.
func registerNoValues(cmd *cobra.Command, flags *pflag.FlagSet) error {
	var err error
	flags.VisitAll(func(f *pflag.Flag) {
		if err != nil || f.Value.Type() == "bool" || f.Name == labelConfig {
			return
		}
		if _, ok := cmd.GetFlagCompletionFunc(f.Name); ok {
			return
		}
		if regErr := cmd.RegisterFlagCompletionFunc(f.Name, cobra.NoFileCompletions); regErr != nil {
			err = fmt.Errorf("registering the %s --%s completion: %w", cmd.Name(), f.Name, regErr)
		}
	})
	return err
}

// globalCompletions returns the completions of the global flags.
func globalCompletions() flagCompletions {
	return flagCompletions{
		labelOutput:     fixedValues(common.OutputNames),
		labelProfiles:   allOrList(awsProfiles),
		labelOrgProfile: fixedValues(awsProfiles),
		labelRegions:    regionValues,
	}
}

// commandCompletions returns the completions of the flags of a command: --sort when it has one, the
// availability zones when the command has them, and the flags of a fixed list.
func commandCompletions(cmd *cobra.Command) flagCompletions {
	name := cmd.Name()
	comps := flagCompletions{}
	if cmd.Flags().Lookup(flagSort) != nil {
		comps[flagSort] = fixedValues(func() []string { return search.SortFieldNames(name) })
	}
	if cmd.Flags().Lookup(flagAvailabilityZones) != nil {
		comps[flagAvailabilityZones] = listValues(common.AvailabilityZoneLetters)
	}
	for flag, values := range fixedLists()[name] {
		comps[flag] = listValues(values)
	}
	return comps
}

// fixedLists returns, by command and flag, the values of the flags that take a fixed list. Their
// help text lists the same values, with listHelp.
func fixedLists() map[string]map[string]func() []string {
	return map[string]map[string]func() []string{
		"ec2": {"instance-states": instanceStateNames},
		"ebs": {flagStatuses: volumeStateNames, "volume-types": volumeTypeNames, "encrypted": trueFalse},
		"vpc": {flagStates: vpcStateNames, "default": trueFalse},
		"subnet": {
			flagStates:            subnetStateNames,
			"default-for-az":      trueFalse,
			"public-ip-on-launch": trueFalse,
		},
		"org": {flagStatuses: searchOrg.StatusNames},
	}
}

// instanceStateNames returns the EC2 instance states of the AWS SDK.
func instanceStateNames() []string { return enumNames(ec2types.InstanceStateName("").Values()) }

// volumeStateNames returns the EBS volume states of the AWS SDK.
func volumeStateNames() []string { return enumNames(ec2types.VolumeState("").Values()) }

// volumeTypeNames returns the EBS volume types of the AWS SDK.
func volumeTypeNames() []string { return enumNames(ec2types.VolumeType("").Values()) }

// vpcStateNames returns the VPC states of the AWS SDK.
func vpcStateNames() []string { return enumNames(ec2types.VpcState("").Values()) }

// subnetStateNames returns the subnet states of the AWS SDK.
func subnetStateNames() []string { return enumNames(ec2types.SubnetState("").Values()) }

// trueFalse returns the values of a filter that is true or false.
func trueFalse() []string { return []string{"true", "false"} }

// enumNames returns the values of an AWS SDK enum as strings.
func enumNames[T ~string](values []T) []string {
	names := make([]string, 0, len(values))
	for _, v := range values {
		names = append(names, string(v))
	}
	return names
}

// listHelp builds the help text of a flag that takes a fixed list from the values it completes,
// so the help never drifts from them. example is the value placeholder shown in --help.
func listHelp(text string, values []string, example string) string {
	return fmt.Sprintf("%s: %s. %#q", text, strings.Join(values, ", "), example)
}

// awsProfiles returns the profiles of the AWS config file. Completion has no way to report an
// error, so an unreadable file offers no profiles.
func awsProfiles() []string {
	profiles, err := common.GetAwsProfiles()
	if err != nil {
		return nil
	}
	return profiles
}

// regionValues completes --regions with the all-regions list: the one of the config file, read
// as a run would (--config or ~/.awss/config.yaml), or the default list.
func regionValues(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if cfg, err := cmd.Flags().GetString(labelConfig); err == nil {
		// Completion has no way to report an error: a bad config file keeps the default list.
		_ = initConfig(cfg)
	}
	regions := func() []string { return viper.GetStringSlice(labelAllRegions) }
	return allOrList(regions)(cmd, args, toComplete)
}

// allOrList completes a comma list of names, as listValues, and offers "all" as the first item
// only: awss accepts "all" alone.
func allOrList(names func() []string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		withAll := names
		if !strings.Contains(toComplete, ",") {
			withAll = func() []string { return append([]string{valueAll}, names()...) }
		}
		return listValues(withAll)(cmd, args, toComplete)
	}
}

// noArgs completes the positional arguments of a command that takes none: no file names, and an
// Active Help hint that points to the flags. bash and zsh show it; Cobra drops it when
// AWSS_ACTIVE_HELP or COBRA_ACTIVE_HELP is 0.
func noArgs(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return cobra.AppendActiveHelp(nil, fmt.Sprintf(noArgsHint, cmd.Name())), cobra.ShellCompDirectiveNoFileComp
}

// fixedValues completes a flag that takes one value of names. The shell keeps the values that
// start with what is typed.
func fixedValues(names func() []string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return names(), cobra.ShellCompDirectiveNoFileComp
	}
}

// listValues completes a flag that takes a comma list of names. After "active," it offers the
// names not given yet, each prefixed with "active," so the shell completes the last item, and
// leaves no space so another comma can follow.
func listValues(names func() []string) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		prefix := toComplete[:strings.LastIndex(toComplete, ",")+1]
		given := strings.Split(prefix, ",")
		var out []string
		for _, n := range names() {
			if !slices.Contains(given, n) {
				out = append(out, prefix+n)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
}
