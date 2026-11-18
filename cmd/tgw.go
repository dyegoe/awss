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
	searchTGW "github.com/dyegoe/awss/search/tgw"

	"github.com/spf13/cobra"
)

const (
	labelTGWRoutesSort      = "tgw.routes.sort"
	labelTGWAttachmentsSort = "tgw.attachments.sort"
)

// tgwRoutesFilters represents the filters of the tgw routes command, keyed by the search/tgw
// FilterKey constants.
type tgwRoutesFilters struct {
	CIDRs             []string `filter:"cidr"`
	Match             []string `filter:"match"`
	RouteTableIDs     []string `filter:"route-table-id"`
	RouteTableNames   []string `filter:"route-table-name"`
	TransitGatewayIDs []string `filter:"transit-gateway-id"`
	Types             []string `filter:"type"`
	States            []string `filter:"state"`
	AttachmentTypes   []string `filter:"attachment-type"`
	AttachmentIDs     []string `filter:"attachment-id"`
	ResourceIDs       []string `filter:"resource-id"`
}

var tgwRoutesF = tgwRoutesFilters{}

// tgwRoutesFilterFlags lists the tgw routes filter flag names: without any, the command lists
// every route.
var tgwRoutesFilterFlags = []string{
	flagCIDRs, flagMatch, "route-table-ids", "route-table-names", "transit-gateway-ids", flagTypes, flagStates,
	"attachment-types", "attachment-ids", "resource-ids",
}

// tgwAttachmentsFilters represents the filters of the tgw attachments command, keyed by the
// search/tgw FilterKey constants.
type tgwAttachmentsFilters struct {
	IDs                     []string `filter:"attachment-id"`
	Names                   []string `filter:"attachment-name"`
	Types                   []string `filter:"attachment-type"`
	ResourceIDs             []string `filter:"resource-id"`
	States                  []string `filter:"attachment-state"`
	TransitGatewayIDs       []string `filter:"transit-gateway-id"`
	AssociatedRouteTableIDs []string `filter:"associated-route-table-id"`
}

var tgwAttachmentsF = tgwAttachmentsFilters{}

// tgwAttachmentsFilterFlags lists the tgw attachments filter flag names: without any, the command
// lists every attachment.
var tgwAttachmentsFilterFlags = []string{
	flagIDs, flagNames, flagTypes, "resource-ids", flagStates, "transit-gateway-ids", "associated-route-table-ids",
}

// tgwCmd groups the AWS Transit Gateway searches.
var tgwCmd = &cobra.Command{
	Use:   "tgw",
	Short: "Search AWS Transit Gateways: the routes of their route tables, and their attachments.",
	Long: `
Search AWS Transit Gateways.

Run them with a profile of the account that owns the transit gateways, usually the network
account: an account a transit gateway is shared with (AWS RAM) does not see its route tables.
A transit gateway is regional, so these commands search every profile and region, as ec2 does.
`,
	Args: cobra.NoArgs,
}

// tgwRoutesCmd searches the routes of the Transit Gateway route tables.
var tgwRoutesCmd = &cobra.Command{
	Use:         "routes",
	Short:       "Find a CIDR in the Transit Gateway route tables, and which attachment it points to.",
	Annotations: map[string]string{annotationEngine: "tgw-routes"},
	Long: `
Find which Transit Gateway route tables have a route to a CIDR, and which attachment it points
to: a VPC, a VPN, a Direct Connect gateway, a peering (such as Cloud WAN) or a Connect
attachment, or nowhere (blackhole).

One row per attachment of each route: a route with several attachments (ECMP) is several rows.
Associated RT is the route table the attachment is associated with: the table that routes the
traffic coming back from it, where to look for the return route.

  --cidrs            CIDRs or IP addresses to find; a bare IP is its /32. Without --cidrs, every
                     route of the selected route tables is listed.
  --match            how a route must relate to each CIDR:
                       any          (default) exact, subnet-of and supernet-of together
                       exact        the route of exactly that CIDR
                       longest      the most specific route covering it: the one traffic takes
                       subnet-of    routes inside the CIDR
                       supernet-of  routes covering the CIDR
  --transit-gateway-ids                      pick route tables, filtered by AWS.
  --route-table-ids, --route-table-names     pick route tables by ID, or by Name tag (globs).
  --types, --states, --attachment-types, --attachment-ids, --resource-ids   filtered by AWS.

Cloud WAN routes a CIDR to a Transit Gateway route table (awss cloudwan routes, target type
transit-gateway-route-table): which VPC holds it behind the transit gateway?
	awss tgw routes --profile network --regions eu-central-1 --route-table-ids tgw-rtb-0123 \
		--cidrs 10.20.1.5 --match longest

Is there a return route? Search the source and the destination at once: the route traffic takes
to each, in every route table (add --route-table-ids for the source's table and the Associated RT
of the destination's attachment). Behind a firewall attachment, the Associated RT is the inspection
table: follow each hop.
	awss tgw routes --profile network --regions eu-central-1 --cidrs 10.60.3.7,10.20.1.5 --match longest

The routes of every route table to one VPC:
	awss tgw routes --profile network --resource-ids vpc-0123
`,
	Args: cobra.NoArgs,
	RunE: tgwRoutesRunE,
}

// checked returns the filters searchTGW.CheckFlags validates before any AWS call.
func (f *tgwRoutesFilters) checked() map[string][]string {
	return map[string][]string{
		searchTGW.FilterKeyCIDR: f.CIDRs, searchTGW.FilterKeyMatch: f.Match,
		searchTGW.FilterKeyRouteTableName: f.RouteTableNames, searchTGW.FilterKeyType: f.Types,
		searchTGW.FilterKeyState: f.States, searchTGW.FilterKeyAttachmentType: f.AttachmentTypes,
	}
}

// tgwAttachmentsCmd lists the Transit Gateway attachments.
var tgwAttachmentsCmd = &cobra.Command{
	Use:         "attachments",
	Short:       "List the Transit Gateway attachments, and the route table each one is associated with.",
	Annotations: map[string]string{annotationEngine: "tgw-attachments"},
	Long: `
List the Transit Gateway attachments: which VPC, VPN, Direct Connect gateway, peering (such as
Cloud WAN) or Connect attachment is attached to which transit gateway, and the route table it is
associated with (Associated RT): the table that routes the traffic coming from it.

Associated RT is shown in any association state; the JSON association_state says whether it is
associated, or associating / disassociating while it changes.

  --ids, --types, --resource-ids, --transit-gateway-ids, --associated-route-table-ids
                     filtered by AWS. --types takes dx for direct-connect-gateway.
  --names, --states  matched by awss; --names takes globs.

The attachments associated with one route table:
	awss tgw attachments --profile network --regions eu-central-1 --associated-route-table-ids tgw-rtb-0123

The attachment of a VPC, and its route table:
	awss tgw attachments --profile network --resource-ids vpc-0123
`,
	Args: cobra.NoArgs,
	RunE: tgwAttachmentsRunE,
}

func tgwAttachmentsRunE(cmd *cobra.Command, _ []string) error {
	f := &tgwAttachmentsF
	if err := searchTGW.CheckAttachmentFlags(map[string][]string{
		searchTGW.FilterKeyAttachmentName: f.Names, searchTGW.FilterKeyAttachmentType: f.Types,
		searchTGW.FilterKeyAttachmentState: f.States,
	}); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		sortLabel: labelTGWAttachmentsSort, filterFlags: tgwAttachmentsFilterFlags, optionalFilters: true,
		accountNames: true,
	}, nil, nil, tgwAttachmentsF)
}

func tgwRoutesRunE(cmd *cobra.Command, _ []string) error {
	if err := searchTGW.CheckFlags(tgwRoutesF.checked()); err != nil {
		return err
	}
	return runSearch(cmd, &cmdSpec{
		sortLabel: labelTGWRoutesSort, filterFlags: tgwRoutesFilterFlags, optionalFilters: true, accountNames: true,
	}, nil, nil, tgwRoutesF)
}

// addTGWCommand adds cmd to the tgw group, and the group to the root. setup runs again in tests,
// so a command added by the previous run is removed first.
func addTGWCommand(cmd *cobra.Command) {
	rootCmd.RemoveCommand(tgwCmd)
	rootCmd.AddCommand(tgwCmd)
	tgwCmd.RemoveCommand(cmd)
	tgwCmd.AddCommand(cmd)
}

func tgwRoutesInitFlags() {
	addTGWCommand(tgwRoutesCmd)
	f, fl := &tgwRoutesF, tgwRoutesCmd.Flags()

	fl.StringSliceVarP(&f.CIDRs, flagCIDRs, "c", []string{},
		"CIDRs or IP addresses to find; a bare IP is its /32. `10.20.0.0/16,10.30.1.5`")
	fl.StringSliceVarP(&f.Match, flagMatch, "m", []string{},
		listHelp("How a route must relate to each CIDR (default any)", searchTGW.MatchNames(), "longest"))
	fl.StringSliceVar(&f.RouteTableIDs, "route-table-ids", []string{},
		"Search these route tables only, exact. `tgw-rtb-0123`")
	fl.StringSliceVar(&f.RouteTableNames, "route-table-names", []string{},
		"Search the route tables with these Name tags only (globs). `'prod-*'`")
	fl.StringSliceVar(&f.TransitGatewayIDs, "transit-gateway-ids", []string{},
		"Search the route tables of these transit gateways only, exact. `tgw-0123`")
	fl.StringSliceVarP(&f.Types, flagTypes, "T", []string{},
		listHelp("Keep the routes of these types", searchTGW.TypeNames(), "static"))
	fl.StringSliceVar(&f.States, flagStates, []string{},
		listHelp("Keep the routes in these states", searchTGW.StateNames(), "blackhole"))
	fl.StringSliceVar(&f.AttachmentTypes, "attachment-types", []string{},
		listHelp("Keep the routes to attachments of these types (dx is short for direct-connect-gateway)",
			tgwAttachmentTypes(), "vpc"))
	fl.StringSliceVar(&f.AttachmentIDs, "attachment-ids", []string{},
		"Keep the routes to these attachments, exact. `tgw-attach-0123`")
	fl.StringSliceVar(&f.ResourceIDs, "resource-ids", []string{},
		"Keep the routes to the attachments of these resources, such as VPCs, exact. `vpc-0123`")
	fl.String(flagSort, "route-table", sortHelp("tgw-routes", "routes", "route-table"))
}

// tgwAttachmentTypes returns the attachment types of --attachment-types and their short names.
func tgwAttachmentTypes() []string {
	return append(searchTGW.AttachmentTypeNames(), searchTGW.AttachmentAliasNames()...)
}

func tgwRoutesInitViper() error {
	return bindFlags(tgwRoutesCmd, map[string]string{labelTGWRoutesSort: flagSort})
}

func tgwAttachmentsInitFlags() {
	addTGWCommand(tgwAttachmentsCmd)
	f, fl := &tgwAttachmentsF, tgwAttachmentsCmd.Flags()

	fl.StringSliceVarP(&f.IDs, flagIDs, "i", []string{},
		"Filter attachments by ID, exact. `tgw-attach-0123`")
	fl.StringSliceVarP(&f.Names, flagNames, "n", []string{},
		"Filter attachments by Name tag (globs). `'prod-*'`")
	fl.StringSliceVarP(&f.Types, flagTypes, "T", []string{},
		listHelp("Filter attachments by resource type (dx is short for direct-connect-gateway)",
			tgwAttachmentTypes(), "vpc"))
	fl.StringSliceVar(&f.ResourceIDs, "resource-ids", []string{},
		"Filter attachments by resource, such as a VPC ID, exact. `vpc-0123`")
	fl.StringSliceVar(&f.States, flagStates, []string{},
		listHelp("Filter attachments by state", searchTGW.AttachmentStateNames(), "available"))
	fl.StringSliceVar(&f.TransitGatewayIDs, "transit-gateway-ids", []string{},
		"List the attachments of these transit gateways only, exact. `tgw-0123`")
	fl.StringSliceVar(&f.AssociatedRouteTableIDs, "associated-route-table-ids", []string{},
		"Filter attachments by the route table they are associated with, exact. `tgw-rtb-0123`")
	fl.String(flagSort, "name", sortHelp("tgw-attachments", "attachments", "name"))
}

func tgwAttachmentsInitViper() error {
	return bindFlags(tgwAttachmentsCmd, map[string]string{labelTGWAttachmentsSort: flagSort})
}
