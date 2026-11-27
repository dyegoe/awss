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

package cmd

import (
	searchDXGW "github.com/dyegoe/awss/search/dxgw"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const labelDXGatewaysSort = "dx.gateways.sort"

// dxGatewaysFilters represents the filters of the dx gateways command, keyed by the search/dxgw
// FilterKey constants.
type dxGatewaysFilters struct {
	IDs             []string `filter:"id"`
	Names           []string `filter:"name"`
	CIDRs           []string `filter:"cidr"`
	Match           []string `filter:"match"`
	AssociatedTypes []string `filter:"associated-type"`
	AssociatedIDs   []string `filter:"associated-id"`
	States          []string `filter:"state"`
}

var dxGatewaysF = dxGatewaysFilters{}

// dxGatewaysFilterFlags lists the dx gateways filter flag names: without any, the command lists
// every gateway.
var dxGatewaysFilterFlags = []string{
	flagIDs, flagNames, flagCIDRs, flagMatch, "associated-types", "associated-ids", flagStates,
}

// dxCmd groups the AWS Direct Connect searches.
var dxCmd = &cobra.Command{
	Use:   "dx",
	Short: "Search AWS Direct Connect: gateways, their associations and allowed prefixes.",
	Long: `
Search AWS Direct Connect.

Run them with a profile of the account that owns the Direct Connect gateways, usually the
network account. A Direct Connect gateway is global, so these commands search each profile once,
in the region of the profile, and ignore --regions.
`,
	Args: cobra.NoArgs,
}

// dxGatewaysCmd lists the Direct Connect gateways, their associations and allowed prefixes.
var dxGatewaysCmd = &cobra.Command{
	Use:         "gateways",
	Short:       "List the Direct Connect gateways: what they are associated with and the prefixes they advertise.",
	Annotations: map[string]string{annotationEngine: "dx-gateways"},
	Long: `
List the Direct Connect gateways, what each one is associated with (a transit gateway, a virtual
private gateway or a Cloud WAN core network), the prefixes it advertises to the data centres, and
the virtual interfaces (VIFs) attached to it.

One row per allowed prefix of each association. The allowed prefixes of an association to a
transit gateway or a virtual private gateway are what the data centres learn over Direct Connect.
An association to a Cloud WAN core network has none: Cloud WAN advertises the routes of the
segment the gateway is attached to (awss cloudwan routes, --target-types dx).

Every filter is matched by awss on the results.
  --cidrs            CIDRs or IP addresses to find in the allowed prefixes; a bare IP is its /32.
  --match            how an allowed prefix must relate to each CIDR:
                       any          (default) exact, subnet-of and supernet-of together
                       exact        the allowed prefix of exactly that CIDR
                       subnet-of    allowed prefixes inside the CIDR
                       supernet-of  allowed prefixes covering the CIDR: those that carry it
  --associated-types core-network, transit-gateway or virtual-private-gateway.

Is 10.20.1.5 advertised to the data centres, and by which gateway and association?
	awss dx gateways --profile network --cidrs 10.20.1.5 --match supernet-of

The gateways attached to Cloud WAN, and their VIFs:
	awss dx gateways --profile network --associated-types core-network
`,
	Args: cobra.NoArgs,
	RunE: dxGatewaysRunE,
}

// checked returns the filters searchDXGW.CheckFlags validates before any AWS call.
func (f *dxGatewaysFilters) checked() map[string][]string {
	return map[string][]string{
		searchDXGW.FilterKeyName: f.Names, searchDXGW.FilterKeyCIDR: f.CIDRs, searchDXGW.FilterKeyMatch: f.Match,
		searchDXGW.FilterKeyAssociatedType: f.AssociatedTypes, searchDXGW.FilterKeyState: f.States,
	}
}

func dxGatewaysRunE(cmd *cobra.Command, _ []string) error {
	if err := searchDXGW.CheckFlags(dxGatewaysF.checked()); err != nil {
		return err
	}
	viper.Set(labelRegions, []string{searchDXGW.Region})

	return runSearch(cmd, &cmdSpec{
		sortLabel: labelDXGatewaysSort, filterFlags: dxGatewaysFilterFlags, optionalFilters: true,
	}, nil, nil, dxGatewaysF)
}

// addDXCommand adds cmd to the dx group, and the group to the root. setup runs again in tests,
// so a command added by the previous run is removed first.
func addDXCommand(cmd *cobra.Command) {
	rootCmd.RemoveCommand(dxCmd)
	rootCmd.AddCommand(dxCmd)
	dxCmd.RemoveCommand(cmd)
	dxCmd.AddCommand(cmd)
}

func dxGatewaysInitFlags() {
	addDXCommand(dxGatewaysCmd)
	f, fl := &dxGatewaysF, dxGatewaysCmd.Flags()

	fl.StringSliceVarP(&f.IDs, flagIDs, "i", []string{},
		"Filter gateways by ID, exact. `01234567-89ab-cdef-0123-456789abcdef`")
	fl.StringSliceVarP(&f.Names, flagNames, "n", []string{},
		"Filter gateways by name (globs). `'dc-*'`")
	fl.StringSliceVarP(&f.CIDRs, flagCIDRs, "c", []string{},
		"CIDRs or IP addresses to find in the allowed prefixes; a bare IP is its /32. `10.20.0.0/16,10.30.1.5`")
	fl.StringSliceVarP(&f.Match, flagMatch, "m", []string{},
		listHelp("How an allowed prefix must relate to each CIDR (default any)", searchDXGW.MatchNames(), "supernet-of"))
	fl.StringSliceVar(&f.AssociatedTypes, "associated-types", []string{},
		listHelp("Keep the associations of these types", searchDXGW.AssociatedTypeNames(), "transit-gateway"))
	fl.StringSliceVar(&f.AssociatedIDs, "associated-ids", []string{},
		"Keep the associations of these transit gateways, virtual private gateways or core networks, exact. `tgw-0123`")
	fl.StringSliceVar(&f.States, flagStates, []string{},
		listHelp("Keep the associations in these states", searchDXGW.StateNames(), "associated"))
	fl.String(flagSort, "gateway", sortHelp("dx-gateways", "gateways", "gateway"))
}

func dxGatewaysInitViper() error {
	return bindFlags(dxGatewaysCmd, map[string]string{labelDXGatewaysSort: flagSort})
}
