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
	"strings"

	searchOrg "github.com/dyegoe/awss/search/org"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelOrgSort = "org.sort"

	flagStatuses = "statuses"
)

// orgFilters represents the filters for the org command. ListAccounts has no server-side filter,
// so search/org matches them client-side.
type orgFilters struct {
	Statuses []string `filter:"status"`
}

var orgF = orgFilters{}

// orgFilterFlags lists the org filter flag names.
var orgFilterFlags = []string{flagStatuses}

// orgCmd represents the org command.
var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "List the accounts of the AWS Organization.",
	Long: `
List the accounts of the AWS Organization: ID, name, email, status and joined date.

Run it with one profile allowed to call organizations:ListAccounts: the management account or a
delegated administrator. A profile without that permission gets an AccessDenied error, never an
empty list.

Organizations is a global service and one call lists the whole organization, so org takes
exactly one profile (--profiles, or the default resolution) and ignores --regions.

Filter by status with --statuses, for example:
	awss org --statuses active,pending-closure

Account tags need one API call per account, so they are fetched only with --show-tags or
--show-tags-keys.
`,
	Args: cobra.NoArgs,
	RunE: orgRunE,
}

func orgRunE(cmd *cobra.Command, _ []string) error {
	// Every profile would list the same organization: refuse several before any call.
	if profiles := viper.GetStringSlice(labelProfiles); len(profiles) != 1 {
		return fmt.Errorf("org lists one organization: pass one profile with --profiles, not %d (%s)",
			len(profiles), strings.Join(profiles, ","))
	}
	if err := searchOrg.CheckStatuses(orgF.Statuses); err != nil {
		return err
	}
	viper.Set(labelRegions, []string{searchOrg.Region})

	return runSearch(cmd, &cmdSpec{
		sortLabel: labelOrgSort, filterFlags: orgFilterFlags, optionalFilters: true,
	}, nil, nil, orgF)
}

func orgInitFlags() {
	rootCmd.AddCommand(orgCmd)

	orgCmd.Flags().StringSliceVarP(&orgF.Statuses, flagStatuses, "s", []string{},
		"Keep the accounts with these statuses: "+strings.Join(searchOrg.StatusNames(), ", ")+". `active,suspended`")
	orgCmd.Flags().String(flagSort, "name", sortHelp("org", "accounts", "name"))
}

func orgInitViper() error {
	return bindFlags(orgCmd, map[string]string{labelOrgSort: flagSort})
}
