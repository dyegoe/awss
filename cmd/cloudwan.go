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
	searchCloudWAN "github.com/dyegoe/awss/search/cloudwan"
	searchCloudWANAtt "github.com/dyegoe/awss/search/cloudwanatt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	labelCloudWANRoutesSort      = "cloudwan.routes.sort"
	labelCloudWANAttachmentsSort = "cloudwan.attachments.sort"
	labelCloudWANBGPSort         = "cloudwan.bgp.sort"

	flagSegments       = "segments"
	flagEdges          = "edges"
	flagTypes          = "types"
	flagCoreNetworkIDs = "core-network-ids"
	flagMatch          = "match"

	// aliasHelp names the short next hop types of --target-types and --next-hop-types.
	aliasHelp = "connect, dx and tgw are short for connect-peer, direct-connect-gateway and transit-gateway-route-table"
)

// cloudwanRoutesFilters represents the filters of the cloudwan routes command, keyed by the
// search/cloudwan FilterKey constants.
type cloudwanRoutesFilters struct {
	CIDRs          []string `filter:"cidr"`
	Match          []string `filter:"match"`
	Segments       []string `filter:"segment"`
	FunctionGroups []string `filter:"function-group"`
	Edges          []string `filter:"edge"`
	Types          []string `filter:"type"`
	States         []string `filter:"state"`
	TargetTypes    []string `filter:"target-type"`
	CoreNetworkIDs []string `filter:"core-network-id"`
}

// cloudwanAttachmentsFilters represents the filters of the cloudwan attachments command, keyed by
// the search/cloudwanatt FilterKey constants.
type cloudwanAttachmentsFilters struct {
	IDs            []string `filter:"id"`
	Names          []string `filter:"name"`
	Types          []string `filter:"type"`
	Segments       []string `filter:"segment"`
	Edges          []string `filter:"edge"`
	States         []string `filter:"state"`
	CoreNetworkIDs []string `filter:"core-network-id"`
}

// cloudwanBGPFilters represents the filters of the cloudwan bgp command, keyed by the
// search/cloudwan FilterKey constants.
type cloudwanBGPFilters struct {
	CIDRs          []string `filter:"cidr"`
	Match          []string `filter:"match"`
	Segments       []string `filter:"segment"`
	Edges          []string `filter:"edge"`
	NextHopTypes   []string `filter:"target-type"`
	Communities    []string `filter:"community"`
	ASNs           []string `filter:"asn"`
	Installed      []string `filter:"installed"`
	CoreNetworkIDs []string `filter:"core-network-id"`
}

var (
	cloudwanRoutesF      = cloudwanRoutesFilters{}
	cloudwanAttachmentsF = cloudwanAttachmentsFilters{}
	cloudwanBGPF         = cloudwanBGPFilters{}
)

// cloudwanBGPFilterFlags lists the cloudwan bgp filter flag names: without any, the command lists
// every BGP route.
var cloudwanBGPFilterFlags = []string{
	flagCIDRs, flagMatch, flagSegments, flagEdges, "next-hop-types", "communities", "asns", "installed",
	flagCoreNetworkIDs,
}

// cloudwanRoutesFilterFlags lists the cloudwan routes filter flag names: without any, the command
// lists every route.
var cloudwanRoutesFilterFlags = []string{
	flagCIDRs, flagMatch, flagSegments, "function-groups", flagEdges, flagTypes, flagStates, "target-types",
	flagCoreNetworkIDs,
}

// cloudwanAttachmentsFilterFlags lists the cloudwan attachments filter flag names: without any,
// the command lists every attachment.
var cloudwanAttachmentsFilterFlags = []string{
	flagIDs, flagNames, flagTypes, flagSegments, flagEdges, flagStates, flagCoreNetworkIDs,
}

// cloudwanCmd groups the AWS Cloud WAN searches.
var cloudwanCmd = &cobra.Command{
	Use:   "cloudwan",
	Short: "Search AWS Cloud WAN core networks: routes and attachments.",
	Long: `
Search AWS Cloud WAN core networks.

Run them with a profile of the account that owns the core network, usually the network account:
an account the core network is shared with (AWS RAM) gets AccessDenied on the routes.

Network Manager is global and answers in us-west-2, so these commands search each profile once
and ignore --regions. Use --edges to pick edge locations.
`,
	Args: cobra.NoArgs,
}

// cloudwanRoutesCmd searches the routes of the core networks.
var cloudwanRoutesCmd = &cobra.Command{
	Use:         "routes",
	Short:       "Find a CIDR in the route tables of the Cloud WAN segments, and where its routes point.",
	Annotations: map[string]string{annotationEngine: "cloudwan-routes"},
	Long: `
Find which Cloud WAN segments and edge locations have a route to a CIDR, and where it points:
a VPC, a Direct Connect gateway, a VPN, a Connect peer, a Transit Gateway route table, or
nowhere (blackhole).

It reads the route table of every segment, and of every network function group (shown as
nfg:<name>), at every edge location. One row per route destination.

  --cidrs            CIDRs or IP addresses to find; a bare IP is its /32. Without --cidrs, every
                     route of the selected route tables is listed.
  --match            how a route must relate to each CIDR:
                       any          (default) exact, subnet-of and supernet-of together
                       exact        the route of exactly that CIDR
                       longest      the most specific route covering it: the one traffic takes
                       subnet-of    routes inside the CIDR
                       supernet-of  routes covering the CIDR
  --segments, --function-groups, --edges   pick route tables (globs). Only --segments skips the
                     function groups, only --function-groups skips the segments.
  --types, --states  route type and state, filtered by AWS.
  --target-types     next hop type, such as direct-connect-gateway (dx) or vpc.

A route to a Transit Gateway route table (tgw) ends there: find the VPC behind it with
awss tgw routes --route-table-ids <the Target ID> in the region of the edge.

Is the data centre prefix 10.50.0.0/16 learned over Direct Connect in every segment?
	awss cloudwan routes --profile network --cidrs 10.50.0.0/16 --target-types direct-connect-gateway

Which route does traffic to 10.20.1.5 take in the prod segment at eu-west-1?
	awss cloudwan routes --profile network --cidrs 10.20.1.5 --match longest -s prod -e eu-west-1
`,
	Args: cobra.NoArgs,
	RunE: cloudwanRoutesRunE,
}

// cloudwanBGPCmd searches the BGP routes of the segments, before the routing policies.
var cloudwanBGPCmd = &cobra.Command{
	Use:         "bgp",
	Short:       "Show the BGP routes of the Cloud WAN segments before the routing policies: AS path, communities...",
	Annotations: map[string]string{annotationEngine: "cloudwan-bgp"},
	Long: `
Show the BGP routes each Cloud WAN segment learned at each edge location, before the routing
policies of the core network, with their AS path, communities, local preference, MED and next hop.

Installed says whether each path made it into the route table (cloudwan routes):
  yes   the installed route of the prefix points to this path's attachment
  no    the installed route of the prefix points to another attachment, such as the backup DX
  none  no route of the prefix is installed: a routing policy dropped or summarised it
A prefix in neither command was never advertised. Network function groups have no BGP routes.

  --cidrs, --match   as in cloudwan routes.
  --next-hop-types   next hop type, such as direct-connect-gateway (dx) or vpc.
  --communities      keep the routes that carry any of these communities, exact.
  --asns             keep the routes whose AS path has any of these AS numbers.
  --installed        keep the routes with these Installed values.

What does the data centre advertise over Direct Connect to the prod segment?
	awss cloudwan bgp --profile network -s prod --next-hop-types dx

What does the data centre advertise that never reaches the prod route table?
	awss cloudwan bgp --profile network -s prod --installed none

Did the prefix 10.50.0.0/16 arrive, and with which AS path?
	awss cloudwan bgp --profile network --cidrs 10.50.0.0/16
`,
	Args: cobra.NoArgs,
	RunE: cloudwanBGPRunE,
}

// cloudwanAttachmentsCmd lists the attachments of the core networks.
var cloudwanAttachmentsCmd = &cobra.Command{
	Use:         "attachments",
	Short:       "List the attachments of the Cloud WAN core networks: VPCs, Direct Connect gateways, VPNs...",
	Annotations: map[string]string{annotationEngine: "cloudwan-attachments"},
	Long: `
List the attachments of the Cloud WAN core networks: which VPC, Direct Connect gateway, VPN,
Connect or Transit Gateway route table is attached to which segment, at which edge locations.

Every filter is matched by awss on the results. --names, --segments and --edges take globs; a
Direct Connect gateway spans several edge locations and matches when any of them does.
The Owner column names the accounts with the accounts map of the config file or --org-profile.

The Direct Connect gateways and the segments they are in:
	awss cloudwan attachments --profile network --types direct-connect-gateway
`,
	Args: cobra.NoArgs,
	RunE: cloudwanAttachmentsRunE,
}

func cloudwanRoutesRunE(cmd *cobra.Command, _ []string) error {
	f := &cloudwanRoutesF
	if err := searchCloudWAN.CheckFlags(f.CIDRs, f.Match, f.Types, f.States); err != nil {
		return err
	}
	viper.Set(labelRegions, []string{searchCloudWAN.Region})

	return runSearch(cmd, &cmdSpec{
		sortLabel: labelCloudWANRoutesSort, filterFlags: cloudwanRoutesFilterFlags, optionalFilters: true,
		accountNames: true,
	}, nil, nil, cloudwanRoutesF)
}

func cloudwanBGPRunE(cmd *cobra.Command, _ []string) error {
	f := &cloudwanBGPF
	if err := searchCloudWAN.CheckBGPFlags(f.CIDRs, f.Match, f.ASNs, f.Installed); err != nil {
		return err
	}
	viper.Set(labelRegions, []string{searchCloudWAN.Region})

	return runSearch(cmd, &cmdSpec{
		sortLabel: labelCloudWANBGPSort, filterFlags: cloudwanBGPFilterFlags, optionalFilters: true,
		accountNames: true,
	}, nil, nil, cloudwanBGPF)
}

func cloudwanAttachmentsRunE(cmd *cobra.Command, _ []string) error {
	f := &cloudwanAttachmentsF
	if err := searchCloudWANAtt.CheckFlags(map[string][]string{
		searchCloudWANAtt.FilterKeyType: f.Types, searchCloudWANAtt.FilterKeyState: f.States,
		searchCloudWANAtt.FilterKeyName: f.Names, searchCloudWANAtt.FilterKeySegment: f.Segments,
		searchCloudWANAtt.FilterKeyEdge: f.Edges,
	}); err != nil {
		return err
	}
	viper.Set(labelRegions, []string{searchCloudWAN.Region})

	return runSearch(cmd, &cmdSpec{
		sortLabel: labelCloudWANAttachmentsSort, filterFlags: cloudwanAttachmentsFilterFlags,
		optionalFilters: true, accountNames: true,
	}, nil, nil, cloudwanAttachmentsF)
}

// addCloudWANCommand adds cmd to the cloudwan group, and the group to the root.
//
// setup runs again in tests, so a command added by the previous run is removed first; removing
// one that is not there does nothing.
func addCloudWANCommand(cmd *cobra.Command) {
	rootCmd.RemoveCommand(cloudwanCmd)
	rootCmd.AddCommand(cloudwanCmd)
	cloudwanCmd.RemoveCommand(cmd)
	cloudwanCmd.AddCommand(cmd)
}

func cloudwanRoutesInitFlags() {
	addCloudWANCommand(cloudwanRoutesCmd)
	f, fl := &cloudwanRoutesF, cloudwanRoutesCmd.Flags()

	fl.StringSliceVarP(&f.CIDRs, flagCIDRs, "c", []string{},
		"CIDRs or IP addresses to find in the route tables; a bare IP is its /32. `10.20.0.0/16,10.30.1.5`")
	fl.StringSliceVarP(&f.Match, flagMatch, "m", []string{},
		listHelp("How a route must relate to each CIDR (default any)", searchCloudWAN.MatchNames(), "longest"))
	fl.StringSliceVarP(&f.Segments, flagSegments, "s", []string{},
		"Search the route tables of these segments only (globs). `'prod,shared*'`")
	fl.StringSliceVar(&f.FunctionGroups, "function-groups", []string{},
		"Search the route tables of these network function groups only (globs). `inspection`")
	fl.StringSliceVarP(&f.Edges, flagEdges, "e", []string{},
		"Search the route tables of these edge locations only (globs). `'eu-west-1,us-*'`")
	fl.StringSliceVarP(&f.Types, flagTypes, "T", []string{},
		listHelp("Keep the routes of these types", searchCloudWAN.TypeNames(), "propagated"))
	fl.StringSliceVar(&f.States, flagStates, []string{},
		listHelp("Keep the routes in these states", searchCloudWAN.StateNames(), "blackhole"))
	fl.StringSliceVar(&f.TargetTypes, "target-types", []string{},
		listHelp("Keep the route destinations of these next hop types ("+aliasHelp+")", cloudwanTargetTypes(), "dx"))
	fl.StringSliceVar(&f.CoreNetworkIDs, flagCoreNetworkIDs, []string{},
		"Search these core networks only. `core-network-0123456789abcdef0`")
	fl.String(flagSort, "segment", sortHelp("cloudwan-routes", "routes", "segment"))
}

func cloudwanRoutesInitViper() error {
	return bindFlags(cloudwanRoutesCmd, map[string]string{labelCloudWANRoutesSort: flagSort})
}

func cloudwanAttachmentsInitFlags() {
	addCloudWANCommand(cloudwanAttachmentsCmd)
	f, fl := &cloudwanAttachmentsF, cloudwanAttachmentsCmd.Flags()

	fl.StringSliceVarP(&f.IDs, flagIDs, "i", []string{},
		"Filter attachments by ID, exact. `attachment-0123456789abcdef0`")
	fl.StringSliceVarP(&f.Names, flagNames, "n", []string{},
		"Filter attachments by Name tag (globs). `'dc-*'`")
	fl.StringSliceVarP(&f.Types, flagTypes, "T", []string{},
		listHelp("Filter attachments by type", searchCloudWANAtt.TypeNames(), "direct-connect-gateway"))
	fl.StringSliceVarP(&f.Segments, flagSegments, "s", []string{},
		"Filter attachments by segment, or nfg:<name> for a network function group (globs). `prod`")
	fl.StringSliceVarP(&f.Edges, flagEdges, "e", []string{},
		"Filter attachments by edge location (globs). `eu-west-1`")
	fl.StringSliceVar(&f.States, flagStates, []string{},
		listHelp("Filter attachments by state", searchCloudWANAtt.StateNames(), "available"))
	fl.StringSliceVar(&f.CoreNetworkIDs, flagCoreNetworkIDs, []string{},
		"List the attachments of these core networks only. `core-network-0123456789abcdef0`")
	fl.String(flagSort, "segment", sortHelp("cloudwan-attachments", "attachments", "segment"))
}

func cloudwanAttachmentsInitViper() error {
	return bindFlags(cloudwanAttachmentsCmd, map[string]string{labelCloudWANAttachmentsSort: flagSort})
}

func cloudwanBGPInitFlags() {
	addCloudWANCommand(cloudwanBGPCmd)
	f, fl := &cloudwanBGPF, cloudwanBGPCmd.Flags()

	fl.StringSliceVarP(&f.CIDRs, flagCIDRs, "c", []string{},
		"CIDRs or IP addresses to find in the BGP routes; a bare IP is its /32. `10.20.0.0/16,10.30.1.5`")
	fl.StringSliceVarP(&f.Match, flagMatch, "m", []string{},
		listHelp("How a route must relate to each CIDR (default any)", searchCloudWAN.MatchNames(), "longest"))
	fl.StringSliceVarP(&f.Segments, flagSegments, "s", []string{},
		"Search the BGP routes of these segments only (globs). `'prod,shared*'`")
	fl.StringSliceVarP(&f.Edges, flagEdges, "e", []string{},
		"Search the BGP routes of these edge locations only (globs). `'eu-west-1,us-*'`")
	fl.StringSliceVar(&f.NextHopTypes, "next-hop-types", []string{},
		listHelp("Keep the routes of these next hop types ("+aliasHelp+")", cloudwanTargetTypes(), "dx"))
	fl.StringSliceVar(&f.Installed, "installed", []string{},
		listHelp("Keep the routes with these Installed values", searchCloudWAN.InstalledNames(), "none"))
	fl.StringSliceVar(&f.Communities, "communities", []string{},
		"Keep the routes that carry any of these BGP communities, exact. `65000:100`")
	fl.StringSliceVar(&f.ASNs, "asns", []string{},
		"Keep the routes whose AS path has any of these AS numbers. `65010`")
	fl.StringSliceVar(&f.CoreNetworkIDs, flagCoreNetworkIDs, []string{},
		"Search these core networks only. `core-network-0123456789abcdef0`")
	fl.String(flagSort, "segment", sortHelp("cloudwan-bgp", "BGP routes", "segment"))
}

func cloudwanBGPInitViper() error {
	return bindFlags(cloudwanBGPCmd, map[string]string{labelCloudWANBGPSort: flagSort})
}
