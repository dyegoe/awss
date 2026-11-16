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
	"errors"
	"fmt"
	"net/netip"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// routeWorkers caps the GetNetworkRoutes calls of one core network running at the same time.
//
// AWS has no call that reads several route tables at once: a core network has one route table per
// segment and edge location, and per network function group and edge location, and each is read on
// its own. The cap keeps a large core network from being throttled.
const routeWorkers = 5

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

// runCalls makes the route calls of one core network, at most routeWorkers at once, and adds their
// rows without duplicates: a route can match a CIDR in several modes.
//
// The first AccessDenied stops the core network: the profile is not the core network owner, and
// every other call would be denied too. It is reported once. When parent ends first, the calls
// not made are reported, so partial rows never look complete.
func (r *Results) runCalls(
	parent context.Context, client networkmanager.GetNetworkRoutesAPIClient, cn *coreNetwork, cs []routeCall, q *query,
) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	rows := make([][]dataRow, len(cs))
	errs := make([]error, len(cs))
	sem := make(chan struct{}, routeWorkers)
	var wg sync.WaitGroup
	for i := range cs {
		if !acquire(ctx, sem) {
			break
		}
		wg.Go(func() {
			defer func() { <-sem }()
			rows[i], errs[i] = routes(ctx, client, cn, &cs[i], q)
			if isAccessDenied(errs[i]) {
				cancel()
			}
		})
	}
	wg.Wait()

	r.addRows(rows)
	if !r.addCallErrors(cn.id, cs, errs) && parent.Err() != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("stopped before reading every route table of core network %s: %v",
			cn.id, parent.Err()))
	}
}

// acquire takes a slot of sem, and reports false when ctx ends first.
func acquire(ctx context.Context, sem chan struct{}) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// addRows appends the rows of every call in call order, skipping the duplicates.
func (r *Results) addRows(rows [][]dataRow) {
	seen := make(map[dataRow]bool, len(r.Data))
	for i := range r.Data {
		seen[r.Data[i]] = true
	}
	for _, callRows := range rows {
		for i := range callRows {
			if !seen[callRows[i]] {
				seen[callRows[i]] = true
				r.Data = append(r.Data, callRows[i])
			}
		}
	}
}

// addCallErrors appends the errors of the calls in call order, and reports whether one was
// AccessDenied: that one is reported once for the core network, without the calls it cancelled.
// A call stopped by the end of the parent context is left to runCalls, which reports it once.
func (r *Results) addCallErrors(coreNetworkID string, cs []routeCall, errs []error) bool {
	for _, err := range errs {
		if isAccessDenied(err) {
			r.Errors = append(r.Errors, fmt.Sprintf("access denied reading the routes of core network %s: "+
				"run it with a profile of the account that owns the core network: %v", coreNetworkID, err))
			return true
		}
	}
	for i, err := range errs {
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		t := &cs[i].table
		r.Errors = append(r.Errors, fmt.Sprintf("error reading the routes of %s at %s: %v", t.label(), t.edge, err))
	}
	return false
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
		found = longestMatches(found, c.cidr)
	}
	var rows []dataRow
	for i := range found {
		rows = appendTargets(rows, parseRoute(&found[i], cn, c), q)
	}
	return rows, nil
}

// longestMatches returns the routes whose CIDR is the longest prefix covering cidr: the routes the
// traffic to cidr takes. A route without a CIDR, such as a prefix list route, never matches.
func longestMatches(all []types.NetworkRoute, cidr string) []types.NetworkRoute {
	want, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil
	}
	best := -1
	var out []types.NetworkRoute
	for i := range all {
		p, err := netip.ParsePrefix(aws.ToString(all[i].DestinationCidrBlock))
		if err != nil || p.Bits() > want.Bits() || !p.Contains(want.Addr()) || p.Bits() < best {
			continue
		}
		if p.Bits() > best {
			best, out = p.Bits(), nil
		}
		out = append(out, all[i])
	}
	return out
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
