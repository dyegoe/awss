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
	"context"
	"errors"
	"maps"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

// fakeEC2 is an EC2 client that answers from fixed route tables, routes and attachments. It applies
// the AWS filters the search sends, as AWS documents them, so the tests check the filters too.
type fakeEC2 struct {
	tables      []types.TransitGatewayRouteTable
	routes      map[string][]types.TransitGatewayRoute
	attachments []types.TransitGatewayAttachment
	// routePage is the routes per SearchTransitGatewayRoutes page; 0 is every route at once.
	routePage int
	truncated map[string]bool

	tablesErr, attachmentsErr error
	routesErr                 map[string]error

	// propagations are the attachments that propagate to each route table, each written
	// attachment-id/state; propagationsErr fails the propagation call of an attachment or a route
	// table; attachmentPropagationCalls and tablePropagationCalls count the calls of each kind.
	propagations                                      map[string][]string
	propagationsErr                                   map[string]error
	attachmentPropagationCalls, tablePropagationCalls int

	// maxResults are the page sizes the calls asked for; searches are the filters of every
	// SearchTransitGatewayRoutes call; attachmentFilters those of the attachment calls.
	maxResults        []int32
	searches          [][]types.Filter
	attachmentFilters [][]types.Filter
}

// filterValues returns the values of the filter name in fs, and whether it is there.
func filterValues(fs []types.Filter, name string) ([]string, bool) {
	for _, f := range fs {
		if aws.ToString(f.Name) == name {
			return f.Values, true
		}
	}
	return nil, false
}

// keep reports whether value passes the filter name of fs: always when fs does not have it.
func keep(fs []types.Filter, name, value string) bool {
	values, ok := filterValues(fs, name)
	return !ok || slices.Contains(values, value)
}

func (f *fakeEC2) DescribeTransitGatewayRouteTables(
	_ context.Context, in *ec2.DescribeTransitGatewayRouteTablesInput, _ ...func(*ec2.Options),
) (*ec2.DescribeTransitGatewayRouteTablesOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	if f.tablesErr != nil {
		return nil, f.tablesErr
	}
	out := &ec2.DescribeTransitGatewayRouteTablesOutput{}
	for _, t := range f.tables {
		if keep(in.Filters, "transit-gateway-route-table-id", aws.ToString(t.TransitGatewayRouteTableId)) &&
			keep(in.Filters, "transit-gateway-id", aws.ToString(t.TransitGatewayId)) {
			out.TransitGatewayRouteTables = append(out.TransitGatewayRouteTables, t)
		}
	}
	return out, nil
}

func (f *fakeEC2) DescribeTransitGatewayAttachments(
	_ context.Context, in *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	f.attachmentFilters = append(f.attachmentFilters, in.Filters)
	if f.attachmentsErr != nil {
		return nil, f.attachmentsErr
	}
	out := &ec2.DescribeTransitGatewayAttachmentsOutput{}
	for _, a := range f.attachments {
		if attachmentFiltersKeep(in.Filters, &a) {
			out.TransitGatewayAttachments = append(out.TransitGatewayAttachments, a)
		}
	}
	return out, nil
}

// attachmentFiltersKeep reports whether the attachment a passes the DescribeTransitGatewayAttachments
// filters of fs.
func attachmentFiltersKeep(fs []types.Filter, a *types.TransitGatewayAttachment) bool {
	rt := ""
	if a.Association != nil {
		rt = aws.ToString(a.Association.TransitGatewayRouteTableId)
	}
	return keep(fs, awsFilterAttTGWID, aws.ToString(a.TransitGatewayId)) &&
		keep(fs, awsFilterAttID, aws.ToString(a.TransitGatewayAttachmentId)) &&
		keep(fs, awsFilterAttResourceType, string(a.ResourceType)) &&
		keep(fs, awsFilterAttResourceID, aws.ToString(a.ResourceId)) &&
		keep(fs, awsFilterAttAssociatedRT, rt)
}

func (f *fakeEC2) GetTransitGatewayAttachmentPropagations(
	_ context.Context, in *ec2.GetTransitGatewayAttachmentPropagationsInput, _ ...func(*ec2.Options),
) (*ec2.GetTransitGatewayAttachmentPropagationsOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	f.attachmentPropagationCalls++
	id := aws.ToString(in.TransitGatewayAttachmentId)
	if err := f.propagationsErr[id]; err != nil {
		return nil, err
	}
	out := &ec2.GetTransitGatewayAttachmentPropagationsOutput{}
	for _, table := range slices.Sorted(maps.Keys(f.propagations)) {
		for _, p := range f.propagations[table] {
			att, state, _ := strings.Cut(p, "/")
			if att == id {
				out.TransitGatewayAttachmentPropagations = append(out.TransitGatewayAttachmentPropagations,
					types.TransitGatewayAttachmentPropagation{
						TransitGatewayRouteTableId: aws.String(table), State: types.TransitGatewayPropagationState(state),
					})
			}
		}
	}
	return out, nil
}

func (f *fakeEC2) GetTransitGatewayRouteTablePropagations(
	_ context.Context, in *ec2.GetTransitGatewayRouteTablePropagationsInput, _ ...func(*ec2.Options),
) (*ec2.GetTransitGatewayRouteTablePropagationsOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	f.tablePropagationCalls++
	id := aws.ToString(in.TransitGatewayRouteTableId)
	if err := f.propagationsErr[id]; err != nil {
		return nil, err
	}
	out := &ec2.GetTransitGatewayRouteTablePropagationsOutput{}
	for _, p := range f.propagations[id] {
		att, state, _ := strings.Cut(p, "/")
		out.TransitGatewayRouteTablePropagations = append(out.TransitGatewayRouteTablePropagations,
			types.TransitGatewayRouteTablePropagation{
				TransitGatewayAttachmentId: aws.String(att), State: types.TransitGatewayPropagationState(state),
			})
	}
	return out, nil
}

func (f *fakeEC2) SearchTransitGatewayRoutes(
	_ context.Context, in *ec2.SearchTransitGatewayRoutesInput, _ ...func(*ec2.Options),
) (*ec2.SearchTransitGatewayRoutesOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	f.searches = append(f.searches, in.Filters)
	id := aws.ToString(in.TransitGatewayRouteTableId)
	if err := f.routesErr[id]; err != nil {
		return nil, err
	}
	var found []types.TransitGatewayRoute
	for _, rt := range f.routes[id] {
		if routeKept(in.Filters, &rt) {
			found = append(found, rt)
		}
	}
	start := len(aws.ToString(in.NextToken))
	end := len(found)
	if f.routePage > 0 {
		end = min(start+f.routePage, len(found))
	}
	out := &ec2.SearchTransitGatewayRoutesOutput{
		Routes: found[start:end], AdditionalRoutesAvailable: aws.Bool(f.truncated[id]),
	}
	if end < len(found) {
		out.NextToken = aws.String(strings.Repeat("x", end))
		out.AdditionalRoutesAvailable = aws.Bool(true)
	}
	return out, nil
}

// routeKept reports whether a route passes the SearchTransitGatewayRoutes filters fs.
func routeKept(fs []types.Filter, rt *types.TransitGatewayRoute) bool {
	if !keep(fs, awsFilterState, string(rt.State)) || !keep(fs, awsFilterType, string(rt.Type)) {
		return false
	}
	if !attachmentKept(fs, rt.TransitGatewayAttachments) {
		return false
	}
	for name, rel := range map[string]func(r, c netip.Prefix) bool{
		awsFilterExact:      func(r, c netip.Prefix) bool { return r == c },
		awsFilterSubnetOf:   func(r, c netip.Prefix) bool { return r.Bits() >= c.Bits() && c.Contains(r.Addr()) },
		awsFilterSupernetOf: func(r, c netip.Prefix) bool { return r.Bits() <= c.Bits() && r.Contains(c.Addr()) },
	} {
		values, ok := filterValues(fs, name)
		if !ok {
			continue
		}
		r, err := netip.ParsePrefix(aws.ToString(rt.DestinationCidrBlock))
		if err != nil || !rel(r, netip.MustParsePrefix(values[0])) {
			return false
		}
	}
	return true
}

// attachmentKept reports whether one of the attachments passes the attachment filters of fs, or
// fs has none.
func attachmentKept(fs []types.Filter, atts []types.TransitGatewayRouteAttachment) bool {
	names := []string{awsFilterAttachmentType, awsFilterAttachmentID, awsFilterResourceID}
	if !slices.ContainsFunc(names, func(n string) bool { _, ok := filterValues(fs, n); return ok }) {
		return true
	}
	return slices.ContainsFunc(atts, func(a types.TransitGatewayRouteAttachment) bool {
		return keep(fs, awsFilterAttachmentType, string(a.ResourceType)) &&
			keep(fs, awsFilterAttachmentID, aws.ToString(a.TransitGatewayAttachmentId)) &&
			keep(fs, awsFilterResourceID, aws.ToString(a.ResourceId))
	})
}

// table returns a route table of the transit gateway tgw-0a.
func table(id, name string) types.TransitGatewayRouteTable {
	return types.TransitGatewayRouteTable{
		TransitGatewayRouteTableId: aws.String(id), TransitGatewayId: aws.String("tgw-0a"),
		Tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String(name)}},
	}
}

// route returns an active route of cidr (a prefix list ID when it starts with pl-) to the
// attachments, each written id/type/resource.
func route(cidr string, typ types.TransitGatewayRouteType, atts ...string) types.TransitGatewayRoute {
	rt := types.TransitGatewayRoute{Type: typ, State: types.TransitGatewayRouteStateActive}
	if strings.HasPrefix(cidr, "pl-") {
		rt.PrefixListId = aws.String(cidr)
	} else {
		rt.DestinationCidrBlock = aws.String(cidr)
	}
	for _, a := range atts {
		p := strings.Split(a, "/")
		rt.TransitGatewayAttachments = append(rt.TransitGatewayAttachments, types.TransitGatewayRouteAttachment{
			TransitGatewayAttachmentId: aws.String(p[0]), ResourceType: types.TransitGatewayAttachmentResourceType(p[1]),
			ResourceId: aws.String(p[2]),
		})
	}
	return rt
}

// attachment returns an attachment of tgw-0a with a Name tag and a resource owner.
func attachment(id, name, owner string) types.TransitGatewayAttachment {
	return types.TransitGatewayAttachment{
		TransitGatewayAttachmentId: aws.String(id), TransitGatewayId: aws.String("tgw-0a"),
		ResourceOwnerId: aws.String(owner), Tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String(name)}},
	}
}

// associated returns an attachment of tgw-0a, as attachment does, associated with the route table
// tableID in the state.
func associated(
	id, name, owner, tableID string, state types.TransitGatewayAssociationState,
) types.TransitGatewayAttachment {
	att := attachment(id, name, owner)
	att.Association = &types.TransitGatewayAttachmentAssociation{
		TransitGatewayRouteTableId: aws.String(tableID), State: state,
	}
	return att
}

// fakeFixture returns a made-up transit gateway with two route tables: prod has VPC routes, a
// Direct Connect and VPN route with two attachments (ECMP), a default route to a peering, a
// blackhole and a prefix list route; dev has one VPC route. The prod VPC is associated with dev,
// the app VPC is still associating with prod, and the Direct Connect attachment is not associated.
func fakeFixture() *fakeEC2 {
	blackhole := route("10.99.0.0/16", types.TransitGatewayRouteTypeStatic)
	blackhole.State = types.TransitGatewayRouteStateBlackhole
	return &fakeEC2{
		tables: []types.TransitGatewayRouteTable{table("tgw-rtb-prod", "prod"), table("tgw-rtb-dev", "dev")},
		routes: map[string][]types.TransitGatewayRoute{
			"tgw-rtb-prod": {
				route("10.20.0.0/16", types.TransitGatewayRouteTypePropagated, "tgw-attach-vpc1/vpc/vpc-0prod"),
				route("10.20.8.0/21", types.TransitGatewayRouteTypeStatic, "tgw-attach-vpc2/vpc/vpc-0app"),
				route("10.50.0.0/16", types.TransitGatewayRouteTypePropagated,
					"tgw-attach-dx1/direct-connect-gateway/dxgw-1", "tgw-attach-vpn1/vpn/vpn-1"),
				route("0.0.0.0/0", types.TransitGatewayRouteTypeStatic, "tgw-attach-peer/peering/tgw-attach-cwan"),
				blackhole,
				route("pl-0abc", types.TransitGatewayRouteTypeStatic, "tgw-attach-vpc1/vpc/vpc-0prod"),
			},
			"tgw-rtb-dev": {
				route("10.30.0.0/16", types.TransitGatewayRouteTypePropagated, "tgw-attach-vpc3/vpc/vpc-0dev"),
			},
		},
		attachments: []types.TransitGatewayAttachment{
			associated("tgw-attach-vpc1", "prod-vpc", "333333333333", "tgw-rtb-dev",
				types.TransitGatewayAssociationStateAssociated),
			associated("tgw-attach-vpc2", "app-vpc", "444444444444", "tgw-rtb-prod",
				types.TransitGatewayAssociationStateAssociating),
			attachment("tgw-attach-dx1", "dc", "111111111111"),
		},
	}
}

// collectWith runs collect over f with the filters and the sort field, and returns the results.
func collectWith(t *testing.T, f *fakeEC2, filters map[string][]string, sortField string) *Results {
	t.Helper()
	q, err := newQuery(filters)
	if err != nil {
		t.Fatalf("newQuery(%v) error = %v", filters, err)
	}
	r := New("network", "eu-central-1", filters, sortField)
	r.AccountNames = map[string]string{"333333333333": "prod-account"}
	r.collect(context.Background(), f, q)
	return r
}

// keys returns route table|destination|attachment|query for each row.
func keys(rows []dataRow) []string {
	out := make([]string, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		dest := string(r.CIDR)
		if dest == "" {
			dest = r.PrefixListID
		}
		out = append(out, strings.Join([]string{r.RouteTableName, dest, r.AttachmentID, r.Query}, "|"))
	}
	return out
}

// TestResults_collect_cidrs checks the rows each match mode keeps, in route table order.
func TestResults_collect_cidrs(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{
			name: "no filter lists every route of every table", filters: map[string][]string{},
			want: []string{
				"dev|10.30.0.0/16|tgw-attach-vpc3|", "prod|0.0.0.0/0|tgw-attach-peer|",
				"prod|10.20.0.0/16|tgw-attach-vpc1|", "prod|10.20.8.0/21|tgw-attach-vpc2|",
				"prod|10.50.0.0/16|tgw-attach-dx1|", "prod|10.50.0.0/16|tgw-attach-vpn1|", "prod|10.99.0.0/16||",
				"prod|pl-0abc|tgw-attach-vpc1|",
			},
		},
		{
			name:    "any finds the routes covering an address",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.9.5"}},
			want: []string{
				"prod|0.0.0.0/0|tgw-attach-peer|10.20.9.5/32", "prod|10.20.0.0/16|tgw-attach-vpc1|10.20.9.5/32",
				"prod|10.20.8.0/21|tgw-attach-vpc2|10.20.9.5/32",
			},
		},
		{
			name:    "longest keeps the route traffic takes in each table",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.9.5", "10.30.1.1"}, FilterKeyMatch: {"longest"}},
			want: []string{
				"dev|10.30.0.0/16|tgw-attach-vpc3|10.30.1.1/32", "prod|0.0.0.0/0|tgw-attach-peer|10.30.1.1/32",
				"prod|10.20.8.0/21|tgw-attach-vpc2|10.20.9.5/32",
			},
		},
		{
			name:    "longest of an exact route",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.0.0/16"}, FilterKeyMatch: {"longest"}},
			want:    []string{"prod|10.20.0.0/16|tgw-attach-vpc1|10.20.0.0/16"},
		},
		{
			name:    "subnet-of",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.0.0/16"}, FilterKeyMatch: {"subnet-of"}},
			want: []string{
				"prod|10.20.0.0/16|tgw-attach-vpc1|10.20.0.0/16", "prod|10.20.8.0/21|tgw-attach-vpc2|10.20.0.0/16",
			},
		},
		{
			name:    "exact, one row per ECMP attachment",
			filters: map[string][]string{FilterKeyCIDR: {"10.50.0.0/16"}, FilterKeyMatch: {"exact"}},
			want:    []string{"prod|10.50.0.0/16|tgw-attach-dx1|10.50.0.0/16", "prod|10.50.0.0/16|tgw-attach-vpn1|10.50.0.0/16"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := collectWith(t, fakeFixture(), tt.filters, "route-table")
			if len(r.Errors) > 0 {
				t.Fatalf("collect(%v) errors = %v", tt.filters, r.Errors)
			}
			if got := keys(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) = %q, want %q", tt.filters, got, tt.want)
			}
		})
	}
}

// TestResults_collect_filters checks the rows the route table, route and attachment filters keep.
func TestResults_collect_filters(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{
			name:    "route table name and id",
			filters: map[string][]string{FilterKeyRouteTableName: {"d*"}, FilterKeyRouteTableID: {"tgw-rtb-dev"}},
			want:    []string{"dev|10.30.0.0/16|tgw-attach-vpc3|"},
		},
		{
			name: "another transit gateway", filters: map[string][]string{FilterKeyTransitGatewayID: {"tgw-0other"}},
			want: []string{},
		},
		{
			name: "blackholes", filters: map[string][]string{FilterKeyState: {"blackhole"}},
			want: []string{"prod|10.99.0.0/16||"},
		},
		{
			name:    "dx is short for direct-connect-gateway",
			filters: map[string][]string{FilterKeyAttachmentType: {"dx"}},
			want:    []string{"prod|10.50.0.0/16|tgw-attach-dx1|", "prod|10.50.0.0/16|tgw-attach-vpn1|"},
		},
		{
			name: "resource, attachment and type",
			filters: map[string][]string{
				FilterKeyResourceID: {"vpc-0prod"}, FilterKeyAttachmentID: {"tgw-attach-vpc1"}, FilterKeyType: {"propagated"},
			},
			want: []string{"prod|10.20.0.0/16|tgw-attach-vpc1|"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := collectWith(t, fakeFixture(), tt.filters, "route-table")
			if len(r.Errors) > 0 {
				t.Fatalf("collect(%v) errors = %v", tt.filters, r.Errors)
			}
			if got := keys(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) = %q, want %q", tt.filters, got, tt.want)
			}
		})
	}
}

// TestResults_collect_columns checks every column of a row, with the attachment name, owner and
// owner name, and a row whose attachment the attachment list does not have.
func TestResults_collect_columns(t *testing.T) {
	r := collectWith(t, fakeFixture(), map[string][]string{FilterKeyCIDR: {"10.20.0.0/16"}, FilterKeyMatch: {"exact"}}, "")
	want := dataRow{
		Query: "10.20.0.0/16", TransitGatewayID: "tgw-0a", RouteTableID: "tgw-rtb-prod", RouteTableName: "prod",
		CIDR: "10.20.0.0/16", Type: "propagated", State: "active", ResourceType: "vpc", ResourceID: "vpc-0prod",
		AttachmentID: "tgw-attach-vpc1", AttachmentName: "prod-vpc", AssocRTID: "tgw-rtb-dev",
		AssocRTName: "dev", OwnerID: "333333333333", OwnerName: "prod-account",
	}
	if len(r.Data) != 1 || !reflect.DeepEqual(r.Data[0], want) {
		t.Errorf("collect(10.20.0.0/16) = %+v, want [%+v]", r.Data, want)
	}

	f := fakeFixture()
	r = collectWith(t, f, map[string][]string{FilterKeyCIDR: {"0.0.0.0/0"}, FilterKeyMatch: {"exact"}}, "")
	if len(r.Data) != 1 || r.Data[0].AttachmentName != "" || r.Data[0].OwnerID != "" {
		t.Errorf("collect(0.0.0.0/0) = %+v, want one row without attachment name and owner", r.Data)
	}
	if got, _ := filterValues(f.attachmentFilters[0], "transit-gateway-id"); !reflect.DeepEqual(got, []string{"tgw-0a"}) {
		t.Errorf("attachments transit-gateway-id = %q, want [tgw-0a]", got)
	}
}

// TestResults_collect_associatedRouteTable checks the associated route table of each attachment:
// named even when --route-table-ids leaves its table out, and empty when the attachment is not
// associated or its association is not done.
func TestResults_collect_associatedRouteTable(t *testing.T) {
	r := collectWith(t, fakeFixture(), map[string][]string{FilterKeyRouteTableID: {"tgw-rtb-prod"}}, "")
	if len(r.Errors) > 0 {
		t.Fatalf("collect(tgw-rtb-prod) errors = %v", r.Errors)
	}
	want := map[string][2]string{
		"tgw-attach-vpc1": {"tgw-rtb-dev", "dev"},
		"tgw-attach-vpc2": {},
		"tgw-attach-dx1":  {},
		"tgw-attach-vpn1": {},
		"tgw-attach-peer": {},
		"":                {},
	}
	for i := range r.Data {
		row := &r.Data[i]
		if row.RouteTableID != "tgw-rtb-prod" {
			t.Errorf("collect(tgw-rtb-prod) row of route table %s, want only tgw-rtb-prod", row.RouteTableID)
		}
		got := [2]string{row.AssocRTID, row.AssocRTName}
		if got != want[row.AttachmentID] {
			t.Errorf("collect(tgw-rtb-prod) %s associated = %q, want %q", row.AttachmentID, got, want[row.AttachmentID])
		}
	}
}

// TestResults_collect_noAttachmentCall checks that rows without attachments, such as blackholes,
// make no attachment call.
func TestResults_collect_noAttachmentCall(t *testing.T) {
	f := fakeFixture()
	f.attachmentsErr = errors.New("must not be called")
	r := collectWith(t, f, map[string][]string{FilterKeyState: {"blackhole"}}, "")
	if len(r.Errors) > 0 || len(f.attachmentFilters) > 0 {
		t.Errorf("collect(blackhole) errors = %v, attachment calls = %d, want none", r.Errors, len(f.attachmentFilters))
	}
}

// TestResults_collect_errors checks that a failed list is recorded, that a route table that fails
// does not stop the others unless the profile is not authorised, and that a failed attachment
// list keeps the rows.
func TestResults_collect_errors(t *testing.T) {
	boom := errors.New("boom")
	denied := &smithy.GenericAPIError{Code: errCodeUnauthorized, Message: "no"}
	tests := []struct {
		name      string
		change    func(*fakeEC2)
		sortField string
		wantLen   int
		wantError string
	}{
		{
			name: "route tables", change: func(f *fakeEC2) { f.tablesErr = boom },
			wantError: "describing the transit gateway route tables: boom",
		},
		{
			name:    "routes of one table",
			change:  func(f *fakeEC2) { f.routesErr = map[string]error{"tgw-rtb-prod": boom} },
			wantLen: 1, wantError: "searching the routes of tgw-rtb-prod: boom",
		},
		{
			name:      "unauthorized stops the region",
			change:    func(f *fakeEC2) { f.routesErr = map[string]error{"tgw-rtb-prod": denied} },
			wantError: "searching the routes of tgw-rtb-prod: api error UnauthorizedOperation: no",
		},
		{
			name: "attachments", change: func(f *fakeEC2) { f.attachmentsErr = boom },
			wantLen: 8, wantError: "describing the transit gateway attachments: boom",
		},
		{
			name: "truncated", change: func(f *fakeEC2) { f.truncated = map[string]bool{"tgw-rtb-dev": true} },
			wantLen: 8, wantError: "route table tgw-rtb-dev has more routes than AWS returns in one search",
		},
		{name: "bad sort field", change: func(*fakeEC2) {}, sortField: "nope", wantLen: 8, wantError: "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fakeFixture()
			tt.change(f)
			r := collectWith(t, f, map[string][]string{}, tt.sortField)
			if r.Len() != tt.wantLen {
				t.Errorf("collect() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			if got := strings.Join(r.Errors, ";"); !strings.Contains(got, tt.wantError) {
				t.Errorf("collect() errors = %q, want %q", got, tt.wantError)
			}
		})
	}
}

// TestSearchRoutes_pages checks that every page of routes is read, that a page with a next token
// is not truncated, and that every call asks for the page size.
func TestSearchRoutes_pages(t *testing.T) {
	f := fakeFixture()
	f.routePage = 2
	r := collectWith(t, f, map[string][]string{FilterKeyRouteTableName: {"prod"}}, "")
	if len(r.Errors) > 0 || r.Len() != 7 {
		t.Errorf("collect() rows = %d, errors = %v, want 7 rows and no error", r.Len(), r.Errors)
	}
	for _, got := range f.maxResults {
		if got != pageSize {
			t.Errorf("MaxResults = %d, want %d", got, pageSize)
		}
	}
}

// TestRouteSearch_filters checks the AWS filters of the calls: every state when --states is not
// given, the user filters, and one CIDR in one mode per call.
func TestRouteSearch_filters(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{name: "no filter asks every state", filters: map[string][]string{}, want: []string{"state=active,blackhole"}},
		{
			name: "any is three calls", filters: map[string][]string{FilterKeyCIDR: {"10.0.0.0/8"}, FilterKeyType: {"Static"}},
			want: []string{
				"state=active,blackhole type=static route-search.exact-match=10.0.0.0/8",
				"state=active,blackhole type=static route-search.subnet-of-match=10.0.0.0/8",
				"state=active,blackhole type=static route-search.supernet-of-match=10.0.0.0/8",
			},
		},
		{
			name:    "longest is exact and supernet-of",
			filters: map[string][]string{FilterKeyCIDR: {"10.1.1.1"}, FilterKeyMatch: {"longest"}, FilterKeyState: {"active"}},
			want: []string{
				"state=active route-search.exact-match=10.1.1.1/32", "state=active route-search.supernet-of-match=10.1.1.1/32",
			},
		},
		{
			name: "attachment filters",
			filters: map[string][]string{
				FilterKeyAttachmentType: {"vpc"}, FilterKeyAttachmentID: {"tgw-attach-1"}, FilterKeyResourceID: {"vpc-1"},
			},
			want: []string{
				"state=active,blackhole attachment.resource-type=vpc attachment.transit-gateway-attachment-id=tgw-attach-1 " +
					"attachment.resource-id=vpc-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := newQuery(tt.filters)
			if err != nil {
				t.Fatalf("newQuery(%v) error = %v", tt.filters, err)
			}
			var got []string
			for _, s := range q.searches() {
				var parts []string
				for _, f := range s.filters(q) {
					parts = append(parts, aws.ToString(f.Name)+"="+strings.Join(f.Values, ","))
				}
				got = append(got, strings.Join(parts, " "))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filters(%v) = %q, want %q", tt.filters, got, tt.want)
			}
		})
	}
}

// TestCheckFlags checks the filters rejected before any AWS call, and some accepted.
func TestCheckFlags(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		wantErr string
	}{
		{name: "valid", filters: map[string][]string{
			FilterKeyCIDR: {"10.0.0.0/8", "fd00::1"}, FilterKeyMatch: {"supernet-of"}, FilterKeyType: {"propagated"},
			FilterKeyState: {"blackhole"}, FilterKeyAttachmentType: {"DX", "vpc"},
		}},
		{name: "cidr", filters: map[string][]string{FilterKeyCIDR: {"10.0.0"}}, wantErr: "invalid CIDR or IP address"},
		{name: "match", filters: map[string][]string{FilterKeyMatch: {"near"}}, wantErr: `invalid match "near"`},
		{name: "two matches", filters: map[string][]string{FilterKeyMatch: {"exact", "longest"}}, wantErr: "one mode, not 2"},
		{name: "type", filters: map[string][]string{FilterKeyType: {"system"}}, wantErr: `invalid route type "system"`},
		{name: "state", filters: map[string][]string{FilterKeyState: {"up"}}, wantErr: `invalid route state "up"`},
		{
			name: "attachment type", filters: map[string][]string{FilterKeyAttachmentType: {"tgw"}},
			wantErr: `invalid attachment type "tgw"`,
		},
		{name: "name glob", filters: map[string][]string{FilterKeyRouteTableName: {"["}}, wantErr: "route table names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckFlags(tt.filters)
			if (err == nil) != (tt.wantErr == "") || err != nil && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("CheckFlags(%v) = %v, want %q", tt.filters, err, tt.wantErr)
			}
		})
	}
}

// TestResults_sortRows checks a sort by CIDR in address order, the rows equal on it kept in
// route table order.
func TestResults_sortRows(t *testing.T) {
	r := collectWith(t, fakeFixture(), map[string][]string{FilterKeyType: {"propagated"}}, "cidr")
	want := []string{
		"prod|10.20.0.0/16|tgw-attach-vpc1|", "dev|10.30.0.0/16|tgw-attach-vpc3|",
		"prod|10.50.0.0/16|tgw-attach-dx1|", "prod|10.50.0.0/16|tgw-attach-vpn1|",
	}
	if got := keys(r.Data); !reflect.DeepEqual(got, want) {
		t.Errorf("collect() sorted by cidr = %q, want %q", got, want)
	}
}

// TestAttachmentTypeNames checks the fixed lists of the flags: the attachment types and the
// others.
func TestAttachmentTypeNames(t *testing.T) {
	tests := []struct {
		name string
		got  []string
		want string
	}{
		{name: "match", got: MatchNames(), want: "any"},
		{name: "types", got: TypeNames(), want: "propagated"},
		{name: "states", got: StateNames(), want: "blackhole"},
		{name: "attachment types", got: AttachmentTypeNames(), want: "direct-connect-gateway"},
		{name: "aliases", got: AttachmentAliasNames(), want: "dx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !slices.Contains(tt.got, tt.want) {
				t.Errorf("%s = %q, want %q in it", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestGetSortFields checks a valid and an invalid sort field.
func TestGetSortFields(t *testing.T) {
	if got, err := GetSortFields("route-table"); err != nil || got["route-table"] != "RouteTableName" {
		t.Errorf("GetSortFields(route-table) = %v, %v, want RouteTableName", got, err)
	}
	if _, err := GetSortFields("nope"); err == nil {
		t.Errorf("GetSortFields(nope) error = nil, want an error")
	}
	if got := SortFieldNames(); !slices.Contains(got, "attachment") {
		t.Errorf("SortFieldNames() = %q, want attachment in it", got)
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("network", "eu-central-1", nil, "route-table")
	r.Data = []dataRow{{RouteTableID: "1"}, {RouteTableID: "2"}}
	r.AddError("e")
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "network" || r.GetRegion() != "eu-central-1" || r.GetSortField() != "route-table" {
		t.Errorf("getters = %q %q %q, want network eu-central-1 route-table", r.GetProfile(), r.GetRegion(), r.GetSortField())
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"e"}) {
		t.Errorf("GetErrors() = %q, want [e]", got)
	}
	wantHeaders := []interface{}{
		"Query", "TGW ID", "Route Table ID\nName", "CIDR\nPrefix List", "Type", "State",
		"Resource Type\nResource ID", "Attachment ID\nName", "Associated RT\nName", "Owner ID\nName",
	}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders() = %q, want %q", got, wantHeaders)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestResults_Search checks Search with a fake client, bad filters and a client that cannot be
// built.
func TestResults_Search(t *testing.T) {
	runSearchCases(t, fakeFixture, 8, map[string][]string{FilterKeyState: {"x"}},
		func(filters map[string][]string) common.Results { return New("network", "eu-central-1", filters, "") })
}

// runSearchCases runs the Search of the results newResults builds over the fixture: it lists
// wantLen rows, reports badFilter as a filter error, and reports a client that cannot be built.
func runSearchCases(
	t *testing.T, fixture func() *fakeEC2, wantLen int, badFilter map[string][]string,
	newResults func(map[string][]string) common.Results,
) {
	t.Helper()
	tests := []struct {
		name      string
		filters   map[string][]string
		clientErr error
		wantLen   int
		wantError string
	}{
		{name: "lists", filters: map[string][]string{}, wantLen: wantLen},
		{name: "bad filter", filters: badFilter, wantError: "error building filters"},
		{
			name: "no client", filters: map[string][]string{}, clientErr: errors.New("no profile"),
			wantError: "error getting aws config: no profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := newClient
			t.Cleanup(func() { newClient = old })
			newClient = func(string, string) (ec2API, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return fixture(), nil
			}
			r := newResults(tt.filters)
			r.Search(context.Background())
			if r.Len() != tt.wantLen {
				t.Errorf("Search() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			got := strings.Join(r.GetErrors(), ";")
			if !strings.HasPrefix(got, tt.wantError) || (tt.wantError == "") != (got == "") {
				t.Errorf("Search() errors = %q, want prefix %q", got, tt.wantError)
			}
		})
	}
}

// TestNewClient checks that the client is built in the asked region. Loading the config makes no
// AWS call.
func TestNewClient(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	c, err := newClient("", "eu-west-1")
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}
	if got := c.(*ec2.Client).Options().Region; got != "eu-west-1" {
		t.Errorf("newClient() region = %q, want eu-west-1", got)
	}
}
