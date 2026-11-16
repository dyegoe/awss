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
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// targetBlackhole is the target type of a blackhole route, which has no destination.
const targetBlackhole = "blackhole"

// routeTable is one route table of a core network: of a segment, or of a network function group,
// at one edge location.
type routeTable struct {
	coreNetworkID string
	segment       string
	group         string
	edge          string
}

// label returns the Segment column of the route table: the segment, or nfg:<name>.
func (t *routeTable) label() string {
	if t.group != "" {
		return FunctionGroupPrefix + t.group
	}
	return t.segment
}

// identifier returns the RouteTableIdentifier of GetNetworkRoutes for the route table.
func (t *routeTable) identifier() *types.RouteTableIdentifier {
	if t.group != "" {
		return &types.RouteTableIdentifier{CoreNetworkNetworkFunctionGroup: &types.CoreNetworkNetworkFunctionGroupIdentifier{
			CoreNetworkId: aws.String(t.coreNetworkID), EdgeLocation: aws.String(t.edge),
			NetworkFunctionGroupName: aws.String(t.group),
		}}
	}
	return &types.RouteTableIdentifier{CoreNetworkSegmentEdge: &types.CoreNetworkSegmentEdgeIdentifier{
		CoreNetworkId: aws.String(t.coreNetworkID), EdgeLocation: aws.String(t.edge),
		SegmentName: aws.String(t.segment),
	}}
}

// routeTables returns the route tables of the core network that q asks for.
func routeTables(coreNetworkID string, cn *types.CoreNetwork, q *query) []routeTable {
	var tables []routeTable
	if q.wantSegments() {
		for _, s := range cn.Segments {
			name := aws.ToString(s.Name)
			if !q.segments.Match(name) {
				continue
			}
			tables = append(tables, tablesAt(q, s.EdgeLocations, routeTable{coreNetworkID: coreNetworkID, segment: name})...)
		}
	}
	if q.wantGroups() {
		for _, g := range cn.NetworkFunctionGroups {
			name := aws.ToString(g.Name)
			if !q.groups.Match(name) {
				continue
			}
			tables = append(tables, tablesAt(q, g.EdgeLocations, routeTable{coreNetworkID: coreNetworkID, group: name})...)
		}
	}
	return tables
}

// tablesAt returns base at every edge location that passes --edges.
func tablesAt(q *query, edges []string, base routeTable) []routeTable {
	var tables []routeTable
	for _, edge := range edges {
		if q.edges.Match(edge) {
			t := base
			t.edge = edge
			tables = append(tables, t)
		}
	}
	return tables
}

// routeCall is one GetNetworkRoutes search: one route table, and at most one CIDR with one match
// mode, since AWS does not document how several match fields of one call combine.
type routeCall struct {
	table routeTable
	cidr  string
	mode  string
}

// calls returns the route calls of the tables: every searched CIDR in every match mode, or one
// call per table, for every route, when no CIDR is searched.
func calls(tables []routeTable, q *query) []routeCall {
	var out []routeCall
	for _, t := range tables {
		if len(q.cidrs) == 0 {
			out = append(out, routeCall{table: t})
			continue
		}
		for _, c := range q.cidrs {
			for _, m := range q.modes {
				out = append(out, routeCall{table: t, cidr: c, mode: m})
			}
		}
	}
	return out
}

// input returns the GetNetworkRoutes input of the call.
//
// A longest call sends no match field and reads the whole route table: AWS answers
// LongestPrefixMatches with "ValidationException: Invalid input. Filter value is invalid.", so
// routes picks the longest match itself.
func (c *routeCall) input(globalNetworkID string, q *query) *networkmanager.GetNetworkRoutesInput {
	in := &networkmanager.GetNetworkRoutesInput{
		GlobalNetworkId:      aws.String(globalNetworkID),
		RouteTableIdentifier: c.table.identifier(),
		MaxResults:           aws.Int32(PageSize),
		Types:                q.types,
		States:               q.states,
	}
	match := []string{c.cidr}
	switch c.mode {
	case MatchExact:
		in.ExactCidrMatches = match
	case MatchSubnetOf:
		in.SubnetOfMatches = match
	case MatchSupernetOf:
		in.SupernetOfMatches = match
	}
	return in
}

// routes reads the routes of one call, following every page, and returns its rows: one per route
// destination that passes --target-types.
func routes(
	ctx context.Context, client networkmanager.GetNetworkRoutesAPIClient, cn *coreNetwork, c *routeCall, q *query,
) ([]dataRow, error) {
	var found []types.NetworkRoute
	paginator := networkmanager.NewGetNetworkRoutesPaginator(client, c.input(cn.globalNetworkID, q))
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting the network routes: %w", err)
		}
		found = append(found, page.NetworkRoutes...)
	}
	if c.mode == MatchLongest {
		found = longestMatches(found, c.cidr, func(r *types.NetworkRoute) *string { return r.DestinationCidrBlock })
	}
	var rows []dataRow
	for i := range found {
		rows = appendTargets(rows, parseRoute(&found[i], cn, c), q)
	}
	return rows, nil
}

// appendTargets appends to rows the rows whose target type passes --target-types.
func appendTargets(rows, routeRows []dataRow, q *query) []dataRow {
	for i := range routeRows {
		if q.wantTarget(routeRows[i].TargetType) {
			rows = append(rows, routeRows[i])
		}
	}
	return rows
}

// parseRoute converts a route into one row per destination. A route without destinations, such as
// a blackhole, is one row without a target.
func parseRoute(route *types.NetworkRoute, cn *coreNetwork, c *routeCall) []dataRow {
	base := dataRow{
		Query:         c.cidr,
		CoreNetworkID: cn.id,
		Segment:       c.table.label(),
		Edge:          c.table.edge,
		CIDR:          aws.ToString(route.DestinationCidrBlock),
		PrefixListID:  aws.ToString(route.PrefixListId),
		Type:          Kebab(string(route.Type)),
		State:         Kebab(string(route.State)),
	}
	if len(route.Destinations) == 0 {
		if route.State == types.RouteStateBlackhole {
			base.TargetType = targetBlackhole
		}
		return []dataRow{base}
	}
	rows := make([]dataRow, 0, len(route.Destinations))
	for i := range route.Destinations {
		d := &route.Destinations[i]
		row := base
		row.TargetType = Kebab(aws.ToString(d.ResourceType))
		row.TargetID = aws.ToString(d.ResourceId)
		row.AttachmentID = aws.ToString(d.CoreNetworkAttachmentId)
		att := cn.attachments[row.AttachmentID]
		row.AttachmentName, row.OwnerID = att.name, att.owner
		row.OwnerName = cn.accountNames[att.owner]
		row.TargetSegment = aws.ToString(d.SegmentName)
		if g := aws.ToString(d.NetworkFunctionGroupName); g != "" && row.TargetSegment == "" {
			row.TargetSegment = FunctionGroupPrefix + g
		}
		row.TargetEdge = aws.ToString(d.EdgeLocation)
		if row.TargetSegment == "" {
			row.TargetSegment = att.segment
		}
		if row.TargetEdge == "" {
			row.TargetEdge = att.edges
		}
		row.TransitGatewayAttachmentID = aws.ToString(d.TransitGatewayAttachmentId)
		rows = append(rows, row)
	}
	return rows
}
