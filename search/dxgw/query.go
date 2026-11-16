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

package dxgw

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"unicode"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search/cloudwan"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/directconnect/types"
)

// Filter keys of the Direct Connect gateways search, all matched on the results.
const (
	FilterKeyID             = "id"
	FilterKeyName           = "name"
	FilterKeyCIDR           = "cidr"
	FilterKeyMatch          = "match"
	FilterKeyAssociatedType = "associated-type"
	FilterKeyAssociatedID   = "associated-id"
	FilterKeyState          = "state"
)

// The associated types: what a Direct Connect gateway is associated with.
const (
	AssociatedTransitGateway        = "transit-gateway"
	AssociatedVirtualPrivateGateway = "virtual-private-gateway"
	AssociatedCoreNetwork           = "core-network"
)

// AssociatedTypeNames returns the associated types: core-network, transit-gateway,
// virtual-private-gateway.
func AssociatedTypeNames() []string {
	return []string{AssociatedCoreNetwork, AssociatedTransitGateway, AssociatedVirtualPrivateGateway}
}

// StateNames returns the association states: associating, associated...
func StateNames() []string {
	return cloudwan.KebabNames(types.DirectConnectGatewayAssociationState("").Values())
}

// MatchNames returns the --match modes, the default first. An allowed prefix is advertised as a
// whole, so there is no longest match: supernet-of finds the prefixes that carry an address.
func MatchNames() []string {
	return []string{
		cloudwan.MatchAny, cloudwan.MatchExact, cloudwan.MatchSubnetOf, cloudwan.MatchSupernetOf,
	}
}

// gatewayType returns a gateway type as awss writes it: transitGateway becomes transit-gateway.
func gatewayType(t types.GatewayType) string {
	var b strings.Builder
	for i, c := range string(t) {
		if unicode.IsUpper(c) && i > 0 {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(c))
	}
	return b.String()
}

// CheckFlags validates the filters before any AWS call: the CIDRs, the match mode, the associated
// types, the states and the name globs.
func CheckFlags(filters map[string][]string) error {
	_, err := newQuery(filters)
	return err
}

// query is the search asked by the filters.
type query struct {
	ids, associatedIDs []string
	names              *common.Matcher
	cidrs              []netip.Prefix
	match              string
	associatedTypes    []string
	states             []string
}

// newQuery builds the query from a filters map, which it only reads: one map is shared by every
// search of a run.
func newQuery(filters map[string][]string) (*query, error) {
	q := &query{ids: filters[FilterKeyID], associatedIDs: filters[FilterKeyAssociatedID]}
	var err error
	if q.names, err = common.NewMatcher(filters[FilterKeyName], false); err != nil {
		return nil, fmt.Errorf("names: %w", err)
	}
	if q.cidrs, err = parseCIDRs(filters[FilterKeyCIDR]); err != nil {
		return nil, err
	}
	if q.match, err = matchMode(filters[FilterKeyMatch]); err != nil {
		return nil, err
	}
	q.associatedTypes, err = oneOf(filters[FilterKeyAssociatedType], AssociatedTypeNames(), "associated type")
	if err != nil {
		return nil, err
	}
	if q.states, err = oneOf(filters[FilterKeyState], StateNames(), "association state"); err != nil {
		return nil, err
	}
	return q, nil
}

// parseCIDRs returns the searched CIDRs: a bare IP address is its /32 (or /128).
func parseCIDRs(values []string) ([]netip.Prefix, error) {
	cidrs, err := cloudwan.NormalizeCIDRs(values)
	if err != nil {
		return nil, fmt.Errorf("normalizing the CIDRs: %w", err)
	}
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}
	return out, nil
}

// matchMode returns the --match mode, any when none is given.
func matchMode(values []string) (string, error) {
	switch {
	case len(values) > 1:
		return "", fmt.Errorf("--match takes one mode, not %d", len(values))
	case len(values) == 0 || values[0] == "":
		return cloudwan.MatchAny, nil
	case !slices.Contains(MatchNames(), values[0]):
		return "", fmt.Errorf("invalid match %q; valid: %s", values[0], strings.Join(MatchNames(), ", "))
	}
	return values[0], nil
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

// wantGateway reports whether a gateway passes --ids and --names, before its associations are
// read.
func (q *query) wantGateway(gw *dataRow) bool {
	return (len(q.ids) == 0 || slices.Contains(q.ids, gw.GatewayID)) && q.names.Match(gw.GatewayName)
}

// wantUnassociated reports whether a gateway without associations is listed: only when no filter
// is about the associations.
func (q *query) wantUnassociated() bool {
	return len(q.cidrs) == 0 && len(q.associatedTypes) == 0 && len(q.associatedIDs) == 0 && len(q.states) == 0
}

// wantAssociation reports whether an association passes --associated-types, --associated-ids and
// --states.
func (q *query) wantAssociation(a *dataRow) bool {
	return (len(q.associatedTypes) == 0 || slices.Contains(q.associatedTypes, a.AssociatedType)) &&
		(len(q.associatedIDs) == 0 || slices.Contains(q.associatedIDs, a.AssociatedID)) &&
		(len(q.states) == 0 || slices.Contains(q.states, a.State))
}

// prefixRows returns the rows of an association a: one per allowed prefix, or one per allowed
// prefix and searched CIDR it matches when --cidrs is given. Without --cidrs, an association
// without allowed prefixes, such as one to a core network, is one row.
func (q *query) prefixRows(a *dataRow, prefixes []types.RouteFilterPrefix) []dataRow {
	if len(q.cidrs) == 0 && len(prefixes) == 0 {
		return []dataRow{*a}
	}
	var rows []dataRow
	for _, p := range prefixes {
		row := *a
		row.AllowedPrefix = common.CIDR(aws.ToString(p.Cidr))
		if len(q.cidrs) == 0 {
			rows = append(rows, row)
			continue
		}
		for _, c := range q.cidrs {
			if matches(string(row.AllowedPrefix), c, q.match) {
				row.Query = c.String()
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// matches reports whether an allowed prefix relates to the searched CIDR c as the match mode asks.
// A prefix that does not parse never matches.
func matches(prefix string, c netip.Prefix, mode string) bool {
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return false
	}
	p = p.Masked()
	exact := p == c
	subnet := p.Bits() >= c.Bits() && c.Contains(p.Addr())
	supernet := p.Bits() <= c.Bits() && p.Contains(c.Addr())
	switch mode {
	case cloudwan.MatchExact:
		return exact
	case cloudwan.MatchSubnetOf:
		return subnet
	case cloudwan.MatchSupernetOf:
		return supernet
	}
	return subnet || supernet
}
