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

package tgw

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search/cloudwan"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Filter keys of the Transit Gateway routes search.
const (
	// FilterKeyCIDR is the CIDRs or IP addresses to find.
	FilterKeyCIDR = "cidr"
	// FilterKeyMatch is how a route must relate to each CIDR.
	FilterKeyMatch = "match"
	// FilterKeyRouteTableID picks route tables by ID, matched by awss.
	FilterKeyRouteTableID = "route-table-id"
	// FilterKeyRouteTableName picks route tables by Name tag (globs), matched by awss.
	FilterKeyRouteTableName = "route-table-name"
	// FilterKeyTransitGatewayID picks the route tables, or the attachments, of these transit
	// gateways, filtered by AWS.
	FilterKeyTransitGatewayID = "transit-gateway-id"
	// FilterKeyType keeps the routes of these types, filtered by AWS.
	FilterKeyType = "type"
	// FilterKeyState keeps the routes in these states, filtered by AWS.
	FilterKeyState = "state"
	// FilterKeyAttachmentType keeps the routes to attachments, or the attachments, of these resource
	// types, filtered by AWS.
	FilterKeyAttachmentType = "attachment-type"
	// FilterKeyAttachmentID keeps the routes to these attachments, or these attachments, filtered
	// by AWS.
	FilterKeyAttachmentID = "attachment-id"
	// FilterKeyResourceID keeps the routes to attachments, or the attachments, of these resources,
	// filtered by AWS.
	FilterKeyResourceID = "resource-id"
)

// The SearchTransitGatewayRoutes filters.
const (
	awsFilterExact          = "route-search.exact-match"
	awsFilterSubnetOf       = "route-search.subnet-of-match"
	awsFilterSupernetOf     = "route-search.supernet-of-match"
	awsFilterType           = "type"
	awsFilterState          = "state"
	awsFilterAttachmentType = "attachment.resource-type"
	awsFilterAttachmentID   = "attachment.transit-gateway-attachment-id"
	awsFilterResourceID     = "attachment.resource-id"
)

// MatchNames returns the --match modes, the default first: those of awss cloudwan routes.
func MatchNames() []string { return cloudwan.MatchNames() }

// TypeNames returns the route types: propagated, static.
func TypeNames() []string { return cloudwan.KebabNames(types.TransitGatewayRouteType("").Values()) }

// StateNames returns the route states: active, blackhole...
func StateNames() []string { return cloudwan.KebabNames(types.TransitGatewayRouteState("").Values()) }

// AttachmentTypeNames returns the resource types of a Transit Gateway attachment: vpc, vpn,
// direct-connect-gateway, peering...
func AttachmentTypeNames() []string {
	return cloudwan.KebabNames(types.TransitGatewayAttachmentResourceType("").Values())
}

// attachmentAliases maps the short attachment type names to the AWS resource types.
var attachmentAliases = map[string]string{"dx": string(types.TransitGatewayAttachmentResourceTypeDirectConnectGateway)}

// AttachmentAliasNames returns the short names --attachment-types takes alongside the AWS types: dx.
func AttachmentAliasNames() []string {
	out := make([]string, 0, len(attachmentAliases))
	for short := range attachmentAliases {
		out = append(out, short)
	}
	slices.Sort(out)
	return out
}

// CheckFlags validates the filters before any AWS call: the CIDRs, the match mode, the route
// types and states, the attachment types and the route table name globs.
func CheckFlags(filters map[string][]string) error {
	_, err := newQuery(filters)
	return err
}

// query is the search asked by the filters.
type query struct {
	cidrs           []string
	modes           []string
	longest         bool
	tableIDs, tgwID []string
	tableNames      *common.Matcher
	// routeFilters are the SearchTransitGatewayRoutes filters of every call, other than the CIDR.
	routeFilters []types.Filter
}

// newQuery builds the query from a filters map, which it only reads: one map is shared by every
// search of a run.
func newQuery(filters map[string][]string) (*query, error) {
	q := &query{
		tableIDs: slices.Clone(filters[FilterKeyRouteTableID]),
		tgwID:    slices.Clone(filters[FilterKeyTransitGatewayID]),
	}
	var err error
	if q.cidrs, err = cloudwan.NormalizeCIDRs(filters[FilterKeyCIDR]); err != nil {
		return nil, fmt.Errorf("normalizing the CIDRs: %w", err)
	}
	if q.modes, q.longest, err = matchModes(filters[FilterKeyMatch]); err != nil {
		return nil, err
	}
	if q.tableNames, err = common.NewMatcher(filters[FilterKeyRouteTableName], false); err != nil {
		return nil, fmt.Errorf("route table names: %w", err)
	}
	if err := q.buildRouteFilters(filters); err != nil {
		return nil, err
	}
	return q, nil
}

// keepsTable reports whether --route-table-ids and --route-table-names keep the route table of
// the route table columns t.
func (q *query) keepsTable(t *dataRow) bool {
	if len(q.tableIDs) > 0 && !slices.Contains(q.tableIDs, t.RouteTableID) {
		return false
	}
	return q.tableNames.Match(t.RouteTableName)
}

// buildRouteFilters checks the route and attachment filters and builds their AWS filters. AWS
// needs at least one filter, so without --states every state is asked.
func (q *query) buildRouteFilters(filters map[string][]string) error {
	routeTypes, err := oneOf(filters[FilterKeyType], TypeNames(), "route type")
	if err != nil {
		return err
	}
	states, err := oneOf(filters[FilterKeyState], StateNames(), "route state")
	if err != nil {
		return err
	}
	if len(states) == 0 {
		states = []string{string(types.TransitGatewayRouteStateActive), string(types.TransitGatewayRouteStateBlackhole)}
	}
	attTypes, err := attachmentTypes(filters[FilterKeyAttachmentType])
	if err != nil {
		return err
	}
	q.routeFilters = appendFilter(q.routeFilters, awsFilterState, states)
	q.routeFilters = appendFilter(q.routeFilters, awsFilterType, routeTypes)
	q.routeFilters = appendFilter(q.routeFilters, awsFilterAttachmentType, attTypes)
	q.routeFilters = appendFilter(q.routeFilters, awsFilterAttachmentID, filters[FilterKeyAttachmentID])
	q.routeFilters = appendFilter(q.routeFilters, awsFilterResourceID, filters[FilterKeyResourceID])
	return nil
}

// attachmentTypes returns the attachment resource types of values, in kebab case with the short
// names (dx) replaced by the AWS types, checking each.
func attachmentTypes(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = cloudwan.Kebab(v)
		if long, ok := attachmentAliases[v]; ok {
			v = long
		}
		out = append(out, v)
	}
	return oneOf(out, AttachmentTypeNames(), "attachment type")
}

// appendFilter appends the AWS filter name=values to fs, unless values is empty.
func appendFilter(fs []types.Filter, name string, values []string) []types.Filter {
	if len(values) == 0 {
		return fs
	}
	return append(fs, types.Filter{Name: aws.String(name), Values: slices.Clone(values)})
}

// matchModes returns the match modes of one SearchTransitGatewayRoutes call each, and whether
// awss keeps only the longest match. MatchAny, the default, is three calls: AWS does not document
// how several route-search filters of one call combine. MatchLongest is an exact and a
// supernet-of call, of which awss keeps the most specific route.
func matchModes(values []string) (modes []string, longest bool, err error) {
	match := cloudwan.MatchAny
	if len(values) > 0 && values[0] != "" {
		match = values[0]
	}
	switch {
	case len(values) > 1:
		return nil, false, fmt.Errorf("--match takes one mode, not %d", len(values))
	case match == cloudwan.MatchAny:
		return []string{cloudwan.MatchExact, cloudwan.MatchSubnetOf, cloudwan.MatchSupernetOf}, false, nil
	case match == cloudwan.MatchLongest:
		return []string{cloudwan.MatchExact, cloudwan.MatchSupernetOf}, true, nil
	case !slices.Contains(MatchNames(), match):
		return nil, false, fmt.Errorf("invalid match %q; valid: %s", match, strings.Join(MatchNames(), ", "))
	}
	return []string{match}, false, nil
}

// oneOf returns the values in kebab case, checking each against valid.
func oneOf(values, valid []string, what string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = cloudwan.Kebab(v)
		if !slices.Contains(valid, v) {
			return nil, fmt.Errorf("invalid %s %q; valid: %s", what, v, strings.Join(valid, ", "))
		}
		out = append(out, v)
	}
	return out, nil
}

// routeSearch is one SearchTransitGatewayRoutes call over a route table: at most one CIDR with one
// match mode.
type routeSearch struct {
	cidr, mode string
}

// searches returns the calls of one route table: every CIDR in every match mode, or one call for
// every route when no CIDR is searched.
func (q *query) searches() []routeSearch {
	if len(q.cidrs) == 0 {
		return []routeSearch{{}}
	}
	var out []routeSearch
	for _, c := range q.cidrs {
		for _, m := range q.modes {
			out = append(out, routeSearch{cidr: c, mode: m})
		}
	}
	return out
}

// filters returns the AWS filters of the call: the route filters of q, and the CIDR in its mode.
func (s *routeSearch) filters(q *query) []types.Filter {
	fs := slices.Clone(q.routeFilters)
	switch s.mode {
	case cloudwan.MatchExact:
		fs = appendFilter(fs, awsFilterExact, []string{s.cidr})
	case cloudwan.MatchSubnetOf:
		fs = appendFilter(fs, awsFilterSubnetOf, []string{s.cidr})
	case cloudwan.MatchSupernetOf:
		fs = appendFilter(fs, awsFilterSupernetOf, []string{s.cidr})
	}
	return fs
}
