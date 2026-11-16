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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
	"github.com/aws/smithy-go"
)

// fakeNetworkManager is a fake Network Manager client. routes holds the pages of routes of each
// call, keyed by callKey; routeErr fails the calls of a key. It records the route inputs.
type fakeNetworkManager struct {
	coreNetworks [][]types.CoreNetworkSummary
	listErr      error
	listCalls    int

	coreNetwork map[string]*types.CoreNetwork
	getErr      error

	attachments []types.Attachment
	attErr      error

	mu          sync.Mutex
	routes      map[string][][]types.NetworkRoute
	routeErr    map[string]error
	routeInputs []*networkmanager.GetNetworkRoutesInput
}

func (f *fakeNetworkManager) ListCoreNetworks(
	_ context.Context, in *networkmanager.ListCoreNetworksInput, _ ...func(*networkmanager.Options),
) (*networkmanager.ListCoreNetworksOutput, error) {
	if aws.ToInt32(in.MaxResults) != PageSize {
		return nil, fmt.Errorf("ListCoreNetworks MaxResults = %d, want %d", aws.ToInt32(in.MaxResults), PageSize)
	}
	if f.listErr != nil {
		return nil, f.listErr
	}
	page := f.listCalls
	f.listCalls++
	out := &networkmanager.ListCoreNetworksOutput{}
	if page < len(f.coreNetworks) {
		out.CoreNetworks = f.coreNetworks[page]
	}
	if page < len(f.coreNetworks)-1 {
		out.NextToken = aws.String(strconv.Itoa(page + 1))
	}
	return out, nil
}

func (f *fakeNetworkManager) GetCoreNetwork(
	_ context.Context, in *networkmanager.GetCoreNetworkInput, _ ...func(*networkmanager.Options),
) (*networkmanager.GetCoreNetworkOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &networkmanager.GetCoreNetworkOutput{CoreNetwork: f.coreNetwork[aws.ToString(in.CoreNetworkId)]}, nil
}

func (f *fakeNetworkManager) ListAttachments(
	_ context.Context, in *networkmanager.ListAttachmentsInput, _ ...func(*networkmanager.Options),
) (*networkmanager.ListAttachmentsOutput, error) {
	if aws.ToInt32(in.MaxResults) != PageSize {
		return nil, fmt.Errorf("ListAttachments MaxResults = %d, want %d", aws.ToInt32(in.MaxResults), PageSize)
	}
	if f.attErr != nil {
		return nil, f.attErr
	}
	return &networkmanager.ListAttachmentsOutput{Attachments: f.attachments}, nil
}

func (f *fakeNetworkManager) GetNetworkRoutes(
	_ context.Context, in *networkmanager.GetNetworkRoutesInput, _ ...func(*networkmanager.Options),
) (*networkmanager.GetNetworkRoutesOutput, error) {
	f.mu.Lock()
	f.routeInputs = append(f.routeInputs, in)
	f.mu.Unlock()
	if aws.ToInt32(in.MaxResults) != PageSize {
		return nil, fmt.Errorf("GetNetworkRoutes MaxResults = %d, want %d", aws.ToInt32(in.MaxResults), PageSize)
	}
	key := callKey(in)
	if err := f.routeErr[key]; err != nil {
		return nil, err
	}
	pages := f.routes[key]
	page, _ := strconv.Atoi(aws.ToString(in.NextToken))
	out := &networkmanager.GetNetworkRoutesOutput{}
	if page < len(pages) {
		out.NetworkRoutes = pages[page]
	}
	if page < len(pages)-1 {
		out.NextToken = aws.String(strconv.Itoa(page + 1))
	}
	return out, nil
}

// callKey identifies a GetNetworkRoutes call: "<table>@<edge>" and, when it matches a CIDR,
// " <mode>=<cidr>". A function group table is "nfg:<name>".
func callKey(in *networkmanager.GetNetworkRoutesInput) string {
	key := "?"
	if s := in.RouteTableIdentifier.CoreNetworkSegmentEdge; s != nil {
		key = aws.ToString(s.SegmentName) + "@" + aws.ToString(s.EdgeLocation)
	}
	if g := in.RouteTableIdentifier.CoreNetworkNetworkFunctionGroup; g != nil {
		key = "nfg:" + aws.ToString(g.NetworkFunctionGroupName) + "@" + aws.ToString(g.EdgeLocation)
	}
	for mode, cidrs := range map[string][]string{
		MatchExact: in.ExactCidrMatches, MatchLongest: in.LongestPrefixMatches,
		MatchSubnetOf: in.SubnetOfMatches, MatchSupernetOf: in.SupernetOfMatches,
	} {
		for _, c := range cidrs {
			key += " " + mode + "=" + c
		}
	}
	return key
}

// coreNetworkFixture returns a core network with segments prod (eu-west-1, us-east-1) and dc
// (eu-west-1), and the function group inspection (eu-west-1).
func coreNetworkFixture() *types.CoreNetwork {
	return &types.CoreNetwork{
		CoreNetworkId:   aws.String("core-network-1"),
		GlobalNetworkId: aws.String("global-network-1"),
		Segments: []types.CoreNetworkSegment{
			{Name: aws.String("prod"), EdgeLocations: []string{"eu-west-1", "us-east-1"}},
			{Name: aws.String("dc"), EdgeLocations: []string{"eu-west-1"}},
		},
		NetworkFunctionGroups: []types.CoreNetworkNetworkFunctionGroup{
			{Name: aws.String("inspection"), EdgeLocations: []string{"eu-west-1"}},
		},
	}
}

// route returns an active propagated route of cidr to each destination "type:id:attachment".
func route(cidr string, destinations ...string) types.NetworkRoute {
	r := types.NetworkRoute{
		DestinationCidrBlock: aws.String(cidr), State: types.RouteStateActive, Type: types.RouteTypePropagated,
	}
	for _, d := range destinations {
		parts := strings.SplitN(d, ":", 3)
		r.Destinations = append(r.Destinations, types.NetworkRouteDestination{
			ResourceType: aws.String(parts[0]), ResourceId: aws.String(parts[1]),
			CoreNetworkAttachmentId: aws.String(parts[2]), EdgeLocation: aws.String("eu-west-1"),
		})
	}
	return r
}

// fakeFixture returns a fake client with one core network (coreNetworkFixture), the attachment
// attachment-dx named dc-link, and these routes.
func fakeFixture(routes map[string][][]types.NetworkRoute) *fakeNetworkManager {
	return &fakeNetworkManager{
		coreNetworks: [][]types.CoreNetworkSummary{{{
			CoreNetworkId: aws.String("core-network-1"), GlobalNetworkId: aws.String("global-network-1"),
		}}},
		coreNetwork: map[string]*types.CoreNetwork{"core-network-1": coreNetworkFixture()},
		attachments: []types.Attachment{{
			AttachmentId: aws.String("attachment-dx"),
			Tags:         []types.Tag{{Key: aws.String("Name"), Value: aws.String("dc-link")}},
		}},
		routes: routes,
	}
}

// accessDenied returns the error of a missing permission.
func accessDenied() error {
	return &smithy.GenericAPIError{Code: errCodeAccessDenied, Message: "not authorized"}
}

// collectWith runs collect on a fresh result set with these filters and returns it.
func collectWith(t *testing.T, client routesAPI, filters map[string][]string, sortField string) *Results {
	t.Helper()
	q, err := newQuery(filters)
	if err != nil {
		t.Fatalf("newQuery(%v) error = %v", filters, err)
	}
	r := New("network", Region, filters, sortField)
	r.collect(context.Background(), client, q)
	return r
}

// summary returns "segment edge cidr target-type target-id attachment-name" of each row, in order.
func summary(rows []dataRow) []string {
	out := []string{}
	for i := range rows {
		r := &rows[i]
		out = append(out, strings.Join([]string{r.Segment, r.Edge, r.CIDR, r.TargetType, r.TargetID, r.AttachmentName}, " "))
	}
	return out
}

// TestResults_collect checks a CIDR search over every route table: one call per table and match
// mode, the attachment names, the function group label, and no duplicate rows when a route
// matches in several modes.
func TestResults_collect(t *testing.T) {
	dcRoute := route("10.50.0.0/16", "DIRECT_CONNECT_GATEWAY:dxgw-1:attachment-dx")
	client := fakeFixture(map[string][][]types.NetworkRoute{
		"prod@eu-west-1 exact=10.50.0.0/16":           {{dcRoute}},
		"prod@eu-west-1 supernet-of=10.50.0.0/16":     {{dcRoute}},
		"dc@eu-west-1 subnet-of=10.50.0.0/16":         {{route("10.50.1.0/24", "vpc:vpc-1:attachment-vpc")}},
		"nfg:inspection@eu-west-1 exact=10.50.0.0/16": {{dcRoute}},
	})

	r := collectWith(t, client, map[string][]string{FilterKeyCIDR: {"10.50.0.0/16"}}, "segment")

	if len(r.Errors) != 0 {
		t.Fatalf("collect() errors = %q, want none", r.Errors)
	}
	want := []string{
		"dc eu-west-1 10.50.1.0/24 vpc vpc-1 ",
		"nfg:inspection eu-west-1 10.50.0.0/16 direct-connect-gateway dxgw-1 dc-link",
		"prod eu-west-1 10.50.0.0/16 direct-connect-gateway dxgw-1 dc-link",
	}
	if got := summary(r.Data); !reflect.DeepEqual(got, want) {
		t.Errorf("collect() rows = %q, want %q", got, want)
	}
	// 4 tables (prod x2, dc, inspection) x 3 modes of "any".
	if got := len(client.routeInputs); got != 12 {
		t.Errorf("collect() GetNetworkRoutes calls = %d, want 12", got)
	}
	for _, in := range client.routeInputs {
		if got := aws.ToString(in.GlobalNetworkId); got != "global-network-1" {
			t.Errorf("GetNetworkRoutes GlobalNetworkId = %q, want global-network-1", got)
		}
	}
}

// loadFixture decodes testdata/name, a sanitized AWS CLI output of the call, into v. The AWS CLI
// writes the API field names, which encoding/json matches to the SDK struct fields.
func loadFixture(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
}

// awsFixture returns a fake client that answers with the testdata outputs: a core network with the
// segments sharedsvc, prd and dev at eu-central-1, its attachments, and the routes of dev.
func awsFixture(t *testing.T) *fakeNetworkManager {
	t.Helper()
	var list networkmanager.ListCoreNetworksOutput
	var core networkmanager.GetCoreNetworkOutput
	var atts networkmanager.ListAttachmentsOutput
	var routes networkmanager.GetNetworkRoutesOutput
	loadFixture(t, "list-core-networks.json", &list)
	loadFixture(t, "get-core-network.json", &core)
	loadFixture(t, "list-attachments.json", &atts)
	loadFixture(t, "get-network-routes.json", &routes)
	return &fakeNetworkManager{
		coreNetworks: [][]types.CoreNetworkSummary{list.CoreNetworks},
		coreNetwork:  map[string]*types.CoreNetwork{aws.ToString(core.CoreNetwork.CoreNetworkId): core.CoreNetwork},
		attachments:  atts.Attachments,
		routes:       map[string][][]types.NetworkRoute{"dev@eu-central-1": {routes.NetworkRoutes}},
	}
}

// TestResults_collect_awsFixtures checks the routes of a segment from the shapes AWS really
// returns: the destinations name only the attachment, its type and resource, so the target segment
// and edges come from the attachments. A route learned over Direct Connect reaches dev through the
// shared segment sharedsvc.
func TestResults_collect_awsFixtures(t *testing.T) {
	client := awsFixture(t)
	q, err := newQuery(map[string][]string{FilterKeySegment: {"dev"}})
	if err != nil {
		t.Fatal(err)
	}
	r := New("network", Region, nil, "cidr")
	r.AccountNames = map[string]string{"222222222222": "app-dev"}
	r.collect(context.Background(), client, q)

	if len(r.Errors) != 0 {
		t.Fatalf("collect() errors = %q, want none", r.Errors)
	}
	core, dev := "core-network-0aaaaaaaaaaaaaaaa", "dev"
	want := []dataRow{
		{
			CoreNetworkID: core, Segment: dev, Edge: "eu-central-1", CIDR: "10.0.0.0/16", Type: "propagated",
			State: "active", TargetType: "direct-connect-gateway", TargetID: "00000000-0000-0000-0000-000000000dx1",
			AttachmentID: "attachment-00000000000000dx1", AttachmentName: "dc-primary", TargetSegment: "sharedsvc",
			TargetEdge: "eu-central-1", OwnerID: "111111111111",
		},
		{
			CoreNetworkID: core, Segment: dev, Edge: "eu-central-1", CIDR: "10.100.0.0/16", Type: "propagated",
			State: "active", TargetType: "transit-gateway-route-table", TargetID: "tgw-rtb-000000000000000s1",
			AttachmentID: "attachment-00000000000000tg1", AttachmentName: "legacy-shared", TargetSegment: "sharedsvc",
			TargetEdge: "eu-central-1", OwnerID: "111111111111",
		},
		{
			CoreNetworkID: core, Segment: dev, Edge: "eu-central-1", CIDR: "10.216.0.0/16", Type: "propagated",
			State: "active", TargetType: "vpc", TargetID: "vpc-0000000000000dev1",
			AttachmentID: "attachment-0000000000000dev1", AttachmentName: "app-dev", TargetSegment: "dev",
			TargetEdge: "eu-central-1", OwnerID: "222222222222", OwnerName: "app-dev",
		},
	}
	if !reflect.DeepEqual(r.Data, want) {
		t.Errorf("collect() rows =\n%+v\nwant\n%+v", r.Data, want)
	}
	if got := callKey(client.routeInputs[0]); len(client.routeInputs) != 1 || got != "dev@eu-central-1" {
		t.Errorf("collect() calls = %d, first %q, want one for dev@eu-central-1", len(client.routeInputs), got)
	}
}

// TestNewAttachmentInfo checks the segment and edges a route row takes from its attachment: one
// edge, several (a Direct Connect gateway), and a network function group.
func TestNewAttachmentInfo(t *testing.T) {
	tests := []struct {
		name string
		att  types.Attachment
		want attachmentInfo
	}{
		{
			name: "vpc",
			att: types.Attachment{
				SegmentName: aws.String("dev"), EdgeLocation: aws.String("eu-central-1"), OwnerAccountId: aws.String("2"),
				Tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String("app")}},
			},
			want: attachmentInfo{name: "app", owner: "2", segment: "dev", edges: "eu-central-1"},
		},
		{
			name: "direct connect gateway",
			att:  types.Attachment{SegmentName: aws.String("sharedsvc"), EdgeLocations: []string{"eu-central-1", "eu-west-1"}},
			want: attachmentInfo{segment: "sharedsvc", edges: "eu-central-1,eu-west-1"},
		},
		{
			name: "function group",
			att:  types.Attachment{NetworkFunctionGroupName: aws.String("inspection")},
			want: attachmentInfo{segment: "nfg:inspection"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newAttachmentInfo(&tt.att); got != tt.want {
				t.Errorf("newAttachmentInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestResults_collect_filters checks that the table, type, state and target filters narrow the
// calls and the rows.
func TestResults_collect_filters(t *testing.T) {
	all := map[string][][]types.NetworkRoute{
		"prod@eu-west-1": {{
			route("10.0.0.0/16", "vpc:vpc-1:a1"), route("10.50.0.0/16", "DIRECT_CONNECT_GATEWAY:dxgw-1:attachment-dx"),
		}},
		"prod@us-east-1":           {{route("10.1.0.0/16", "vpc:vpc-2:a2")}},
		"dc@eu-west-1":             {{route("10.50.0.0/16", "DIRECT_CONNECT_GATEWAY:dxgw-1:attachment-dx")}},
		"nfg:inspection@eu-west-1": {{route("0.0.0.0/0", "attachment:x:a3")}},
	}
	tests := []struct {
		name      string
		filters   map[string][]string
		wantCalls []string
		wantRows  int
	}{
		{
			name: "every table", filters: map[string][]string{},
			wantCalls: []string{"dc@eu-west-1", "nfg:inspection@eu-west-1", "prod@eu-west-1", "prod@us-east-1"}, wantRows: 5,
		},
		{
			name: "segments only skip the function groups", filters: map[string][]string{FilterKeySegment: {"pr*"}},
			wantCalls: []string{"prod@eu-west-1", "prod@us-east-1"}, wantRows: 3,
		},
		{
			name: "function groups only skip the segments", filters: map[string][]string{FilterKeyFunctionGroup: {"inspection"}},
			wantCalls: []string{"nfg:inspection@eu-west-1"}, wantRows: 1,
		},
		{
			name: "edges", filters: map[string][]string{FilterKeyEdge: {"us-*"}},
			wantCalls: []string{"prod@us-east-1"}, wantRows: 1,
		},
		{
			name: "target types", filters: map[string][]string{FilterKeyTargetType: {"direct_connect_gateway"}},
			wantCalls: []string{"dc@eu-west-1", "nfg:inspection@eu-west-1", "prod@eu-west-1", "prod@us-east-1"}, wantRows: 2,
		},
		{
			name: "unknown core network", filters: map[string][]string{FilterKeyCoreNetwork: {"core-network-9"}},
			wantCalls: []string{}, wantRows: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakeFixture(all)
			r := collectWith(t, client, tt.filters, "")
			got := []string{}
			for _, in := range client.routeInputs {
				got = append(got, callKey(in))
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tt.wantCalls) {
				t.Errorf("collect(%v) calls = %q, want %q", tt.filters, got, tt.wantCalls)
			}
			if r.Len() != tt.wantRows {
				t.Errorf("collect(%v) rows = %d, want %d", tt.filters, r.Len(), tt.wantRows)
			}
		})
	}
}

// TestResults_collect_typesAndStates checks that --types and --states are sent to AWS.
func TestResults_collect_typesAndStates(t *testing.T) {
	client := fakeFixture(nil)
	collectWith(t, client, map[string][]string{
		FilterKeySegment: {"dc"}, FilterKeyType: {"static"}, FilterKeyState: {"blackhole"},
	}, "")
	if len(client.routeInputs) != 1 {
		t.Fatalf("collect() calls = %d, want 1", len(client.routeInputs))
	}
	in := client.routeInputs[0]
	if !reflect.DeepEqual(in.Types, []types.RouteType{types.RouteTypeStatic}) ||
		!reflect.DeepEqual(in.States, []types.RouteState{types.RouteStateBlackhole}) {
		t.Errorf("GetNetworkRoutes Types, States = %v, %v, want [STATIC], [BLACKHOLE]", in.Types, in.States)
	}
}

// TestResults_collect_errors checks that a failure is recorded in Errors: a failing list stops the
// search, a failing core network or route table is skipped, missing attachment names leave the
// rows, and AccessDenied is reported once.
func TestResults_collect_errors(t *testing.T) {
	ok := map[string][][]types.NetworkRoute{"dc@eu-west-1": {{route("10.50.0.0/16", "vpc:vpc-1:attachment-dx")}}}
	tests := []struct {
		name      string
		change    func(f *fakeNetworkManager)
		wantRows  int
		wantError string
	}{
		{
			name: "list fails", change: func(f *fakeNetworkManager) { f.listErr = errors.New("boom") },
			wantError: "listing the core networks: boom",
		},
		{
			name: "core network fails", change: func(f *fakeNetworkManager) { f.getErr = errors.New("boom") },
			wantError: "error reading core network core-network-1: boom",
		},
		{
			name: "no attachment names", change: func(f *fakeNetworkManager) { f.attErr = errors.New("boom") },
			wantRows: 1, wantError: "no attachment names: listing the attachments of core network core-network-1: boom",
		},
		{
			name: "one table fails",
			change: func(f *fakeNetworkManager) {
				f.routeErr = map[string]error{"prod@us-east-1": errors.New("boom")}
			},
			wantRows: 1, wantError: "error reading the routes of prod at us-east-1: getting the network routes: boom",
		},
		{
			name: "access denied",
			change: func(f *fakeNetworkManager) {
				f.routeErr = map[string]error{}
				for _, k := range []string{"dc@eu-west-1", "prod@eu-west-1", "prod@us-east-1", "nfg:inspection@eu-west-1"} {
					f.routeErr[k] = accessDenied()
				}
			},
			wantError: "access denied reading the routes of core network core-network-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakeFixture(ok)
			tt.change(client)
			r := collectWith(t, client, map[string][]string{}, "")
			if r.Len() != tt.wantRows {
				t.Errorf("collect() rows = %d, want %d", r.Len(), tt.wantRows)
			}
			if len(r.Errors) != 1 || !strings.HasPrefix(r.Errors[0], tt.wantError) {
				t.Errorf("collect() errors = %q, want one starting with %q", r.Errors, tt.wantError)
			}
		})
	}
}

// TestResults_collect_cancelled checks that a search whose context ends before the route tables
// are read reports it, instead of an empty result that looks complete.
func TestResults_collect_cancelled(t *testing.T) {
	q, err := newQuery(map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := New("network", Region, map[string][]string{}, "")
	cn := &coreNetwork{id: "core-network-1"}
	r.runCalls(ctx, fakeFixture(nil), cn, calls([]routeTable{{segment: "dc", edge: "eu-west-1"}}, q), q)
	if len(r.Errors) != 1 || !strings.HasPrefix(r.Errors[0], "stopped before reading every route table") {
		t.Errorf("runCalls(cancelled) errors = %q, want one stopped error", r.Errors)
	}
}

// TestResults_collect_owner checks that a route row names the owner account of its attachment.
func TestResults_collect_owner(t *testing.T) {
	client := fakeFixture(map[string][][]types.NetworkRoute{
		"dc@eu-west-1": {{route("10.0.0.0/8", "vpc:vpc-1:attachment-dx")}},
	})
	client.attachments[0].OwnerAccountId = aws.String("222222222222")
	q, err := newQuery(map[string][]string{FilterKeySegment: {"dc"}})
	if err != nil {
		t.Fatal(err)
	}
	r := New("network", Region, map[string][]string{}, "")
	r.AccountNames = map[string]string{"222222222222": "app-prod"}
	r.collect(context.Background(), client, q)
	if r.Len() != 1 || r.Data[0].OwnerID != "222222222222" || r.Data[0].OwnerName != "app-prod" {
		t.Errorf("collect() rows = %+v, want one owned by 222222222222 app-prod", r.Data)
	}
}

// TestResults_collect_nilCoreNetwork checks that a core network AWS returns empty is skipped.
func TestResults_collect_nilCoreNetwork(t *testing.T) {
	client := fakeFixture(nil)
	client.coreNetwork = map[string]*types.CoreNetwork{}
	r := collectWith(t, client, map[string][]string{}, "")
	if r.Len() != 0 || len(r.Errors) != 0 || len(client.routeInputs) != 0 {
		t.Errorf("collect() = %d rows, errors %q, %d calls, want nothing", r.Len(), r.Errors, len(client.routeInputs))
	}
}

// TestResults_collect_badSortField checks that an unknown sort field is an error, not a panic.
func TestResults_collect_badSortField(t *testing.T) {
	r := collectWith(t, fakeFixture(nil), map[string][]string{}, "nope")
	if len(r.Errors) != 1 {
		t.Errorf("collect() errors = %q, want one sort error", r.Errors)
	}
}

// TestRoutes checks that every page of a call is read.
func TestRoutes(t *testing.T) {
	client := fakeFixture(map[string][][]types.NetworkRoute{
		"dc@eu-west-1": {{route("10.0.0.0/16")}, {route("10.1.0.0/16")}},
	})
	q, err := newQuery(map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	c := routeCall{table: routeTable{coreNetworkID: "core-network-1", segment: "dc", edge: "eu-west-1"}}
	rows, err := routes(context.Background(), client, &coreNetwork{id: "core-network-1"}, &c, q)
	if err != nil {
		t.Fatalf("routes() error = %v", err)
	}
	if len(rows) != 2 || len(client.routeInputs) != 2 {
		t.Errorf("routes() = %d rows in %d calls, want 2 in 2", len(rows), len(client.routeInputs))
	}
}

// TestRouteCall_input checks that each match mode sets only its own field, and the route table
// identifier of a segment and of a function group.
func TestRouteCall_input(t *testing.T) {
	q, err := newQuery(map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	seg := routeTable{coreNetworkID: "cn", segment: "prod", edge: "eu-west-1"}
	nfg := routeTable{coreNetworkID: "cn", group: "inspection", edge: "eu-west-1"}
	tests := []struct {
		name string
		call routeCall
		want string
	}{
		{name: "no cidr", call: routeCall{table: seg}, want: "prod@eu-west-1"},
		{
			name: "exact", call: routeCall{table: seg, cidr: "10.0.0.0/8", mode: MatchExact},
			want: "prod@eu-west-1 exact=10.0.0.0/8",
		},
		{
			name: "longest", call: routeCall{table: seg, cidr: "10.0.0.1/32", mode: MatchLongest},
			want: "prod@eu-west-1 longest=10.0.0.1/32",
		},
		{
			name: "subnet-of", call: routeCall{table: seg, cidr: "10.0.0.0/8", mode: MatchSubnetOf},
			want: "prod@eu-west-1 subnet-of=10.0.0.0/8",
		},
		{
			name: "supernet-of", call: routeCall{table: nfg, cidr: "10.0.0.0/8", mode: MatchSupernetOf},
			want: "nfg:inspection@eu-west-1 supernet-of=10.0.0.0/8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.call.input("gn", q)
			if got := callKey(in); got != tt.want {
				t.Errorf("input() = %q, want %q", got, tt.want)
			}
			if aws.ToString(in.GlobalNetworkId) != "gn" || aws.ToInt32(in.MaxResults) != PageSize {
				t.Errorf("input() GlobalNetworkId, MaxResults = %q, %d, want gn, %d",
					aws.ToString(in.GlobalNetworkId), aws.ToInt32(in.MaxResults), PageSize)
			}
		})
	}
}

// TestParseRoute checks a route without destinations, a route to a function group, and the row
// fields of a destination.
func TestParseRoute(t *testing.T) {
	cn := &coreNetwork{
		id:           "cn",
		attachments:  map[string]attachmentInfo{"att-1": {name: "dc-link", owner: "222222222222"}},
		accountNames: map[string]string{"222222222222": "network"},
	}
	c := &routeCall{table: routeTable{segment: "prod", edge: "eu-west-1"}, cidr: "10.0.0.0/8"}
	tests := []struct {
		name  string
		route types.NetworkRoute
		want  []dataRow
	}{
		{
			name: "blackhole",
			route: types.NetworkRoute{
				DestinationCidrBlock: aws.String("10.9.0.0/16"), State: types.RouteStateBlackhole, Type: types.RouteTypeStatic,
			},
			want: []dataRow{{
				Query: "10.0.0.0/8", CoreNetworkID: "cn", Segment: "prod", Edge: "eu-west-1", CIDR: "10.9.0.0/16",
				Type: "static", State: "blackhole", TargetType: "blackhole",
			}},
		},
		{
			name: "function group and transit gateway",
			route: types.NetworkRoute{
				DestinationCidrBlock: aws.String("10.8.0.0/16"), PrefixListId: aws.String("pl-1"),
				Type: types.RouteTypePropagated, State: types.RouteStateActive,
				Destinations: []types.NetworkRouteDestination{{
					CoreNetworkAttachmentId: aws.String("att-1"), NetworkFunctionGroupName: aws.String("inspection"),
					ResourceType: aws.String("transit-gateway-route-table"), ResourceId: aws.String("tgw-rtb-1"),
					TransitGatewayAttachmentId: aws.String("tgw-attach-1"), EdgeLocation: aws.String("us-east-1"),
				}},
			},
			want: []dataRow{{
				Query: "10.0.0.0/8", CoreNetworkID: "cn", Segment: "prod", Edge: "eu-west-1", CIDR: "10.8.0.0/16",
				PrefixListID: "pl-1", Type: "propagated", State: "active", TargetType: "transit-gateway-route-table",
				TargetID: "tgw-rtb-1", AttachmentID: "att-1", AttachmentName: "dc-link", TargetSegment: "nfg:inspection",
				TargetEdge: "us-east-1", TransitGatewayAttachmentID: "tgw-attach-1",
				OwnerID: "222222222222", OwnerName: "network",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRoute(&tt.route, cn, c); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRoute() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestRouteTables checks which route tables a query selects.
func TestRouteTables(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{
			name: "all", filters: map[string][]string{},
			want: []string{"prod@eu-west-1", "prod@us-east-1", "dc@eu-west-1", "nfg:inspection@eu-west-1"},
		},
		{
			name:    "both kinds asked",
			filters: map[string][]string{FilterKeySegment: {"dc"}, FilterKeyFunctionGroup: {"*"}},
			want:    []string{"dc@eu-west-1", "nfg:inspection@eu-west-1"},
		},
		{name: "no edge", filters: map[string][]string{FilterKeyEdge: {"ap-*"}}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := newQuery(tt.filters)
			if err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, tb := range routeTables("cn", coreNetworkFixture(), q) {
				got = append(got, tb.label()+"@"+tb.edge)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("routeTables(%v) = %q, want %q", tt.filters, got, tt.want)
			}
		})
	}
}

// TestCalls checks the calls of the tables: one per table without CIDRs, one per CIDR and mode
// otherwise.
func TestCalls(t *testing.T) {
	tables := []routeTable{{segment: "a"}, {segment: "b"}}
	tests := []struct {
		name    string
		filters map[string][]string
		want    int
	}{
		{name: "no cidr", filters: map[string][]string{}, want: 2},
		{name: "any", filters: map[string][]string{FilterKeyCIDR: {"10.0.0.0/8", "10.1.1.1"}}, want: 12},
		{
			name:    "longest",
			filters: map[string][]string{FilterKeyCIDR: {"10.0.0.0/8"}, FilterKeyMatch: {MatchLongest}}, want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := newQuery(tt.filters)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(calls(tables, q)); got != tt.want {
				t.Errorf("calls(%v) = %d, want %d", tt.filters, got, tt.want)
			}
		})
	}
}

// TestCoreNetworks checks that every page is read and the IDs filter the core networks.
func TestCoreNetworks(t *testing.T) {
	pages := [][]types.CoreNetworkSummary{
		{{CoreNetworkId: aws.String("cn-1")}},
		{{CoreNetworkId: aws.String("cn-2")}},
	}
	tests := []struct {
		name string
		ids  []string
		want []string
	}{
		{name: "all", want: []string{"cn-1", "cn-2"}},
		{name: "by id", ids: []string{"cn-2"}, want: []string{"cn-2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nets, err := CoreNetworks(context.Background(), &fakeNetworkManager{coreNetworks: pages}, tt.ids)
			if err != nil {
				t.Fatalf("CoreNetworks() error = %v", err)
			}
			got := []string{}
			for _, n := range nets {
				got = append(got, aws.ToString(n.CoreNetworkId))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CoreNetworks(%v) = %q, want %q", tt.ids, got, tt.want)
			}
		})
	}
}

// TestAttachments checks that the attachments are listed and a failure names the core network.
func TestAttachments(t *testing.T) {
	f := fakeFixture(nil)
	atts, err := Attachments(context.Background(), f, "cn-1")
	if err != nil || len(atts) != 1 {
		t.Errorf("Attachments() = %d, %v, want 1, nil", len(atts), err)
	}
	f.attErr = errors.New("boom")
	if _, err := Attachments(context.Background(), f, "cn-1"); err == nil || !strings.Contains(err.Error(), "cn-1") {
		t.Errorf("Attachments() error = %v, want one naming cn-1", err)
	}
}

// TestNormalizeCIDRs checks bare addresses, host bits, IPv6 and invalid values.
func TestNormalizeCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{name: "ipv4 address", in: []string{"10.1.2.3"}, want: []string{"10.1.2.3/32"}},
		{name: "host bits", in: []string{" 10.1.2.3/16"}, want: []string{"10.1.0.0/16"}},
		{
			name: "ipv6", in: []string{"2001:db8::1", "2001:db8::/32"},
			want: []string{"2001:db8::1/128", "2001:db8::/32"},
		},
		{name: "bad address", in: []string{"10.1.2"}, wantErr: true},
		{name: "bad prefix", in: []string{"10.0.0.0/33"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCIDRs(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeCIDRs(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeCIDRs(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestCheckFlags checks the validation of the flags before any AWS call.
func TestCheckFlags(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		match   []string
		types   []string
		states  []string
		wantErr string
	}{
		{
			name: "valid", cidrs: []string{"10.0.0.0/8"}, match: []string{"longest"}, types: []string{"PROPAGATED"},
			states: []string{"active"},
		},
		{name: "default match"},
		{name: "bad cidr", cidrs: []string{"x"}, wantErr: "invalid CIDR"},
		{name: "bad match", match: []string{"nearest"}, wantErr: "invalid match"},
		{name: "two modes", match: []string{"exact", "longest"}, wantErr: "--match takes one mode"},
		{name: "bad type", types: []string{"dynamic"}, wantErr: "invalid route type"},
		{name: "bad state", states: []string{"down"}, wantErr: "invalid route state"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckFlags(tt.cidrs, tt.match, tt.types, tt.states)
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("CheckFlags() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// TestNewQuery_errors checks that invalid globs and several match modes are errors.
func TestNewQuery_errors(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    string
	}{
		{
			name: "two modes", filters: map[string][]string{FilterKeyMatch: {"exact", "longest"}},
			want: "--match takes one mode",
		},
		{name: "segment", filters: map[string][]string{FilterKeySegment: {"[x"}}, want: "segments"},
		{name: "function group", filters: map[string][]string{FilterKeyFunctionGroup: {"[x"}}, want: "function groups"},
		{name: "edge", filters: map[string][]string{FilterKeyEdge: {"[x"}}, want: "edges"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := newQuery(tt.filters); err == nil || !strings.HasPrefix(err.Error(), tt.want) {
				t.Errorf("newQuery(%v) error = %v, want prefix %q", tt.filters, err, tt.want)
			}
		})
	}
}

// TestTypeNames checks the route type, state, match and target type lists.
func TestTypeNames(t *testing.T) {
	tests := []struct {
		name string
		got  []string
		want string
	}{
		{name: "types", got: TypeNames(), want: "propagated"},
		{name: "states", got: StateNames(), want: "blackhole"},
		{name: "match", got: MatchNames(), want: MatchAny},
		{name: "targets", got: TargetTypeNames(), want: "direct-connect-gateway"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !slices.Contains(tt.got, tt.want) {
				t.Errorf("%s = %q, want it to contain %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestTagsToMap checks the conversion of tags, and that a tag without key is dropped.
func TestTagsToMap(t *testing.T) {
	if got := TagsToMap(nil); got != nil {
		t.Errorf("TagsToMap(nil) = %v, want nil", got)
	}
	got := TagsToMap([]types.Tag{{Key: aws.String("Name"), Value: aws.String("x")}, {Value: aws.String("y")}})
	if !reflect.DeepEqual(got, map[string]string{"Name": "x"}) {
		t.Errorf("TagsToMap() = %v, want map[Name:x]", got)
	}
	if got := TagName(nil); got != "" {
		t.Errorf("TagName(nil) = %q, want empty", got)
	}
}

// TestIsAccessDenied checks the AccessDenied detection.
func TestIsAccessDenied(t *testing.T) {
	if !isAccessDenied(fmt.Errorf("wrapped: %w", accessDenied())) {
		t.Error("isAccessDenied(AccessDeniedException) = false, want true")
	}
	if isAccessDenied(errors.New("boom")) || isAccessDenied(nil) {
		t.Error("isAccessDenied(other) = true, want false")
	}
}

// TestGetSortFields checks a valid and an invalid sort field.
func TestGetSortFields(t *testing.T) {
	if _, err := GetSortFields("cidr"); err != nil {
		t.Errorf("GetSortFields(cidr) error = %v, want nil", err)
	}
	if _, err := GetSortFields("nope"); err == nil {
		t.Error("GetSortFields(nope) error = nil, want an error")
	}
	if got := SortFieldNames(); !slices.Contains(got, "target-type") {
		t.Errorf("SortFieldNames() = %q, want target-type in it", got)
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("network", Region, nil, "cidr")
	r.Data = []dataRow{{CIDR: "10.0.0.0/8"}, {CIDR: "10.1.0.0/16"}}
	r.AddError("e")
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "network" || r.GetRegion() != Region || r.GetSortField() != "cidr" {
		t.Errorf("getters = %q %q %q, want network %s cidr", r.GetProfile(), r.GetRegion(), r.GetSortField(), Region)
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"e"}) {
		t.Errorf("GetErrors() = %q, want [e]", got)
	}
	if got := len(r.GetHeaders()); got != 12 {
		t.Errorf("len(GetHeaders()) = %d, want 12", got)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestResults_Search checks that Search searches through the client of the profile, and reports
// an invalid filter or a client that cannot be built.
func TestResults_Search(t *testing.T) {
	tests := []struct {
		name      string
		filters   map[string][]string
		clientErr error
		wantLen   int
		wantError string
	}{
		{name: "searches", filters: map[string][]string{FilterKeySegment: {"dc"}}, wantLen: 1},
		{name: "bad filter", filters: map[string][]string{FilterKeyCIDR: {"x"}}, wantError: "error building filters"},
		{
			name: "no client", filters: map[string][]string{}, clientErr: errors.New("no profile"),
			wantError: "error getting aws config: no profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakeFixture(map[string][][]types.NetworkRoute{"dc@eu-west-1": {{route("10.0.0.0/8")}}})
			old := newClient
			t.Cleanup(func() { newClient = old })
			newClient = func(string) (routesAPI, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return client, nil
			}
			r := New("network", Region, tt.filters, "")
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

// TestNewClient checks that a profile missing from the AWS config file is an error before any
// call, and that the client calls HomeRegion and tries a throttled call maxAttempts times.
func TestNewClient(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)
	t.Setenv("AWS_REGION", "eu-west-1")

	if _, err := NewClient("awss-test-missing-profile"); err == nil {
		t.Error("NewClient(missing profile) error = nil, want an error")
	}
	client, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient(default) error = %v, want nil", err)
	}
	if got := client.Options().Region; got != HomeRegion {
		t.Errorf("NewClient() Region = %q, want %q", got, HomeRegion)
	}
	if got := client.Options().Retryer.MaxAttempts(); got != maxAttempts {
		t.Errorf("NewClient() MaxAttempts = %d, want %d", got, maxAttempts)
	}
}
