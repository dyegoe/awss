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
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/directconnect"
	"github.com/aws/aws-sdk-go-v2/service/directconnect/types"
)

// fakeDX is a Direct Connect client that answers from fixed gateways, associations and
// attachments. The gateways come in pages of pageLen; the per-gateway answers are keyed by gateway
// ID.
type fakeDX struct {
	gateways     []types.DirectConnectGateway
	pageLen      int
	sameToken    bool
	associations map[string][]types.DirectConnectGatewayAssociation
	attachments  map[string][]types.DirectConnectGatewayAttachment

	gatewaysErr     error
	associationsErr map[string]error
	attachmentsErr  map[string]error

	// maxResults are the page sizes the calls asked for.
	maxResults []int32
}

func (f *fakeDX) DescribeDirectConnectGateways(
	_ context.Context, in *directconnect.DescribeDirectConnectGatewaysInput, _ ...func(*directconnect.Options),
) (*directconnect.DescribeDirectConnectGatewaysOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	if f.gatewaysErr != nil {
		return nil, f.gatewaysErr
	}
	n := f.pageLen
	if n == 0 {
		n = len(f.gateways)
	}
	start := 0
	if in.NextToken != nil {
		start = len(*in.NextToken)
	}
	end := min(start+n, len(f.gateways))
	out := &directconnect.DescribeDirectConnectGatewaysOutput{DirectConnectGateways: f.gateways[start:end]}
	switch {
	case f.sameToken:
		out.NextToken = aws.String("x")
	case end < len(f.gateways):
		out.NextToken = aws.String(strings.Repeat("x", end))
	}
	return out, nil
}

func (f *fakeDX) DescribeDirectConnectGatewayAssociations(
	_ context.Context, in *directconnect.DescribeDirectConnectGatewayAssociationsInput,
	_ ...func(*directconnect.Options),
) (*directconnect.DescribeDirectConnectGatewayAssociationsOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	id := aws.ToString(in.DirectConnectGatewayId)
	if err := f.associationsErr[id]; err != nil {
		return nil, err
	}
	return &directconnect.DescribeDirectConnectGatewayAssociationsOutput{
		DirectConnectGatewayAssociations: f.associations[id],
	}, nil
}

func (f *fakeDX) DescribeDirectConnectGatewayAttachments(
	_ context.Context, in *directconnect.DescribeDirectConnectGatewayAttachmentsInput,
	_ ...func(*directconnect.Options),
) (*directconnect.DescribeDirectConnectGatewayAttachmentsOutput, error) {
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	id := aws.ToString(in.DirectConnectGatewayId)
	if err := f.attachmentsErr[id]; err != nil {
		return nil, err
	}
	return &directconnect.DescribeDirectConnectGatewayAttachmentsOutput{
		DirectConnectGatewayAttachments: f.attachments[id],
	}, nil
}

// gateway returns a Direct Connect gateway.
func gateway(id, name string, asn int64) types.DirectConnectGateway {
	return types.DirectConnectGateway{
		DirectConnectGatewayId: aws.String(id), DirectConnectGatewayName: aws.String(name),
		AmazonSideAsn: aws.Int64(asn), DirectConnectGatewayState: types.DirectConnectGatewayStateAvailable,
		OwnerAccount: aws.String("111111111111"),
	}
}

// prefixes returns allowed prefixes.
func prefixes(cidrs ...string) []types.RouteFilterPrefix {
	out := make([]types.RouteFilterPrefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, types.RouteFilterPrefix{Cidr: aws.String(c)})
	}
	return out
}

// vif returns a virtual interface attachment.
func vif(id string) types.DirectConnectGatewayAttachment {
	return types.DirectConnectGatewayAttachment{
		VirtualInterfaceId: aws.String(id), VirtualInterfaceRegion: aws.String("eu-central-1"),
		VirtualInterfaceOwnerAccount: aws.String("111111111111"),
		AttachmentState:              types.DirectConnectGatewayAttachmentStateAttached,
	}
}

// fakeFixture returns three gateways, made up: gw-cwan is associated with a core network and
// has two VIFs; gw-legacy is associated with a transit gateway that allows three prefixes, and
// with an old virtual private gateway; gw-spare has no association.
func fakeFixture() *fakeDX {
	return &fakeDX{
		gateways: []types.DirectConnectGateway{
			gateway("gw-cwan", "dc-cwan", 64512), gateway("gw-legacy", "dc-legacy", 64513),
			gateway("gw-spare", "spare", 64514),
		},
		associations: map[string][]types.DirectConnectGatewayAssociation{
			"gw-cwan": {{
				AssociationId: aws.String("as-cwan"), AssociationState: types.DirectConnectGatewayAssociationStateAssociated,
				AssociatedCoreNetwork: &types.AssociatedCoreNetwork{
					Id: aws.String("core-network-0aaa"), OwnerAccount: aws.String("111111111111"),
					AttachmentId: aws.String("attachment-0dx1"),
				},
			}},
			"gw-legacy": {
				{
					AssociationId:    aws.String("as-tgw"),
					AssociationState: types.DirectConnectGatewayAssociationStateAssociated,
					AssociatedGateway: &types.AssociatedGateway{
						Id: aws.String("tgw-0leg"), Type: types.GatewayTypeTransitGateway,
						OwnerAccount: aws.String("222222222222"), Region: aws.String("eu-central-1"),
					},
					AllowedPrefixesToDirectConnectGateway: prefixes("10.20.8.0/21", "10.20.0.0/16", "10.2.0.0/16"),
				},
				{
					AssociationId:    aws.String("as-vgw"),
					AssociationState: types.DirectConnectGatewayAssociationStateDisassociated,
					VirtualGatewayId: aws.String("vgw-0old"), VirtualGatewayOwnerAccount: aws.String("333333333333"),
					AllowedPrefixesToDirectConnectGateway: prefixes("172.16.0.0/24"),
				},
			},
		},
		attachments: map[string][]types.DirectConnectGatewayAttachment{
			"gw-cwan": {vif("dxvif-0a"), vif("dxvif-0b")},
		},
	}
}

// collectWith runs collect with client, filters and sortField, and fails on bad filters.
func collectWith(t *testing.T, client gatewaysAPI, filters map[string][]string, sortField string) *Results {
	t.Helper()
	q, err := newQuery(filters)
	if err != nil {
		t.Fatalf("newQuery(%v) error = %v", filters, err)
	}
	r := New("network", Region, filters, sortField)
	r.AccountNames = map[string]string{"222222222222": "legacy-net"}
	r.collect(context.Background(), client, q)
	return r
}

// keys returns gateway|associated|prefix|query of each row.
func keys(rows []dataRow) []string {
	out := make([]string, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, strings.Join([]string{r.GatewayName, r.AssociatedID, string(r.AllowedPrefix), r.Query}, "|"))
	}
	return out
}

// TestResults_collect_filters checks the rows each filter keeps, in the default gateway order.
func TestResults_collect_filters(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{
			name: "no filter lists every association and prefix", filters: map[string][]string{},
			want: []string{
				"dc-cwan|core-network-0aaa||", "dc-legacy|tgw-0leg|10.2.0.0/16|", "dc-legacy|tgw-0leg|10.20.0.0/16|",
				"dc-legacy|tgw-0leg|10.20.8.0/21|", "dc-legacy|vgw-0old|172.16.0.0/24|", "spare|||",
			},
		},
		{
			name:    "an address is carried by the prefixes covering it",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.9.5"}, FilterKeyMatch: {"supernet-of"}},
			want:    []string{"dc-legacy|tgw-0leg|10.20.0.0/16|10.20.9.5/32", "dc-legacy|tgw-0leg|10.20.8.0/21|10.20.9.5/32"},
		},
		{
			name:    "subnet-of keeps the prefixes inside",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.0.0/16"}, FilterKeyMatch: {"subnet-of"}},
			want:    []string{"dc-legacy|tgw-0leg|10.20.0.0/16|10.20.0.0/16", "dc-legacy|tgw-0leg|10.20.8.0/21|10.20.0.0/16"},
		},
		{
			name:    "exact",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.8.0/21"}, FilterKeyMatch: {"exact"}},
			want:    []string{"dc-legacy|tgw-0leg|10.20.8.0/21|10.20.8.0/21"},
		},
		{
			name:    "any is both, for each CIDR",
			filters: map[string][]string{FilterKeyCIDR: {"10.20.8.0/22", "172.16.0.0/16"}},
			want: []string{
				"dc-legacy|tgw-0leg|10.20.0.0/16|10.20.8.0/22", "dc-legacy|tgw-0leg|10.20.8.0/21|10.20.8.0/22",
				"dc-legacy|vgw-0old|172.16.0.0/24|172.16.0.0/16",
			},
		},
		{name: "core network", filters: map[string][]string{FilterKeyAssociatedType: {"core-network"}}, want: []string{
			"dc-cwan|core-network-0aaa||",
		}},
		{
			name:    "associated id and state",
			filters: map[string][]string{FilterKeyAssociatedID: {"vgw-0old", "tgw-0leg"}, FilterKeyState: {"disassociated"}},
			want:    []string{"dc-legacy|vgw-0old|172.16.0.0/24|"},
		},
		{name: "id and name", filters: map[string][]string{
			FilterKeyID: {"gw-spare", "gw-cwan"}, FilterKeyName: {"sp*"},
		}, want: []string{"spare|||"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := collectWith(t, fakeFixture(), tt.filters, "gateway")
			if len(r.Errors) > 0 {
				t.Fatalf("collect(%v) errors = %v", tt.filters, r.Errors)
			}
			if got := keys(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) = %q, want %q", tt.filters, got, tt.want)
			}
		})
	}
}

// TestResults_collect_columns checks the columns of a transit gateway row and of a core network
// row, with the VIFs of the gateway and the owner name.
func TestResults_collect_columns(t *testing.T) {
	r := collectWith(t, fakeFixture(), map[string][]string{FilterKeyCIDR: {"10.2.0.0/16"}}, "")
	want := dataRow{
		Query: "10.2.0.0/16", GatewayID: "gw-legacy", GatewayName: "dc-legacy", AmazonASN: "64513",
		GatewayState: "available", GatewayOwnerID: "111111111111", AssociatedType: "transit-gateway",
		AssociatedID: "tgw-0leg", AssociatedRegion: "eu-central-1", OwnerID: "222222222222",
		OwnerName: "legacy-net", State: "associated", AllowedPrefix: "10.2.0.0/16", AssociationID: "as-tgw",
	}
	if len(r.Data) != 1 || !reflect.DeepEqual(r.Data[0], want) {
		t.Errorf("collect(10.2.0.0/16) = %+v, want [%+v]", r.Data, want)
	}

	r = collectWith(t, fakeFixture(), map[string][]string{FilterKeyID: {"gw-cwan"}}, "")
	if len(r.Data) != 1 {
		t.Fatalf("collect(gw-cwan) rows = %d, want 1", len(r.Data))
	}
	got := r.Data[0]
	if got.CoreNetworkAttachmentID != "attachment-0dx1" || got.AssociatedRegion != "" ||
		!reflect.DeepEqual(got.VIFs, []string{"dxvif-0a", "dxvif-0b"}) {
		t.Errorf("collect(gw-cwan) = %+v, want attachment-0dx1, no region, VIFs dxvif-0a dxvif-0b", got)
	}
	wantVIF := vifAttachment{ID: "dxvif-0a", Region: "eu-central-1", OwnerID: "111111111111", State: "attached"}
	if len(got.VIFAttachments) != 2 || got.VIFAttachments[0] != wantVIF {
		t.Errorf("collect(gw-cwan) VIFAttachments = %+v, want first %+v", got.VIFAttachments, wantVIF)
	}
}

// TestResults_collect_errors checks that a failed list is recorded, and that a gateway that fails
// does not stop the others.
func TestResults_collect_errors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		change    func(*fakeDX)
		sortField string
		wantLen   int
		wantError string
	}{
		{
			name: "gateways", change: func(f *fakeDX) { f.gatewaysErr = boom },
			wantError: "listing the Direct Connect gateways: boom",
		},
		{
			name:    "associations of one gateway",
			change:  func(f *fakeDX) { f.associationsErr = map[string]error{"gw-legacy": boom} },
			wantLen: 2, wantError: "listing the associations of Direct Connect gateway gw-legacy: boom",
		},
		{
			name:    "virtual interfaces of one gateway",
			change:  func(f *fakeDX) { f.attachmentsErr = map[string]error{"gw-cwan": boom} },
			wantLen: 5, wantError: "listing the virtual interfaces of Direct Connect gateway gw-cwan: boom",
		},
		{name: "bad sort field", change: func(*fakeDX) {}, sortField: "nope", wantLen: 6, wantError: "nope"},
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

// TestGateways_pages checks that every page of gateways is read with the page size, and that a
// page returning the token it was given ends the list.
func TestGateways_pages(t *testing.T) {
	tests := []struct {
		name      string
		sameToken bool
		want      int
		wantCalls int
	}{
		{name: "three pages", want: 3, wantCalls: 3},
		{name: "same token twice", sameToken: true, want: 2, wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fakeFixture()
			f.pageLen, f.sameToken = 1, tt.sameToken
			gws, err := gateways(context.Background(), f)
			if err != nil {
				t.Fatalf("gateways() error = %v", err)
			}
			if len(gws) != tt.want || len(f.maxResults) != tt.wantCalls {
				t.Errorf("gateways() = %d gateways in %d calls, want %d in %d", len(gws), len(f.maxResults), tt.want, tt.wantCalls)
			}
			for _, m := range f.maxResults {
				if m != pageSize {
					t.Errorf("gateways() MaxResults = %d, want %d", m, pageSize)
				}
			}
		})
	}
}

// TestResults_parseAssociation checks the associated columns of each kind of association,
// including one with no associated gateway at all.
func TestResults_parseAssociation(t *testing.T) {
	tests := []struct {
		name     string
		a        types.DirectConnectGatewayAssociation
		wantType string
		wantID   string
	}{
		{
			name: "virtual private gateway",
			a: types.DirectConnectGatewayAssociation{AssociatedGateway: &types.AssociatedGateway{
				Id: aws.String("vgw-1"), Type: types.GatewayTypeVirtualPrivateGateway,
			}},
			wantType: "virtual-private-gateway", wantID: "vgw-1",
		},
		{name: "nothing", a: types.DirectConnectGatewayAssociation{StateChangeError: aws.String("e")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("network", Region, nil, "")
			got := r.parseAssociation(&dataRow{GatewayID: "gw"}, &tt.a)
			if got.AssociatedType != tt.wantType || got.AssociatedID != tt.wantID || got.GatewayID != "gw" {
				t.Errorf("parseAssociation() = %q %q %q, want gw %q %q",
					got.GatewayID, got.AssociatedType, got.AssociatedID, tt.wantType, tt.wantID)
			}
		})
	}
}

// TestResults_parseGateway checks that a gateway without an Amazon ASN has an empty ASN column.
func TestResults_parseGateway(t *testing.T) {
	r := New("network", Region, nil, "")
	got := r.parseGateway(&types.DirectConnectGateway{DirectConnectGatewayId: aws.String("gw")})
	if want := (dataRow{GatewayID: "gw"}); !reflect.DeepEqual(got, want) {
		t.Errorf("parseGateway() = %+v, want %+v", got, want)
	}
}

// TestGatewayType checks the AWS gateway types in kebab case.
func TestGatewayType(t *testing.T) {
	tests := []struct {
		name string
		in   types.GatewayType
		want string
	}{
		{name: "transit", in: types.GatewayTypeTransitGateway, want: AssociatedTransitGateway},
		{name: "virtual private", in: types.GatewayTypeVirtualPrivateGateway, want: AssociatedVirtualPrivateGateway},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gatewayType(tt.in); got != tt.want {
				t.Errorf("gatewayType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestMatches checks each match mode, IPv6, a prefix with host bits and a prefix that does not
// parse.
func TestMatches(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		cidr   string
		mode   string
		want   bool
	}{
		{name: "exact", prefix: "10.0.0.0/16", cidr: "10.0.0.0/16", mode: "exact", want: true},
		{name: "exact host bits", prefix: "10.0.1.0/16", cidr: "10.0.0.0/16", mode: "exact", want: true},
		{name: "not exact", prefix: "10.0.0.0/16", cidr: "10.0.0.0/17", mode: "exact"},
		{name: "subnet", prefix: "10.0.8.0/21", cidr: "10.0.0.0/16", mode: "subnet-of", want: true},
		{name: "not subnet", prefix: "10.0.0.0/8", cidr: "10.0.0.0/16", mode: "subnet-of"},
		{name: "supernet", prefix: "10.0.0.0/8", cidr: "10.0.0.0/16", mode: "supernet-of", want: true},
		{name: "not supernet", prefix: "10.1.0.0/16", cidr: "10.0.0.0/16", mode: "supernet-of"},
		{name: "any", prefix: "10.0.0.0/8", cidr: "10.0.0.0/16", mode: "any", want: true},
		{name: "disjoint", prefix: "192.168.0.0/16", cidr: "10.0.0.0/16", mode: "any"},
		{name: "ipv6", prefix: "2001:db8::/32", cidr: "2001:db8:1::/48", mode: "supernet-of", want: true},
		{name: "families differ", prefix: "10.0.0.0/8", cidr: "2001:db8::/32", mode: "any"},
		{name: "bad prefix", prefix: "x", cidr: "10.0.0.0/16", mode: "any"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matches(tt.prefix, netip.MustParsePrefix(tt.cidr), tt.mode); got != tt.want {
				t.Errorf("matches(%q, %q, %q) = %v, want %v", tt.prefix, tt.cidr, tt.mode, got, tt.want)
			}
		})
	}
}

// TestCheckFlags checks the values rejected before any AWS call.
func TestCheckFlags(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		wantErr string
	}{
		{name: "valid", filters: map[string][]string{
			FilterKeyCIDR: {"10.0.0.1"}, FilterKeyMatch: {"exact"}, FilterKeyAssociatedType: {"Transit-Gateway"},
			FilterKeyState: {"associated"}, FilterKeyName: {"dc-*"},
		}},
		{name: "empty match", filters: map[string][]string{FilterKeyMatch: {""}}},
		{name: "bad cidr", filters: map[string][]string{FilterKeyCIDR: {"10.0.0.0/33"}}, wantErr: "invalid CIDR"},
		{name: "longest", filters: map[string][]string{FilterKeyMatch: {"longest"}}, wantErr: "invalid match"},
		{name: "two modes", filters: map[string][]string{FilterKeyMatch: {"exact", "any"}}, wantErr: "one mode"},
		{name: "bad type", filters: map[string][]string{FilterKeyAssociatedType: {"vgw"}}, wantErr: "associated type"},
		{name: "bad state", filters: map[string][]string{FilterKeyState: {"up"}}, wantErr: "association state"},
		{name: "bad name", filters: map[string][]string{FilterKeyName: {"["}}, wantErr: "names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckFlags(tt.filters)
			if (err == nil) != (tt.wantErr == "") || (err != nil && !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("CheckFlags(%v) error = %v, want %q", tt.filters, err, tt.wantErr)
			}
		})
	}
}

// TestResults_sortRows checks that rows equal on the sort field stay grouped by gateway and
// association, with the prefixes in address order.
func TestResults_sortRows(t *testing.T) {
	r := collectWith(t, fakeFixture(), map[string][]string{}, "state")
	want := []string{
		"dc-cwan|core-network-0aaa||", "dc-legacy|tgw-0leg|10.2.0.0/16|", "dc-legacy|tgw-0leg|10.20.0.0/16|",
		"dc-legacy|tgw-0leg|10.20.8.0/21|", "dc-legacy|vgw-0old|172.16.0.0/24|", "spare|||",
	}
	// state sorts empty first, then associated, then disassociated.
	want = []string{want[5], want[0], want[1], want[2], want[3], want[4]}
	if got := keys(r.Data); !reflect.DeepEqual(got, want) {
		t.Errorf("collect() sorted by state = %q, want %q", got, want)
	}
}

// TestAssociatedTypeNames checks the fixed lists of the flags: the associated types, the match
// modes and the states.
func TestAssociatedTypeNames(t *testing.T) {
	tests := []struct {
		name string
		got  []string
		want string
	}{
		{name: "associated types", got: AssociatedTypeNames(), want: "core-network transit-gateway virtual-private-gateway"},
		{name: "match", got: MatchNames(), want: "any exact subnet-of supernet-of"},
		{name: "states", got: StateNames(), want: "associating associated disassociating disassociated updating"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(tt.got, " "); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestGetSortFields checks a valid and an invalid sort field.
func TestGetSortFields(t *testing.T) {
	if got, err := GetSortFields("prefix"); err != nil || got["prefix"] != "AllowedPrefix" {
		t.Errorf("GetSortFields(prefix) = %v, %v, want AllowedPrefix", got, err)
	}
	if _, err := GetSortFields("nope"); err == nil {
		t.Error("GetSortFields(nope) error = nil, want an error")
	}
	if got := SortFieldNames(); len(got) != 11 {
		t.Errorf("SortFieldNames() = %v, want 11 fields", got)
	}
}

// TestResults_accessors checks Len, the BaseResults getters, the headers and the rows.
func TestResults_accessors(t *testing.T) {
	r := New("network", Region, nil, "gateway")
	r.Data = []dataRow{{GatewayID: "1"}, {GatewayID: "2"}}
	r.AddError("e")
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "network" || r.GetRegion() != Region || r.GetSortField() != "gateway" {
		t.Errorf("getters = %q %q %q, want network %s gateway", r.GetProfile(), r.GetRegion(), r.GetSortField(), Region)
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"e"}) {
		t.Errorf("GetErrors() = %q, want [e]", got)
	}
	if got := len(r.GetHeaders()); got != 10 {
		t.Errorf("len(GetHeaders()) = %d, want 10", got)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestResults_Search checks Search with a fake client, bad filters and a client that cannot be
// built.
func TestResults_Search(t *testing.T) {
	tests := []struct {
		name      string
		filters   map[string][]string
		clientErr error
		wantLen   int
		wantError string
	}{
		{name: "lists", filters: map[string][]string{}, wantLen: 6},
		{name: "bad filter", filters: map[string][]string{FilterKeyState: {"x"}}, wantError: "error building filters"},
		{
			name: "no client", filters: map[string][]string{}, clientErr: errors.New("no profile"),
			wantError: "error getting aws config: no profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := newClient
			t.Cleanup(func() { newClient = old })
			newClient = func(string) (gatewaysAPI, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return fakeFixture(), nil
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

// TestNewClient checks the region of the client: the region of the profile, or us-east-1 when it
// names none. Loading the config makes no AWS call.
func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{name: "profile region", profile: "with", want: "eu-central-1"},
		{name: "fallback", profile: "without", want: fallbackRegion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config")
			content := "[profile with]\nregion = eu-central-1\n[profile without]\noutput = json\n"
			if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			t.Setenv("AWS_CONFIG_FILE", file)
			t.Setenv("AWS_REGION", "")
			t.Setenv("AWS_DEFAULT_REGION", "")
			c, err := NewClient(tt.profile)
			if err != nil {
				t.Fatalf("NewClient(%q) error = %v", tt.profile, err)
			}
			if got := c.Options().Region; got != tt.want {
				t.Errorf("NewClient(%q) region = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}
