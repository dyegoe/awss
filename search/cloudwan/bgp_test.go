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
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// bgpRoute returns a BGP route of prefix learned from the next hop "type:id:attachment", with this
// AS path and these communities.
func bgpRoute(prefix, nextHop string, asPath, communities []string) types.CoreNetworkRoutingInformation {
	parts := strings.SplitN(nextHop, ":", 3)
	return types.CoreNetworkRoutingInformation{
		Prefix: aws.String(prefix), AsPath: asPath, Communities: communities,
		LocalPreference: aws.String("100"), Med: aws.String("0"),
		NextHop: &types.RoutingInformationNextHop{
			ResourceType: aws.String(parts[0]), ResourceId: aws.String(parts[1]),
			CoreNetworkAttachmentId: aws.String(parts[2]), IpAddress: aws.String("169.254.0.1"),
		},
	}
}

// bgpFixture returns fakeFixture with these BGP routes; attachment-dx is in the segment dc at
// eu-west-1 and is owned by 111111111111.
func bgpFixture(bgp map[string][][]types.CoreNetworkRoutingInformation) *fakeNetworkManager {
	f := fakeFixture(nil)
	f.attachments[0].SegmentName = aws.String("dc")
	f.attachments[0].EdgeLocations = []string{"eu-west-1"}
	f.attachments[0].OwnerAccountId = aws.String("111111111111")
	f.bgp = bgp
	return f
}

// collectBGP runs collect on a fresh BGP result set with these filters and returns it.
func collectBGP(t *testing.T, client bgpAPI, filters map[string][]string) *BGPResults {
	t.Helper()
	q, err := newBGPQuery(filters)
	if err != nil {
		t.Fatalf("newBGPQuery(%v) error = %v", filters, err)
	}
	r := NewBGP("network", Region, filters, "")
	r.collect(context.Background(), client, q)
	return r
}

// bgpPrefixes returns "segment edge prefix" of each row, in order.
func bgpPrefixes(rows []bgpRow) []string {
	out := []string{}
	for i := range rows {
		out = append(out, rows[i].Segment+" "+rows[i].Edge+" "+string(rows[i].Prefix))
	}
	return out
}

// TestBGPResults_collect checks a BGP route row: its BGP attributes, its next hop, the segment,
// edges and owner of the next hop taken from the attachment, and Installed from the installed
// route of its prefix. Only segments are read, at every edge
// location, never the network function groups.
func TestBGPResults_collect(t *testing.T) {
	client := bgpFixture(map[string][][]types.CoreNetworkRoutingInformation{
		"prod@eu-west-1": {{bgpRoute("10.50.0.0/16", "DIRECT_CONNECT_GATEWAY:dxgw-1:attachment-dx",
			[]string{"65010", "65020"}, []string{"65000:100", "65000:200"})}},
	})
	client.routes = map[string][][]types.NetworkRoute{
		"prod@eu-west-1": {{route("10.50.0.0/16", "DIRECT_CONNECT_GATEWAY:dxgw-1:attachment-dx")}},
	}
	r := NewBGP("network", Region, nil, "")
	r.AccountNames = map[string]string{"111111111111": "network"}
	q, err := newBGPQuery(map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	r.collect(context.Background(), client, q)

	if len(r.Errors) != 0 {
		t.Fatalf("collect() errors = %q, want none", r.Errors)
	}
	want := []bgpRow{{
		CoreNetworkID: "core-network-1", Segment: "prod", Edge: "eu-west-1", Prefix: "10.50.0.0/16", Installed: "yes",
		ASPath: "65010 65020", Communities: "65000:100,65000:200", LocalPreference: "100", MED: "0",
		NextHopType: "direct-connect-gateway", NextHopID: "dxgw-1", NextHopIP: "169.254.0.1",
		AttachmentID: "attachment-dx", AttachmentName: "dc-link", NextHopSegment: "dc", NextHopEdge: "eu-west-1",
		OwnerID: "111111111111", OwnerName: "network",
	}}
	if !reflect.DeepEqual(r.Data, want) {
		t.Errorf("collect() rows =\n%+v\nwant\n%+v", r.Data, want)
	}
	got := bgpCalls(client)
	if wantCalls := []string{"dc@eu-west-1", "prod@eu-west-1", "prod@us-east-1"}; !reflect.DeepEqual(got, wantCalls) {
		t.Errorf("collect() calls = %q, want %q", got, wantCalls)
	}
}

// bgpFilterPages returns the BGP routes of dc at eu-west-1, in two pages: 10.50.0.0/16 from AS
// 65010 with the community 65000:100 and 10.0.0.0/8 from AS 65020 (one AS path item "64512
// 65020"), both over Direct Connect, and
// 10.60.0.0/16 from a VPC; and the answers of the exact and supernet-of searches of 10.50.0.0/16.
func bgpFilterPages() map[string][][]types.CoreNetworkRoutingInformation {
	dx := bgpRoute("10.50.0.0/16", "direct-connect-gateway:dxgw-1:attachment-dx", []string{"65010"},
		[]string{"65000:100"})
	wide := bgpRoute("10.0.0.0/8", "direct-connect-gateway:dxgw-1:attachment-dx", []string{"64512 65020"}, nil)
	vpc := bgpRoute("10.60.0.0/16", "vpc:vpc-1:attachment-vpc", nil, nil)
	return map[string][][]types.CoreNetworkRoutingInformation{
		"dc@eu-west-1":                          {{dx, wide}, {vpc}},
		"dc@eu-west-1 exact=10.50.0.0/16":       {{dx}},
		"dc@eu-west-1 supernet-of=10.50.0.0/16": {{dx, wide}},
	}
}

// TestBGPResults_collect_filters checks the CIDR match modes, sent to AWS except longest, and the
// --next-hop-types, --communities and --asns filters, applied by awss.
func TestBGPResults_collect_filters(t *testing.T) {
	tests := []struct {
		name      string
		filters   map[string][]string
		want      []string
		wantCalls []string
	}{
		{
			name: "any", filters: map[string][]string{FilterKeyCIDR: {"10.50.0.0/16"}, FilterKeySegment: {"dc"}},
			want: []string{"dc eu-west-1 10.0.0.0/8", "dc eu-west-1 10.50.0.0/16"},
			wantCalls: []string{
				"dc@eu-west-1 exact=10.50.0.0/16", "dc@eu-west-1 subnet-of=10.50.0.0/16",
				"dc@eu-west-1 supernet-of=10.50.0.0/16",
			},
		},
		{
			name: "longest reads the whole segment",
			filters: map[string][]string{
				FilterKeyCIDR: {"10.50.1.1"}, FilterKeyMatch: {MatchLongest}, FilterKeySegment: {"dc"},
			},
			want: []string{"dc eu-west-1 10.50.0.0/16"}, wantCalls: []string{"dc@eu-west-1"},
		},
		{
			name:    "next hop type",
			filters: map[string][]string{FilterKeyTargetType: {"vpc"}, FilterKeySegment: {"dc"}},
			want:    []string{"dc eu-west-1 10.60.0.0/16"}, wantCalls: []string{"dc@eu-west-1"},
		},
		{
			name:    "community",
			filters: map[string][]string{FilterKeyCommunity: {"65000:100", "65000:999"}, FilterKeySegment: {"dc"}},
			want:    []string{"dc eu-west-1 10.50.0.0/16"}, wantCalls: []string{"dc@eu-west-1"},
		},
		{
			name:    "asn",
			filters: map[string][]string{FilterKeyASN: {"65020"}, FilterKeySegment: {"dc"}},
			want:    []string{"dc eu-west-1 10.0.0.0/8"}, wantCalls: []string{"dc@eu-west-1"},
		},
		{
			name:    "edge",
			filters: map[string][]string{FilterKeyEdge: {"us-*"}},
			want:    []string{}, wantCalls: []string{"prod@us-east-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := bgpFixture(bgpFilterPages())
			r := collectBGP(t, client, tt.filters)
			if len(r.Errors) != 0 {
				t.Fatalf("collect() errors = %q, want none", r.Errors)
			}
			got := bgpPrefixes(r.Data)
			sort.Strings(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) rows = %q, want %q", tt.filters, got, tt.want)
			}
			if calls := bgpCalls(client); !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Errorf("collect(%v) calls = %q, want %q", tt.filters, calls, tt.wantCalls)
			}
		})
	}
}

// bgpCalls returns the keys of the BGP calls of client, sorted, once each: a call of two pages
// is two requests.
func bgpCalls(client *fakeNetworkManager) []string {
	calls := []string{}
	for _, in := range client.bgpInputs {
		calls = append(calls, bgpKey(in))
	}
	sort.Strings(calls)
	return slices.Compact(calls)
}

// TestBGPResults_collect_errors checks that an AccessDenied is reported once with the permission
// hint, that another failure names its segment and edge, and that a failed core network list
// stops the search.
func TestBGPResults_collect_errors(t *testing.T) {
	tests := []struct {
		name      string
		client    *fakeNetworkManager
		wantError string
	}{
		{
			name: "access denied",
			client: &fakeNetworkManager{
				coreNetworks: bgpFixture(nil).coreNetworks, coreNetwork: bgpFixture(nil).coreNetwork,
				bgpErr: map[string]error{
					"prod@eu-west-1": accessDenied(), "prod@us-east-1": accessDenied(), "dc@eu-west-1": accessDenied(),
				},
			},
			wantError: "access denied reading the routes of core network core-network-1: " + deniedBGP,
		},
		{
			name: "one segment fails",
			client: func() *fakeNetworkManager {
				f := bgpFixture(nil)
				f.bgpErr = map[string]error{"dc@eu-west-1": errors.New("boom")}
				return f
			}(),
			wantError: "error reading the routes of dc at eu-west-1: listing the core network routing information: boom",
		},
		{
			name:      "no core networks",
			client:    &fakeNetworkManager{listErr: errors.New("denied")},
			wantError: "denied",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := collectBGP(t, tt.client, map[string][]string{})
			if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], tt.wantError) {
				t.Errorf("collect() errors = %q, want one with %q", r.Errors, tt.wantError)
			}
		})
	}
}

// TestRouteCall_bgpInput checks the input of each match mode: longest sends no match field.
func TestRouteCall_bgpInput(t *testing.T) {
	table := routeTable{coreNetworkID: "cn", segment: "prod", edge: "eu-west-1"}
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "no cidr", want: "prod@eu-west-1"},
		{name: "exact", mode: MatchExact, want: "prod@eu-west-1 exact=10.0.0.0/8"},
		{name: "longest", mode: MatchLongest, want: "prod@eu-west-1"},
		{name: "subnet-of", mode: MatchSubnetOf, want: "prod@eu-west-1 subnet-of=10.0.0.0/8"},
		{name: "supernet-of", mode: MatchSupernetOf, want: "prod@eu-west-1 supernet-of=10.0.0.0/8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := routeCall{table: table, cidr: "10.0.0.0/8", mode: tt.mode}
			in := c.bgpInput()
			if got := bgpKey(in); got != tt.want {
				t.Errorf("bgpInput() = %q, want %q", got, tt.want)
			}
			if aws.ToString(in.CoreNetworkId) != "cn" || aws.ToInt32(in.MaxResults) != PageSize {
				t.Errorf("bgpInput() CoreNetworkId, MaxResults = %q, %d, want cn, %d",
					aws.ToString(in.CoreNetworkId), aws.ToInt32(in.MaxResults), PageSize)
			}
		})
	}
}

// TestParseBGPRoute checks a route without a next hop, and a next hop that names its own segment
// and edge, which win over the attachment.
func TestParseBGPRoute(t *testing.T) {
	cn := &coreNetwork{id: "cn", attachments: map[string]attachmentInfo{"att": {segment: "dc", edges: "eu-west-1"}}}
	c := &routeCall{table: routeTable{segment: "prod", edge: "us-east-1"}, cidr: "10.0.0.0/8"}
	tests := []struct {
		name  string
		route types.CoreNetworkRoutingInformation
		want  bgpRow
	}{
		{
			name:  "no next hop",
			route: types.CoreNetworkRoutingInformation{Prefix: aws.String("10.0.0.0/8")},
			want:  bgpRow{Query: "10.0.0.0/8", CoreNetworkID: "cn", Segment: "prod", Edge: "us-east-1", Prefix: "10.0.0.0/8"},
		},
		{
			name: "next hop segment",
			route: types.CoreNetworkRoutingInformation{NextHop: &types.RoutingInformationNextHop{
				CoreNetworkAttachmentId: aws.String("att"), SegmentName: aws.String("shared"),
				EdgeLocation: aws.String("eu-central-1"), ResourceType: aws.String("vpc"),
			}},
			want: bgpRow{
				Query: "10.0.0.0/8", CoreNetworkID: "cn", Segment: "prod", Edge: "us-east-1", NextHopType: "vpc",
				AttachmentID: "att", NextHopSegment: "shared", NextHopEdge: "eu-central-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseBGPRoute(&tt.route, cn, c); got != tt.want {
				t.Errorf("parseBGPRoute() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// TestCheckBGPFlags checks the AS numbers on top of the CIDR and match checks.
func TestCheckBGPFlags(t *testing.T) {
	tests := []struct {
		name      string
		cidrs     []string
		match     []string
		asns      []string
		installed []string
		wantErr   string
	}{
		{
			name: "valid", cidrs: []string{"10.0.0.0/8"}, match: []string{"longest"}, asns: []string{"65010", "4200000000"},
			installed: []string{"yes", "none"},
		},
		{name: "bad installed", installed: []string{"maybe"}, wantErr: `invalid installed "maybe"`},
		{name: "bad cidr", cidrs: []string{"x"}, wantErr: "invalid CIDR"},
		{name: "bad asn", asns: []string{"AS65010"}, wantErr: `invalid AS number "AS65010"`},
		{name: "asn too large", asns: []string{"4294967296"}, wantErr: "invalid AS number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckBGPFlags(tt.cidrs, tt.match, tt.asns, tt.installed)
			if (err == nil) != (tt.wantErr == "") || (err != nil && !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("CheckBGPFlags() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// TestBGPResults_Search checks that Search builds the query and the client, and reports a failure
// of either.
func TestBGPResults_Search(t *testing.T) {
	tests := []struct {
		name      string
		filters   map[string][]string
		clientErr error
		wantLen   int
		wantError string
	}{
		{name: "searches", filters: map[string][]string{FilterKeySegment: {"dc"}}, wantLen: 1},
		{name: "bad filter", filters: map[string][]string{FilterKeyASN: {"x"}}, wantError: "error building filters"},
		{
			name: "no client", filters: map[string][]string{}, clientErr: errors.New("no profile"),
			wantError: "error getting aws config: no profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := bgpFixture(map[string][][]types.CoreNetworkRoutingInformation{
				"dc@eu-west-1": {{bgpRoute("10.0.0.0/8", "vpc:vpc-1:attachment-dx", nil, nil)}},
			})
			old := newBGPClient
			t.Cleanup(func() { newBGPClient = old })
			newBGPClient = func(string) (bgpAPI, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return client, nil
			}
			r := NewBGP("network", Region, tt.filters, "prefix")
			r.Search(context.Background())
			if r.Len() != tt.wantLen {
				t.Errorf("Search() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			if got := strings.Join(r.Errors, ";"); !strings.HasPrefix(got, tt.wantError) || (tt.wantError == "") != (got == "") {
				t.Errorf("Search() errors = %q, want prefix %q", got, tt.wantError)
			}
		})
	}
}

// TestBGPResults_accessors checks Len, the BaseResults getters, the headers and the rows.
func TestBGPResults_accessors(t *testing.T) {
	r := NewBGP("network", Region, nil, "prefix")
	r.Data = []bgpRow{{Prefix: "10.0.0.0/8"}, {Prefix: "10.1.0.0/16"}}
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "network" || r.GetRegion() != Region || r.GetSortField() != "prefix" {
		t.Errorf("getters = %q %q %q, want network %s prefix", r.GetProfile(), r.GetRegion(), r.GetSortField(), Region)
	}
	wantHeaders := []interface{}{
		"Query", "Segment\nEdge", "Prefix\nInstalled", "AS Path", "Communities", "Local Pref", "MED",
		"Next Hop Type\nNext Hop ID\nAttachment\nNext Hop Segment", "Owner ID\nName",
	}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders() = %q, want %q", got, wantHeaders)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestGetBGPSortFields checks a valid and an invalid BGP sort field, and the list of names.
func TestGetBGPSortFields(t *testing.T) {
	if got, err := GetBGPSortFields("as-path"); err != nil || got["as-path"] != "ASPath" {
		t.Errorf("GetBGPSortFields(as-path) = %v, %v, want ASPath", got, err)
	}
	if _, err := GetBGPSortFields("cidr"); err == nil {
		t.Error("GetBGPSortFields(cidr) error = nil, want an error")
	}
	if got := BGPSortFieldNames(); !slices.Contains(got, "next-hop-type") {
		t.Errorf("BGPSortFieldNames() = %q, want next-hop-type in it", got)
	}
	q, err := newBGPQuery(map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewBGP("network", Region, nil, "cidr")
	r.collect(context.Background(), bgpFixture(nil), q)
	if len(r.Errors) != 1 {
		t.Errorf("collect(sort cidr) errors = %q, want one", r.Errors)
	}
}

// bgpSummary returns "prefix installed next-hop-type next-hop-segment as-path local-pref med" of
// each row, in order.
func bgpSummary(rows []bgpRow) []string {
	out := []string{}
	for i := range rows {
		d := &rows[i]
		out = append(out, strings.Join([]string{
			string(d.Prefix), d.Installed, d.NextHopType, d.NextHopSegment, d.ASPath, d.LocalPreference, d.MED,
		}, " "))
	}
	return out
}

// TestBGPResults_collect_awsFixtures checks the BGP routes of a segment from the shape AWS really
// returns: an AS path of one AS number per item, no communities, a VPC route without BGP
// attributes, and a next hop that names only the attachment, its type and resource, so its
// segment comes from the attachments. A more specific prefix of the data centre arrives through a
// legacy Transit Gateway route table, and is not installed: no installed route of
// get-network-routes.json has that prefix.
func TestBGPResults_collect_awsFixtures(t *testing.T) {
	var bgp networkmanager.ListCoreNetworkRoutingInformationOutput
	loadFixture(t, "list-core-network-routing-information.json", &bgp)
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{
			name:    "segment",
			filters: map[string][]string{FilterKeySegment: {"dev"}},
			want: []string{
				"10.216.0.0/16 yes vpc dev   ",
				"10.0.0.0/16 yes direct-connect-gateway sharedsvc 65100 65201 65201 64700 0 0",
				"10.0.4.0/22 none transit-gateway-route-table sharedsvc 64512 65300 64700 0 100",
			},
		},
		{
			name:    "asn of the data centre",
			filters: map[string][]string{FilterKeySegment: {"dev"}, FilterKeyASN: {"65201"}},
			want:    []string{"10.0.0.0/16 yes direct-connect-gateway sharedsvc 65100 65201 65201 64700 0 0"},
		},
		{
			name:    "dropped by a policy",
			filters: map[string][]string{FilterKeySegment: {"dev"}, FilterKeyInstalled: {"none"}},
			want:    []string{"10.0.4.0/22 none transit-gateway-route-table sharedsvc 64512 65300 64700 0 100"},
		},
		{
			name:    "short next hop name",
			filters: map[string][]string{FilterKeySegment: {"dev"}, FilterKeyTargetType: {"dx"}},
			want:    []string{"10.0.0.0/16 yes direct-connect-gateway sharedsvc 65100 65201 65201 64700 0 0"},
		},
		{
			name: "longest",
			filters: map[string][]string{
				FilterKeySegment: {"dev"}, FilterKeyCIDR: {"10.0.5.1"}, FilterKeyMatch: {MatchLongest},
			},
			want: []string{"10.0.4.0/22 none transit-gateway-route-table sharedsvc 64512 65300 64700 0 100"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := awsFixture(t)
			client.bgp = map[string][][]types.CoreNetworkRoutingInformation{
				"dev@eu-central-1": {bgp.CoreNetworkRoutingInformation},
			}
			r := collectBGP(t, client, tt.filters)
			if len(r.Errors) != 0 {
				t.Fatalf("collect() errors = %q, want none", r.Errors)
			}
			if got := bgpSummary(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) rows =\n%q\nwant\n%q", tt.filters, got, tt.want)
			}
		})
	}
}

// TestInstalled_state checks the Installed value of a path: its attachment is the installed one,
// another attachment is, or no route of the prefix is installed.
func TestInstalled_state(t *testing.T) {
	in := installed{"10.0.0.0/16": {"attachment-dx1": true}, "10.9.0.0/16": {}}
	tests := []struct {
		name       string
		prefix     string
		attachment string
		want       string
	}{
		{name: "installed", prefix: "10.0.0.0/16", attachment: "attachment-dx1", want: InstalledYes},
		{name: "backup", prefix: "10.0.0.0/16", attachment: "attachment-dx2", want: InstalledNo},
		{name: "no attachment", prefix: "10.0.0.0/16", want: InstalledNo},
		{name: "blackhole installed", prefix: "10.9.0.0/16", attachment: "attachment-dx1", want: InstalledNo},
		{name: "dropped", prefix: "10.1.0.0/16", attachment: "attachment-dx1", want: InstalledNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := in.state(tt.prefix, tt.attachment); got != tt.want {
				t.Errorf("state(%q, %q) = %q, want %q", tt.prefix, tt.attachment, got, tt.want)
			}
		})
	}
}

// TestBGPResults_collect_installedError checks that a failed read of the installed routes fails
// the route table, with what was read.
func TestBGPResults_collect_installedError(t *testing.T) {
	client := bgpFixture(map[string][][]types.CoreNetworkRoutingInformation{
		"dc@eu-west-1": {{bgpRoute("10.0.0.0/8", "vpc:vpc-1:attachment-dx", nil, nil)}},
	})
	client.routeErr = map[string]error{"dc@eu-west-1": errors.New("boom")}
	r := collectBGP(t, client, map[string][]string{FilterKeySegment: {"dc"}})
	want := "error reading the routes of dc at eu-west-1: reading the installed routes: getting the network routes: boom"
	if len(r.Errors) != 1 || r.Errors[0] != want {
		t.Errorf("collect() errors = %q, want [%q]", r.Errors, want)
	}
}

// TestBGPResults_sortResults checks the route table order under the sort field: prefixes by
// address, and the installed path of a prefix first.
func TestBGPResults_sortResults(t *testing.T) {
	r := NewBGP("network", Region, nil, "segment")
	r.Data = []bgpRow{
		{Segment: "prod", Prefix: "10.100.0.0/16", Installed: InstalledYes},
		{Segment: "dev", Prefix: "10.2.0.0/16", Installed: InstalledNone},
		{Segment: "prod", Prefix: "10.2.0.0/16", Installed: InstalledNo},
		{Segment: "prod", Prefix: "10.2.0.0/16", Installed: InstalledYes},
		{Segment: "prod", Prefix: "10.0.0.0/8", Installed: ""},
	}
	if err := r.sortResults("segment"); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for i := range r.Data {
		got = append(got, r.Data[i].Segment+" "+string(r.Data[i].Prefix)+" "+r.Data[i].Installed)
	}
	want := []string{
		"dev 10.2.0.0/16 none", "prod 10.0.0.0/8 ", "prod 10.2.0.0/16 yes", "prod 10.2.0.0/16 no",
		"prod 10.100.0.0/16 yes",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortResults(segment) = %q, want %q", got, want)
	}
}

// TestBGPResults_sortResults_installed checks that --sort installed orders yes, no, none, then an
// empty value, not alphabetically, and keeps the rows of one value in route table order.
func TestBGPResults_sortResults_installed(t *testing.T) {
	r := NewBGP("network", Region, nil, "installed")
	r.Data = []bgpRow{
		{Segment: "prod", Prefix: "10.2.0.0/16", Installed: InstalledNone},
		{Segment: "prod", Prefix: "10.0.0.0/8", Installed: ""},
		{Segment: "prod", Prefix: "10.100.0.0/16", Installed: InstalledYes},
		{Segment: "dev", Prefix: "10.2.0.0/16", Installed: InstalledNo},
		{Segment: "dev", Prefix: "10.1.0.0/16", Installed: InstalledYes},
		{Segment: "dev", Prefix: "10.3.0.0/16", Installed: InstalledNone},
	}
	if err := r.sortResults("installed"); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for i := range r.Data {
		got = append(got, r.Data[i].Installed+" "+r.Data[i].Segment+" "+string(r.Data[i].Prefix))
	}
	want := []string{
		"yes dev 10.1.0.0/16", "yes prod 10.100.0.0/16", "no dev 10.2.0.0/16",
		"none dev 10.3.0.0/16", "none prod 10.2.0.0/16", " prod 10.0.0.0/8",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortResults(installed) = %q, want %q", got, want)
	}
}
