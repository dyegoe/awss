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

package cloudwanatt

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/search/cloudwan"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// fakeNetworkManager is a fake Network Manager client: the core networks in one page, and the
// attachments of each core network in pages. attErr fails the attachments of a core network.
type fakeNetworkManager struct {
	coreNetworks []types.CoreNetworkSummary
	listErr      error
	attachments  map[string][][]types.Attachment
	attErr       map[string]error
}

func (f *fakeNetworkManager) ListCoreNetworks(
	_ context.Context, _ *networkmanager.ListCoreNetworksInput, _ ...func(*networkmanager.Options),
) (*networkmanager.ListCoreNetworksOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &networkmanager.ListCoreNetworksOutput{CoreNetworks: f.coreNetworks}, nil
}

func (f *fakeNetworkManager) ListAttachments(
	_ context.Context, in *networkmanager.ListAttachmentsInput, _ ...func(*networkmanager.Options),
) (*networkmanager.ListAttachmentsOutput, error) {
	id := aws.ToString(in.CoreNetworkId)
	if err := f.attErr[id]; err != nil {
		return nil, err
	}
	pages := f.attachments[id]
	page, _ := strconv.Atoi(aws.ToString(in.NextToken))
	out := &networkmanager.ListAttachmentsOutput{}
	if page < len(pages) {
		out.Attachments = pages[page]
	}
	if page < len(pages)-1 {
		out.NextToken = aws.String(strconv.Itoa(page + 1))
	}
	return out, nil
}

// attachment returns an available attachment of type t, named name, in segment at edge.
func attachment(id, name string, t types.AttachmentType, segment, edge string) types.Attachment {
	return types.Attachment{
		AttachmentId:   aws.String(id),
		CoreNetworkId:  aws.String("cn-1"),
		AttachmentType: t,
		SegmentName:    aws.String(segment),
		EdgeLocation:   aws.String(edge),
		State:          types.AttachmentStateAvailable,
		OwnerAccountId: aws.String("111111111111"),
		Tags:           []types.Tag{{Key: aws.String("Name"), Value: aws.String(name)}},
	}
}

// fakeFixture returns a fake client with core networks cn-1 and cn-2: cn-1 has a VPC in prod and,
// on a second page, a Direct Connect gateway in dc spanning two edges; cn-2 has a VPN.
func fakeFixture() *fakeNetworkManager {
	dx := attachment("att-dx", "dc-link", types.AttachmentTypeDirectConnectGateway, "dc", "")
	dx.EdgeLocation = nil
	dx.EdgeLocations = []string{"eu-west-1", "us-east-1"}
	vpn := attachment("att-vpn", "office", types.AttachmentTypeSiteToSiteVpn, "dc", "eu-west-1")
	vpn.CoreNetworkId = aws.String("cn-2")
	return &fakeNetworkManager{
		coreNetworks: []types.CoreNetworkSummary{{CoreNetworkId: aws.String("cn-1")}, {CoreNetworkId: aws.String("cn-2")}},
		attachments: map[string][][]types.Attachment{
			"cn-1": {{attachment("att-vpc", "app", types.AttachmentTypeVpc, "prod", "eu-west-1")}, {dx}},
			"cn-2": {{vpn}},
		},
	}
}

// collectWith runs collect on a fresh result set with these filters and returns it.
func collectWith(t *testing.T, client attachmentsAPI, filters map[string][]string, sortField string) *Results {
	t.Helper()
	local, err := newLocalFilters(filters)
	if err != nil {
		t.Fatalf("newLocalFilters(%v) error = %v", filters, err)
	}
	r := New("network", cloudwan.Region, filters, sortField)
	r.collect(context.Background(), client, local)
	return r
}

// ids returns the IDs of the rows, in order.
func ids(rows []dataRow) []string {
	out := []string{}
	for i := range rows {
		out = append(out, rows[i].ID)
	}
	return out
}

// TestResults_collect_filters checks every filter, matched on the results of every page of every
// core network.
func TestResults_collect_filters(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{name: "all", filters: map[string][]string{}, want: []string{"att-dx", "att-vpc", "att-vpn"}},
		{name: "id", filters: map[string][]string{FilterKeyID: {"att-vpc"}}, want: []string{"att-vpc"}},
		{name: "name", filters: map[string][]string{FilterKeyName: {"dc-*"}}, want: []string{"att-dx"}},
		{
			name: "type", filters: map[string][]string{FilterKeyType: {"DIRECT_CONNECT_GATEWAY"}},
			want: []string{"att-dx"},
		},
		{name: "segment", filters: map[string][]string{FilterKeySegment: {"dc"}}, want: []string{"att-dx", "att-vpn"}},
		{
			name: "edge of a multi-edge gateway", filters: map[string][]string{FilterKeyEdge: {"us-*"}},
			want: []string{"att-dx"},
		},
		{name: "state", filters: map[string][]string{FilterKeyState: {"deleting"}}, want: []string{}},
		{name: "core network", filters: map[string][]string{FilterKeyCoreNetwork: {"cn-2"}}, want: []string{"att-vpn"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := collectWith(t, fakeFixture(), tt.filters, "id")
			if len(r.Errors) != 0 {
				t.Fatalf("collect(%v) errors = %q", tt.filters, r.Errors)
			}
			if got := ids(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) = %q, want %q", tt.filters, got, tt.want)
			}
		})
	}
}

// loadFixture decodes name, a sanitized AWS CLI output in the testdata of search/cloudwan, into v.
func loadFixture(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "cloudwan", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
}

// TestResults_collect_awsFixtures checks the attachments from the shapes AWS really returns: a
// Direct Connect gateway with EdgeLocations and no EdgeLocation, and a Transit Gateway route table
// attachment in no segment.
func TestResults_collect_awsFixtures(t *testing.T) {
	var list networkmanager.ListCoreNetworksOutput
	var atts networkmanager.ListAttachmentsOutput
	loadFixture(t, "list-core-networks.json", &list)
	loadFixture(t, "list-attachments.json", &atts)
	client := &fakeNetworkManager{
		coreNetworks: list.CoreNetworks,
		attachments:  map[string][][]types.Attachment{"core-network-0aaaaaaaaaaaaaaaa": {atts.Attachments}},
	}

	r := collectWith(t, client, map[string][]string{}, "")

	if len(r.Errors) != 0 {
		t.Fatalf("collect() errors = %q, want none", r.Errors)
	}
	got := []string{}
	for i := range r.Data {
		d := &r.Data[i]
		fields := []string{d.ID, d.Name, d.Type, d.Segment, strings.Join(d.Edges, ","), d.Resource}
		got = append(got, strings.Join(fields, " "))
	}
	want := []string{
		"attachment-0000000000000dev1 app-dev vpc dev eu-central-1 vpc-0000000000000dev1",
		"attachment-00000000000000dx1 dc-primary direct-connect-gateway sharedsvc eu-central-1 " +
			"00000000-0000-0000-0000-000000000dx1",
		"attachment-00000000000000tg1 legacy-shared transit-gateway-route-table sharedsvc eu-central-1 " +
			"tgw-rtb-000000000000000s1",
		"attachment-00000000000000tg2 legacy-unmapped transit-gateway-route-table  eu-central-1 " +
			"tgw-rtb-000000000000000n1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collect() =\n%q\nwant\n%q", got, want)
	}
}

// TestResults_collect_errors checks that a failing list stops the search and a failing core network
// is skipped.
func TestResults_collect_errors(t *testing.T) {
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
			name:     "one core network fails",
			change:   func(f *fakeNetworkManager) { f.attErr = map[string]error{"cn-1": errors.New("boom")} },
			wantRows: 1, wantError: "listing the attachments of core network cn-1: boom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakeFixture()
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

// TestResults_collect_badSortField checks that an unknown sort field is an error, not a panic.
func TestResults_collect_badSortField(t *testing.T) {
	r := collectWith(t, fakeFixture(), map[string][]string{}, "nope")
	if len(r.Errors) != 1 {
		t.Errorf("collect() errors = %q, want one sort error", r.Errors)
	}
}

// TestResults_parseAttachment checks the row of a fully filled attachment, of a function group
// attachment, and of an empty one.
func TestResults_parseAttachment(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))
	tests := []struct {
		name string
		att  types.Attachment
		want dataRow
	}{
		{
			name: "vpc",
			att: types.Attachment{
				AttachmentId: aws.String("att-1"), CoreNetworkId: aws.String("cn-1"), AttachmentType: types.AttachmentTypeVpc,
				SegmentName: aws.String("prod"), EdgeLocation: aws.String("eu-west-1"),
				State:          types.AttachmentStatePendingNetworkUpdate,
				ResourceArn:    aws.String("arn:aws:ec2:eu-west-1:222222222222:vpc/vpc-0abc"),
				OwnerAccountId: aws.String("222222222222"), AttachmentPolicyRuleNumber: aws.Int32(100), CreatedAt: &created,
				Tags: []types.Tag{{Key: aws.String("Name"), Value: aws.String("app")}},
			},
			want: dataRow{
				CoreNetworkID: "cn-1", ID: "att-1", Name: "app", Type: "vpc", Segment: "prod", Edges: []string{"eu-west-1"},
				State: "pending-network-update", Resource: "vpc-0abc",
				ResourceARN: "arn:aws:ec2:eu-west-1:222222222222:vpc/vpc-0abc",
				OwnerID:     "222222222222", OwnerName: "app-prod", PolicyRule: "100", Created: "2026-01-02T02:04:05Z",
				Tags: map[string]string{"Name": "app"},
			},
		},
		{
			name: "function group",
			att:  types.Attachment{AttachmentId: aws.String("att-2"), NetworkFunctionGroupName: aws.String("inspection")},
			want: dataRow{ID: "att-2", Segment: "nfg:inspection"},
		},
		{name: "empty", att: types.Attachment{}, want: dataRow{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("network", cloudwan.Region, nil, "")
			r.AccountNames = map[string]string{"222222222222": "app-prod"}
			if got := r.parseAttachment(&tt.att); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAttachment() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestResourceID checks the resource ID of ARNs with and without a slash.
func TestResourceID(t *testing.T) {
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{name: "slash", arn: "arn:aws:directconnect::111111111111:dx-gateway/abc-123", want: "abc-123"},
		{name: "colon", arn: "arn:aws:ec2:eu-west-1:111111111111:thing:x-1", want: "x-1"},
		{name: "plain", arn: "x", want: "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceID(tt.arn); got != tt.want {
				t.Errorf("resourceID(%q) = %q, want %q", tt.arn, got, tt.want)
			}
		})
	}
}

// TestCheckFlags checks the validation of the flags before any AWS call.
func TestCheckFlags(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		wantErr string
	}{
		{name: "valid", filters: map[string][]string{FilterKeyType: {"vpc"}, FilterKeyState: {"available"}}},
		{name: "bad type", filters: map[string][]string{FilterKeyType: {"tgw"}}, wantErr: "invalid attachment type"},
		{name: "bad state", filters: map[string][]string{FilterKeyState: {"up"}}, wantErr: "invalid attachment state"},
		{name: "bad name", filters: map[string][]string{FilterKeyName: {"[x"}}, wantErr: "names"},
		{name: "bad segment", filters: map[string][]string{FilterKeySegment: {"[x"}}, wantErr: "segments"},
		{name: "bad edge", filters: map[string][]string{FilterKeyEdge: {"[x"}}, wantErr: "edges"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckFlags(tt.filters)
			failed := err == nil || !strings.HasPrefix(err.Error(), tt.wantErr)
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && failed {
				t.Errorf("CheckFlags(%v) error = %v, want %q", tt.filters, err, tt.wantErr)
			}
		})
	}
}

// TestTypeNames checks the attachment type and state lists.
func TestTypeNames(t *testing.T) {
	if !slices.Contains(TypeNames(), "direct-connect-gateway") || !slices.Contains(StateNames(), "available") {
		t.Errorf("TypeNames(), StateNames() = %q, %q", TypeNames(), StateNames())
	}
}

// TestGetSortFields checks a valid and an invalid sort field.
func TestGetSortFields(t *testing.T) {
	if _, err := GetSortFields("segment"); err != nil {
		t.Errorf("GetSortFields(segment) error = %v, want nil", err)
	}
	if _, err := GetSortFields("nope"); err == nil {
		t.Error("GetSortFields(nope) error = nil, want an error")
	}
	if got := SortFieldNames(); !slices.Contains(got, "edge") {
		t.Errorf("SortFieldNames() = %q, want edge in it", got)
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("network", cloudwan.Region, nil, "id")
	r.Data = []dataRow{{ID: "1"}, {ID: "2"}}
	r.AddError("e")
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "network" || r.GetRegion() != cloudwan.Region || r.GetSortField() != "id" {
		t.Errorf("getters = %q %q %q, want network %s id", r.GetProfile(), r.GetRegion(), r.GetSortField(), cloudwan.Region)
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"e"}) {
		t.Errorf("GetErrors() = %q, want [e]", got)
	}
	if got := len(r.GetHeaders()); got != 8 {
		t.Errorf("len(GetHeaders()) = %d, want 8", got)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestResults_Search checks that Search lists through the client of the profile, and reports an
// invalid filter or a client that cannot be built.
func TestResults_Search(t *testing.T) {
	tests := []struct {
		name      string
		filters   map[string][]string
		clientErr error
		wantLen   int
		wantError string
	}{
		{name: "lists", filters: map[string][]string{}, wantLen: 3},
		{name: "bad filter", filters: map[string][]string{FilterKeyType: {"x"}}, wantError: "error building filters"},
		{
			name: "no client", filters: map[string][]string{}, clientErr: errors.New("no profile"),
			wantError: "error getting aws config: no profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := newClient
			t.Cleanup(func() { newClient = old })
			newClient = func(string) (attachmentsAPI, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return fakeFixture(), nil
			}
			r := New("network", cloudwan.Region, tt.filters, "")
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
