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

package cloudwan

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// BGPResults describes the BGP routes of the Cloud WAN segments found with one profile: the
// routes before the routing policies, with their BGP attributes.
type BGPResults struct {
	common.BaseResults

	// Data contains the BGP routes found: one row per route and next hop.
	Data []bgpRow `json:"data"`

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// AccountNames maps an account ID to its name, for the Owner column. It is built once per run
	// and shared by every search of the run, so it is only read.
	AccountNames map[string]string `json:"-"`
}

// bgpRow represents one BGP route of one segment at one edge location. bgpRow holds only strings,
// so it is comparable and rows are deduplicated as map keys.
type bgpRow struct {
	// Query is the searched CIDR the route matched; empty when no CIDR was searched.
	Query string `json:"query,omitempty" header:"Query" sort:"query"`

	// CoreNetworkID is the ID of the core network.
	CoreNetworkID string `json:"core_network_id,omitempty" sort:"core-network"`

	// Segment is the segment of the route.
	Segment string `json:"segment,omitempty" header:"Segment" sort:"segment"`

	// Edge is the edge location (region) of the route.
	Edge string `json:"edge,omitempty" header:"Edge" sort:"edge"`

	// Prefix is the IP prefix of the route.
	Prefix string `json:"prefix,omitempty" header:"Prefix" sort:"prefix"`

	// ASPath is the BGP AS path, its AS numbers separated by spaces.
	ASPath string `json:"as_path,omitempty" header:"AS Path" sort:"as-path"`

	// Communities are the BGP communities of the route, separated by commas.
	Communities string `json:"communities,omitempty" header:"Communities" sort:"communities"`

	// LocalPreference is the BGP local preference.
	LocalPreference string `json:"local_preference,omitempty" header:"Local Pref" sort:"local-preference"`

	// MED is the BGP Multi-Exit Discriminator.
	MED string `json:"med,omitempty" header:"MED" sort:"med"`

	// NextHopType is the type of the next hop: vpc, direct-connect-gateway, vpn, connect-peer...
	NextHopType string `json:"next_hop_type,omitempty" header:"Next Hop Type" sort:"next-hop-type"`

	// NextHopID is the ID of the next hop resource, such as a VPC or a Direct Connect gateway.
	NextHopID string `json:"next_hop_id,omitempty" header:"Next Hop ID" sort:"next-hop-id"`

	// NextHopIP is the IP address of the next hop, such as the BGP peer of a Connect peer.
	NextHopIP string `json:"next_hop_ip,omitempty" sort:"next-hop-ip"`

	// AttachmentID is the core network attachment of the next hop.
	AttachmentID string `json:"attachment_id,omitempty" sort:"attachment"`

	// AttachmentName is the Name tag of the attachment.
	AttachmentName string `json:"attachment_name,omitempty" header:"Attachment" sort:"attachment-name"`

	// NextHopSegment is the segment of the next hop, from the attachment when AWS does not return
	// it: another segment for a route shared between segments.
	NextHopSegment string `json:"next_hop_segment,omitempty" header:"Next Hop Segment" sort:"next-hop-segment"`

	// NextHopEdge is the edge location of the next hop, or the edges of its attachment.
	NextHopEdge string `json:"next_hop_edge,omitempty"`

	// OwnerID is the account that owns the attachment of the next hop; empty when the attachments
	// could not be listed.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// OwnerName is the name of the owner account, from AccountNames; empty when it is unknown.
	OwnerName string `json:"owner_name,omitempty" header:"Owner" sort:"owner-name"`
}

// NewBGP initiates and returns a new instance of Cloud WAN BGP results. region is kept for the
// output only: the search always calls HomeRegion.
func NewBGP(profile, region string, filters map[string][]string, sortField string) *BGPResults {
	return &BGPResults{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data:    []bgpRow{},
		Filters: filters,
	}
}

// bgpAPI is the part of the Network Manager client used by the BGP search.
type bgpAPI interface {
	networkmanager.ListCoreNetworksAPIClient
	networkmanager.ListCoreNetworkRoutingInformationAPIClient
	coreNetworkAPI
}

// newBGPClient builds the Network Manager client of a profile for the BGP search. It is a variable
// so tests can replace the AWS config with a fake client.
var newBGPClient = func(profile string) (bgpAPI, error) { return NewClient(profile) }

// deniedBGP is the hint of an AccessDenied on ListCoreNetworkRoutingInformation.
const deniedBGP = "run it with a profile of the account that owns the core network, allowed " +
	"networkmanager:ListCoreNetworkRoutingInformation"

// CheckBGPFlags validates the CIDRs, the match mode and the AS numbers before any AWS call.
func CheckBGPFlags(cidrs, match, asns []string) error {
	_, err := newBGPQuery(map[string][]string{FilterKeyCIDR: cidrs, FilterKeyMatch: match, FilterKeyASN: asns})
	return err
}

// newBGPQuery builds the query of the BGP search: newQuery, and AS numbers that are numbers.
func newBGPQuery(filters map[string][]string) (*query, error) {
	q, err := newQuery(filters)
	if err != nil {
		return nil, err
	}
	for _, a := range q.asns {
		if _, err := strconv.ParseUint(a, 10, 32); err != nil {
			return nil, fmt.Errorf("invalid AS number %q", a)
		}
	}
	return q, nil
}

// Search searches the BGP routes of every segment of every core network the profile sees. A
// missing permission is an error in the result set, never an empty list.
func (r *BGPResults) Search(ctx context.Context) {
	q, err := newBGPQuery(r.Filters)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}
	client, err := newBGPClient(r.Profile)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	r.collect(ctx, client, q)
}

// collect searches every core network of the query with client, then sorts the rows.
func (r *BGPResults) collect(ctx context.Context, client bgpAPI, q *query) {
	s := search[bgpRow]{
		tables: func(id string, c *types.CoreNetwork) []routeTable { return segmentTables(id, c, q) },
		read: func(ctx context.Context, cn *coreNetwork, c *routeCall) ([]bgpRow, error) {
			return bgpRoutes(ctx, client, cn, c, q)
		},
		denied: deniedBGP,
	}
	rows, msgs := s.run(ctx, client, q, r.AccountNames)
	r.Data, r.Errors = append(r.Data, rows...), append(r.Errors, msgs...)

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// segmentTables returns the segment route tables of the core network that q asks for: AWS keeps
// BGP routes per segment and edge location, not per network function group.
func segmentTables(coreNetworkID string, cn *types.CoreNetwork, q *query) []routeTable {
	var out []routeTable
	for _, t := range routeTables(coreNetworkID, cn, q) {
		if t.group == "" {
			out = append(out, t)
		}
	}
	return out
}

// bgpInput returns the ListCoreNetworkRoutingInformation input of the call. A longest call sends
// no match field and reads every route of the segment, as for GetNetworkRoutes (longestMatches).
func (c *routeCall) bgpInput() *networkmanager.ListCoreNetworkRoutingInformationInput {
	in := &networkmanager.ListCoreNetworkRoutingInformationInput{
		CoreNetworkId: aws.String(c.table.coreNetworkID),
		SegmentName:   aws.String(c.table.segment),
		EdgeLocation:  aws.String(c.table.edge),
		MaxResults:    aws.Int32(PageSize),
	}
	match := []string{c.cidr}
	switch c.mode {
	case MatchExact:
		in.ExactPrefixMatches = match
	case MatchSubnetOf:
		in.SubnetOfMatches = match
	case MatchSupernetOf:
		in.SupernetOfMatches = match
	}
	return in
}

// bgpRoutes reads the BGP routes of one call, following every page, and returns the rows that
// pass --next-hop-types, --communities and --asns.
func bgpRoutes(
	ctx context.Context, client networkmanager.ListCoreNetworkRoutingInformationAPIClient, cn *coreNetwork,
	c *routeCall, q *query,
) ([]bgpRow, error) {
	var found []types.CoreNetworkRoutingInformation
	paginator := networkmanager.NewListCoreNetworkRoutingInformationPaginator(client, c.bgpInput())
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing the core network routing information: %w", err)
		}
		found = append(found, page.CoreNetworkRoutingInformation...)
	}
	if c.mode == MatchLongest {
		found = longestMatches(found, c.cidr, func(r *types.CoreNetworkRoutingInformation) *string { return r.Prefix })
	}
	var rows []bgpRow
	for i := range found {
		row := parseBGPRoute(&found[i], cn, c)
		if q.wantTarget(row.NextHopType) && q.wantBGP(&found[i]) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// wantBGP reports whether a BGP route passes --communities and --asns: it carries one of the
// communities, and its AS path has one of the AS numbers. AWS does not document whether an AS path
// item is one AS number or a path, so an item is split on spaces.
func (q *query) wantBGP(route *types.CoreNetworkRoutingInformation) bool {
	if len(q.communities) > 0 && !slices.ContainsFunc(route.Communities, func(c string) bool {
		return slices.Contains(q.communities, c)
	}) {
		return false
	}
	return len(q.asns) == 0 || slices.ContainsFunc(route.AsPath, func(item string) bool {
		return slices.ContainsFunc(strings.Fields(item), func(a string) bool { return slices.Contains(q.asns, a) })
	})
}

// parseBGPRoute converts a BGP route into a row. The segment and edges of the next hop come from
// its attachment when AWS does not return them.
func parseBGPRoute(route *types.CoreNetworkRoutingInformation, cn *coreNetwork, c *routeCall) bgpRow {
	row := bgpRow{
		Query:           c.cidr,
		CoreNetworkID:   cn.id,
		Segment:         c.table.segment,
		Edge:            c.table.edge,
		Prefix:          aws.ToString(route.Prefix),
		ASPath:          strings.Join(route.AsPath, " "),
		Communities:     strings.Join(route.Communities, ","),
		LocalPreference: aws.ToString(route.LocalPreference),
		MED:             aws.ToString(route.Med),
	}
	h := route.NextHop
	if h == nil {
		return row
	}
	row.NextHopType = Kebab(aws.ToString(h.ResourceType))
	row.NextHopID = aws.ToString(h.ResourceId)
	row.NextHopIP = aws.ToString(h.IpAddress)
	row.AttachmentID = aws.ToString(h.CoreNetworkAttachmentId)
	att := cn.attachments[row.AttachmentID]
	row.AttachmentName, row.OwnerID, row.OwnerName = att.name, att.owner, cn.accountNames[att.owner]
	row.NextHopSegment = cmp.Or(aws.ToString(h.SegmentName), att.segment)
	row.NextHopEdge = cmp.Or(aws.ToString(h.EdgeLocation), att.edges)
	return row
}

// sortResults sorts the results by the given field.
func (r *BGPResults) sortResults(field string) error {
	sortFields, err := GetBGPSortFields(field)
	if err != nil {
		return err
	}
	common.SortByField(r.Data, sortFields[field])
	return nil
}

// Len returns the length of the results.
func (r *BGPResults) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the bgpRow fields.
func (r *BGPResults) GetHeaders() []interface{} { return common.Headers(bgpRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *BGPResults) GetRows() []interface{} { return common.Rows(r.Data) }

// GetBGPSortFields returns a map of the BGP sort fields and their corresponding struct field. It
// returns an error if the given field is not a valid sort field.
func GetBGPSortFields(f string) (map[string]string, error) {
	return common.SortFields(bgpRow{}, f)
}

// BGPSortFieldNames returns the valid BGP sort fields, sorted alphabetically.
func BGPSortFieldNames() []string {
	return common.SortFieldNames(bgpRow{})
}
