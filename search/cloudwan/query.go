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
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// Filter keys of the routes search.
const (
	FilterKeyCIDR          = "cidr"
	FilterKeyMatch         = "match"
	FilterKeySegment       = "segment"
	FilterKeyFunctionGroup = "function-group"
	FilterKeyEdge          = "edge"
	FilterKeyType          = "type"
	FilterKeyState         = "state"
	FilterKeyTargetType    = "target-type"
	FilterKeyCoreNetwork   = "core-network-id"

	// FilterKeyCommunity keeps the BGP routes that carry any of these communities.
	FilterKeyCommunity = "community"
	// FilterKeyASN keeps the BGP routes whose AS path has any of these AS numbers.
	FilterKeyASN = "asn"
)

// The --match modes: how a route must relate to a searched CIDR.
const (
	// MatchAny finds the routes equal to the CIDR, inside it or covering it: MatchExact,
	// MatchSubnetOf and MatchSupernetOf together.
	MatchAny = "any"
	// MatchExact finds the route of exactly that CIDR.
	MatchExact = "exact"
	// MatchLongest finds the most specific route covering the CIDR: the route traffic takes.
	MatchLongest = "longest"
	// MatchSubnetOf finds the routes inside the CIDR.
	MatchSubnetOf = "subnet-of"
	// MatchSupernetOf finds the routes covering the CIDR.
	MatchSupernetOf = "supernet-of"
)

// MatchNames returns the --match modes, the default first.
func MatchNames() []string {
	return []string{MatchAny, MatchExact, MatchLongest, MatchSubnetOf, MatchSupernetOf}
}

// TypeNames returns the route types, as awss writes them: propagated, static, system.
func TypeNames() []string { return KebabNames(types.RouteType("").Values()) }

// StateNames returns the route states, as awss writes them: active, blackhole.
func StateNames() []string { return KebabNames(types.RouteState("").Values()) }

// TargetTypeNames returns the target types AWS documents for a Cloud WAN next hop. Routes are not
// checked against this list: AWS may add types.
func TargetTypeNames() []string {
	return []string{
		"attachment", "blackhole", "connect-peer", "direct-connect-gateway", "transit-gateway-route-table",
		"vpc", "vpn",
	}
}

// KebabNames returns the values of an AWS enum in kebab case.
func KebabNames[T ~string](values []T) []string {
	names := make([]string, 0, len(values))
	for _, v := range values {
		names = append(names, Kebab(string(v)))
	}
	return names
}

// NormalizeCIDRs returns the CIDRs in canonical form: a bare IP address becomes its /32 (or /128),
// and host bits are cleared (10.20.1.5/16 becomes 10.20.0.0/16). IPv4 and IPv6 are accepted.
func NormalizeCIDRs(cidrs []string) ([]string, error) {
	out := make([]string, 0, len(cidrs))
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if !strings.Contains(c, "/") {
			addr, err := netip.ParseAddr(c)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR or IP address %q", c)
			}
			out = append(out, netip.PrefixFrom(addr, addr.BitLen()).String())
			continue
		}
		prefix, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR or IP address %q", c)
		}
		out = append(out, prefix.Masked().String())
	}
	return out, nil
}

// CheckFlags validates the CIDRs, the match mode, the route types and the route states before any
// AWS call.
func CheckFlags(cidrs, match, routeTypes, states []string) error {
	_, err := newQuery(map[string][]string{
		FilterKeyCIDR: cidrs, FilterKeyMatch: match, FilterKeyType: routeTypes, FilterKeyState: states,
	})
	return err
}

// query is the routes search asked by the filters.
type query struct {
	cidrs                     []string
	modes                     []string
	segments, groups, edges   *common.Matcher
	bySegment, byGroup        bool
	types                     []types.RouteType
	states                    []types.RouteState
	targetTypes, coreNetworks []string
	communities, asns         []string
}

// newQuery builds the query from a filters map, which it only reads: one map is shared by every
// search of a run.
func newQuery(filters map[string][]string) (*query, error) {
	q := &query{
		bySegment:    len(filters[FilterKeySegment]) > 0,
		byGroup:      len(filters[FilterKeyFunctionGroup]) > 0,
		coreNetworks: slices.Clone(filters[FilterKeyCoreNetwork]),
		communities:  slices.Clone(filters[FilterKeyCommunity]),
		asns:         slices.Clone(filters[FilterKeyASN]),
	}
	var err error
	if q.cidrs, err = NormalizeCIDRs(filters[FilterKeyCIDR]); err != nil {
		return nil, err
	}
	if q.modes, err = matchModes(filters[FilterKeyMatch]); err != nil {
		return nil, err
	}
	if err := q.buildMatchers(filters); err != nil {
		return nil, err
	}
	if q.types, err = enumValues[types.RouteType](filters[FilterKeyType], TypeNames(), "route type"); err != nil {
		return nil, err
	}
	if q.states, err = enumValues[types.RouteState](filters[FilterKeyState], StateNames(), "route state"); err != nil {
		return nil, err
	}
	for _, t := range filters[FilterKeyTargetType] {
		q.targetTypes = append(q.targetTypes, Kebab(t))
	}
	return q, nil
}

// buildMatchers compiles the segment, function group and edge globs.
func (q *query) buildMatchers(filters map[string][]string) error {
	var err error
	if q.segments, err = common.NewMatcher(filters[FilterKeySegment], false); err != nil {
		return fmt.Errorf("segments: %w", err)
	}
	if q.groups, err = common.NewMatcher(filters[FilterKeyFunctionGroup], false); err != nil {
		return fmt.Errorf("function groups: %w", err)
	}
	if q.edges, err = common.NewMatcher(filters[FilterKeyEdge], false); err != nil {
		return fmt.Errorf("edges: %w", err)
	}
	return nil
}

// matchModes returns the match modes of one GetNetworkRoutes call each. MatchAny, the default,
// is three calls: AWS does not document how the match fields of one call combine.
func matchModes(values []string) ([]string, error) {
	match := MatchAny
	if len(values) > 0 && values[0] != "" {
		match = values[0]
	}
	if len(values) > 1 {
		return nil, fmt.Errorf("--match takes one mode, not %d", len(values))
	}
	if match == MatchAny {
		return []string{MatchExact, MatchSubnetOf, MatchSupernetOf}, nil
	}
	if !slices.Contains(MatchNames(), match) {
		return nil, fmt.Errorf("invalid match %q; valid: %s", match, strings.Join(MatchNames(), ", "))
	}
	return []string{match}, nil
}

// enumValues converts kebab-case values to an AWS enum, checking each against valid.
func enumValues[T ~string](values, valid []string, what string) ([]T, error) {
	var out []T
	for _, v := range values {
		v = Kebab(v)
		if !slices.Contains(valid, v) {
			return nil, fmt.Errorf("invalid %s %q; valid: %s", what, v, strings.Join(valid, ", "))
		}
		out = append(out, T(strings.ToUpper(strings.ReplaceAll(v, "-", "_"))))
	}
	return out, nil
}

// wantSegments reports whether the segment route tables are searched: always, unless only
// function groups were asked.
func (q *query) wantSegments() bool { return q.bySegment || !q.byGroup }

// wantGroups reports whether the network function group route tables are searched: always,
// unless only segments were asked.
func (q *query) wantGroups() bool { return q.byGroup || !q.bySegment }

// wantTarget reports whether a route destination of targetType passes --target-types.
func (q *query) wantTarget(targetType string) bool {
	return len(q.targetTypes) == 0 || slices.Contains(q.targetTypes, Kebab(targetType))
}
