/*
Copyright © 2026 Dyego Alexandre Eugenio github@dyego.com.br

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
	"time"

	"github.com/dyegoe/awss/search"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	flagApply      = "apply"
	flagConfirmAll = "confirm-all"
)

// cleanupCmd groups the commands that delete resources. They only report a plan unless --apply
// is given.
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Delete unused resources. Preview by default; --apply deletes.",
	Long: `
Delete unused resources.

Unlike the search commands, the cleanup commands change your AWS accounts.
Every cleanup command only shows its plan unless --apply is given.
`,
	Args: cobra.NoArgs,
}

// defaultVpcCmd deletes the default VPCs.
var defaultVpcCmd = &cobra.Command{
	Use:   "default-vpc",
	Short: "Delete the default VPCs and their network plumbing, skipping the VPCs with workloads.",
	Long: `
Delete the default VPC of each profile and region, with the network plumbing inside it.

Without --apply it only shows the plan: nothing changes.

Deleted: VPC endpoints, NAT gateways (their Elastic IPs are released: permanent), peering
connections, the Transit Gateway attachment (the Transit Gateway stays), detached ENIs of the
account, security groups, subnets, route tables, network ACLs, internet gateways, egress-only
internet gateways, flow logs (the stored logs stay), then the VPC. VPN gateways are detached
only: the gateway and its VPN connections stay.

All or nothing: any other network interface in the VPC (EC2, RDS, Lambda, load balancers, EFS,
ECS, ElastiCache...) blocks it, and nothing in that VPC is touched. The Details column names the
blockers. A region without a default VPC shows nothing (--show-empty shows it).

Statuses: deletable, blocked, denied (the role may not delete), deleted, failed (run again to
resume).

--apply with --profiles all or --regions all needs --confirm-all too.
Deleting waits for NAT gateways and endpoints: raise --timeout for large runs, e.g. --timeout 30m.
`,
	Example: `  awss cleanup default-vpc --profiles sandbox --regions all
  awss cleanup default-vpc --profiles sandbox --regions eu-west-1 --apply
  awss cleanup default-vpc --profiles all --regions all --apply --confirm-all --timeout 30m`,
	Args: cobra.NoArgs,
	RunE: cleanupRunE,
}

// cleanupRunE is the RunE of every cleanup command: it checks the safety flags and runs the
// cleanup named after the command over every profile and region.
func cleanupRunE(cmd *cobra.Command, _ []string) error {
	apply, err := cmd.Flags().GetBool(flagApply)
	if err != nil {
		return fmt.Errorf("reading the --%s flag: %w", flagApply, err)
	}
	confirm, err := cmd.Flags().GetBool(flagConfirmAll)
	if err != nil {
		return fmt.Errorf("reading the --%s flag: %w", flagConfirmAll, err)
	}
	everywhere := viper.GetBool(labelProfilesAllAsked) || viper.GetBool(labelRegionsAllAsked)
	if err := checkApply(apply, confirm, everywhere); err != nil {
		return err
	}

	start := time.Now()
	summary, err := executeSearch(
		cmd.Name(),
		viper.GetStringSlice(labelProfiles),
		viper.GetStringSlice(labelRegions),
		map[string][]string{},
		&search.Options{
			Output:      viper.GetString(labelOutput),
			ShowEmpty:   viper.GetBool(labelShowEmpty),
			Concurrency: viper.GetInt(labelConcurrency),
			Timeout:     viper.GetDuration(labelTimeout),
			Apply:       apply,
		},
	)
	if err != nil {
		return err
	}
	finishRun(cmd, start, &summary)
	return nil
}

// checkApply refuses --apply over every profile or every region (everywhere) without
// --confirm-all, and --confirm-all where it means nothing.
func checkApply(apply, confirm, everywhere bool) error {
	switch {
	case confirm && !apply:
		return fmt.Errorf("--%s only goes with --%s", flagConfirmAll, flagApply)
	case confirm && !everywhere:
		return fmt.Errorf("--%s only goes with --profiles all or --regions all", flagConfirmAll)
	case apply && everywhere && !confirm:
		return fmt.Errorf("--%s with --profiles all or --regions all deletes in every profile or region: "+
			"add --%s to confirm", flagApply, flagConfirmAll)
	}
	return nil
}

// addCleanupCommand adds cmd to the cleanup group, and the group to the root, with the safety
// flags of every cleanup command.
//
// setup runs again in tests, so a command added by the previous run is removed first; removing
// one that is not there does nothing. The flags are not bound to viper on purpose: a config file
// must never turn on deleting.
func addCleanupCommand(cmd *cobra.Command) {
	rootCmd.RemoveCommand(cleanupCmd)
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.RemoveCommand(cmd)
	cleanupCmd.AddCommand(cmd)

	cmd.Flags().Bool(flagApply, false,
		"Delete what the plan lists. Without it, nothing changes.")
	cmd.Flags().Bool(flagConfirmAll, false,
		"Confirm --apply with --profiles all or --regions all, which deletes in every profile or region.")
}

// cleanupInitViper binds nothing: the flags of a cleanup command are read from the command only.
func cleanupInitViper() error { return nil }

// defaultVpcInitFlags registers the default-vpc command.
func defaultVpcInitFlags() { addCleanupCommand(defaultVpcCmd) }
