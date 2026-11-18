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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// typedAttachment returns an attachment of the transit gateway tgw of the resource type and ID, in
// the state, associated with the route table tableID when it is not empty.
func typedAttachment(
	id, tgw string, typ types.TransitGatewayAttachmentResourceType, resource string,
	state types.TransitGatewayAttachmentState, tableID string,
) types.TransitGatewayAttachment {
	att := attachment(id, id+"-name", "333333333333")
	att.TransitGatewayId = aws.String(tgw)
	att.ResourceType = typ
	att.ResourceId = aws.String(resource)
	att.State = state
	if tableID != "" {
		att.Association = &types.TransitGatewayAttachmentAssociation{
			TransitGatewayRouteTableId: aws.String(tableID), State: types.TransitGatewayAssociationStateAssociated,
		}
	}
	return att
}

// attachmentFixture returns a fake with the route tables of fakeFixture and attachments of every
// kind: a VPC associated with prod, a Direct Connect gateway associated with dev, a VPN without
// association, a peering pending acceptance, and a VPC of another transit gateway associated with a
// route table the fake does not list.
func attachmentFixture() *fakeEC2 {
	f := fakeFixture()
	f.attachments = []types.TransitGatewayAttachment{
		typedAttachment("tgw-attach-vpc1", "tgw-0a", types.TransitGatewayAttachmentResourceTypeVpc, "vpc-0prod",
			types.TransitGatewayAttachmentStateAvailable, "tgw-rtb-prod"),
		typedAttachment("tgw-attach-dx1", "tgw-0a", types.TransitGatewayAttachmentResourceTypeDirectConnectGateway,
			"dxgw-1", types.TransitGatewayAttachmentStateAvailable, "tgw-rtb-dev"),
		typedAttachment("tgw-attach-vpn1", "tgw-0a", types.TransitGatewayAttachmentResourceTypeVpn, "vpn-1",
			types.TransitGatewayAttachmentStateAvailable, ""),
		typedAttachment("tgw-attach-peer", "tgw-0a", types.TransitGatewayAttachmentResourceTypePeering,
			"tgw-0peer", types.TransitGatewayAttachmentStatePendingAcceptance, ""),
		typedAttachment("tgw-attach-vpc9", "tgw-0b", types.TransitGatewayAttachmentResourceTypeVpc, "vpc-0other",
			types.TransitGatewayAttachmentStateAvailable, "tgw-rtb-other"),
	}
	return f
}

// collectAttachments runs collect over f with the filters and the sort field, and returns the
// results.
func collectAttachments(t *testing.T, f *fakeEC2, filters map[string][]string, sortField string) *AttachmentResults {
	t.Helper()
	q, err := newAttachmentQuery(filters)
	if err != nil {
		t.Fatalf("newAttachmentQuery(%v) error = %v", filters, err)
	}
	r := NewAttachments("network", "eu-central-1", filters, sortField)
	r.AccountNames = map[string]string{"333333333333": "prod-account"}
	r.collect(context.Background(), f, q)
	return r
}

// attachmentIDs returns the attachment ID of each row.
func attachmentIDs(rows []attachmentRow) []string {
	out := make([]string, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].ID)
	}
	return out
}

// TestAttachmentResults_collect checks the attachments each filter keeps, sorted by ID.
func TestAttachmentResults_collect(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		want    []string
	}{
		{
			name: "no filter lists every attachment", filters: map[string][]string{},
			want: []string{"tgw-attach-dx1", "tgw-attach-peer", "tgw-attach-vpc1", "tgw-attach-vpc9", "tgw-attach-vpn1"},
		},
		{
			name: "dx is short for direct-connect-gateway", filters: map[string][]string{FilterKeyAttachmentType: {"dx"}},
			want: []string{"tgw-attach-dx1"},
		},
		{
			name:    "associated route table",
			filters: map[string][]string{FilterKeyAssociatedRouteTableID: {"tgw-rtb-prod", "tgw-rtb-dev"}},
			want:    []string{"tgw-attach-dx1", "tgw-attach-vpc1"},
		},
		{
			name: "ids and resource", filters: map[string][]string{
				FilterKeyAttachmentID: {"tgw-attach-vpc1", "tgw-attach-vpc9"}, FilterKeyResourceID: {"vpc-0other"},
			},
			want: []string{"tgw-attach-vpc9"},
		},
		{
			name: "transit gateway", filters: map[string][]string{FilterKeyTransitGatewayID: {"tgw-0b"}},
			want: []string{"tgw-attach-vpc9"},
		},
		{
			name: "name glob", filters: map[string][]string{FilterKeyAttachmentName: {"tgw-attach-vpc*"}},
			want: []string{"tgw-attach-vpc1", "tgw-attach-vpc9"},
		},
		{
			name: "state in kebab case", filters: map[string][]string{FilterKeyAttachmentState: {"pending-acceptance"}},
			want: []string{"tgw-attach-peer"},
		},
		{
			name: "state in camel case", filters: map[string][]string{FilterKeyAttachmentState: {"pendingAcceptance"}},
			want: []string{"tgw-attach-peer"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := collectAttachments(t, attachmentFixture(), tt.filters, "id")
			if len(r.Errors) > 0 {
				t.Fatalf("collect(%v) errors = %v", tt.filters, r.Errors)
			}
			if got := attachmentIDs(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) = %q, want %q", tt.filters, got, tt.want)
			}
		})
	}
}

// TestAttachmentResults_collect_columns checks every column of an associated attachment, the
// association state of one still associating, and the empty columns of an attachment without
// association or whose route table the profile does not list.
func TestAttachmentResults_collect_columns(t *testing.T) {
	f := attachmentFixture()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("CET", 3600))
	f.attachments[0].CreationTime = &created
	f.attachments[0].TransitGatewayOwnerId = aws.String("222222222222")
	f.attachments[1].Association.State = types.TransitGatewayAssociationStateAssociating
	r := collectAttachments(t, f, map[string][]string{}, "")
	if len(r.Errors) > 0 {
		t.Fatalf("collect() errors = %v", r.Errors)
	}
	want := attachmentRow{
		ID: "tgw-attach-vpc1", Name: "tgw-attach-vpc1-name", TransitGatewayID: "tgw-0a", Type: "vpc",
		ResourceID: "vpc-0prod", State: "available", AssocRTID: "tgw-rtb-prod", AssocRTName: "prod",
		AssocState: "associated", OwnerID: "333333333333", OwnerName: "prod-account",
		TransitGatewayOwnerID: "222222222222", Created: "2026-01-02T02:04:05Z",
		Tags: map[string]string{"Name": "tgw-attach-vpc1-name"},
	}
	if len(r.Data) != 5 || !reflect.DeepEqual(r.Data[0], want) {
		t.Fatalf("collect()[0] = %+v, want %+v", r.Data, want)
	}
	tests := []struct {
		row                  int
		wantRT, wantName     string
		wantAssoc, wantState string
	}{
		{row: 1, wantRT: "tgw-rtb-dev", wantName: "dev", wantAssoc: "associating", wantState: "available"},
		{row: 2, wantState: "available"},
		{row: 3, wantState: "pending-acceptance"},
		{row: 4, wantRT: "tgw-rtb-other", wantAssoc: "associated", wantState: "available"},
	}
	for _, tt := range tests {
		got := &r.Data[tt.row]
		if got.AssocRTID != tt.wantRT || got.AssocRTName != tt.wantName || got.AssocState != tt.wantAssoc ||
			got.State != tt.wantState {
			t.Errorf("collect() %s = %q %q %q %q, want %q %q %q %q", got.ID, got.AssocRTID, got.AssocRTName,
				got.AssocState, got.State, tt.wantRT, tt.wantName, tt.wantAssoc, tt.wantState)
		}
	}
	if got, _ := filterValues(f.attachmentFilters[0], awsFilterAttTGWID); got != nil {
		t.Errorf("attachments transit-gateway-id = %q, want no filter", got)
	}
}

// TestAttachmentResults_collect_errors checks that a failed attachment call is recorded with no
// rows, a failed route table call keeps the rows without route table names, rows without
// association make no route table call, and a bad sort field is recorded.
func TestAttachmentResults_collect_errors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		change    func(*fakeEC2)
		filters   map[string][]string
		sortField string
		wantLen   int
		wantError string
	}{
		{
			name: "attachments", change: func(f *fakeEC2) { f.attachmentsErr = boom },
			wantError: "describing the transit gateway attachments: boom",
		},
		{
			name: "route tables", change: func(f *fakeEC2) { f.tablesErr = boom },
			wantLen: 5, wantError: "describing the transit gateway route tables: boom",
		},
		{
			name: "no association, no route table call", change: func(f *fakeEC2) { f.tablesErr = boom },
			filters: map[string][]string{FilterKeyAttachmentType: {"vpn"}}, wantLen: 1,
		},
		{name: "bad sort field", change: func(*fakeEC2) {}, sortField: "nope", wantLen: 5, wantError: "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := attachmentFixture()
			tt.change(f)
			filters := tt.filters
			if filters == nil {
				filters = map[string][]string{}
			}
			r := collectAttachments(t, f, filters, tt.sortField)
			if r.Len() != tt.wantLen {
				t.Errorf("collect() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			if got := strings.Join(r.Errors, ";"); !strings.Contains(got, tt.wantError) || (tt.wantError == "") != (got == "") {
				t.Errorf("collect() errors = %q, want %q", got, tt.wantError)
			}
			if tt.wantError == "describing the transit gateway route tables: boom" && r.Data[0].AssocRTName != "" {
				t.Errorf("collect() route table name = %q, want empty", r.Data[0].AssocRTName)
			}
		})
	}
}

// TestCheckAttachmentFlags checks the attachment types, states and name globs.
func TestCheckAttachmentFlags(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		wantErr string
	}{
		{
			name: "valid", filters: map[string][]string{
				FilterKeyAttachmentType: {"dx", "VPC"}, FilterKeyAttachmentState: {"pendingAcceptance", "available"},
				FilterKeyAttachmentName: {"prod-*"},
			},
		},
		{
			name: "bad type", filters: map[string][]string{FilterKeyAttachmentType: {"tgw"}},
			wantErr: "invalid attachment type",
		},
		{
			name: "bad state", filters: map[string][]string{FilterKeyAttachmentState: {"up"}},
			wantErr: "invalid attachment state",
		},
		{name: "bad glob", filters: map[string][]string{FilterKeyAttachmentName: {"["}}, wantErr: "attachment names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckAttachmentFlags(tt.filters)
			if got := ""; err != nil {
				got = err.Error()
				if tt.wantErr == "" || !strings.Contains(got, tt.wantErr) {
					t.Errorf("CheckAttachmentFlags(%v) = %q, want %q", tt.filters, got, tt.wantErr)
				}
			} else if tt.wantErr != "" {
				t.Errorf("CheckAttachmentFlags(%v) = nil, want %q", tt.filters, tt.wantErr)
			}
		})
	}
}

// TestCamelKebab checks camel case AWS values and values already in kebab case.
func TestCamelKebab(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{name: "camel", in: "pendingAcceptance", want: "pending-acceptance"},
		{name: "two words", in: "initiatingRequest", want: "initiating-request"},
		{name: "one word", in: "available", want: "available"},
		{name: "kebab", in: "rolling-back", want: "rolling-back"},
		{name: "leading capital", in: "Available", want: "available"},
		{name: "upper case", in: "AVAILABLE", want: "available"},
		{name: "title kebab", in: "Pending-Acceptance", want: "pending-acceptance"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := camelKebab(tt.in); got != tt.want {
				t.Errorf("camelKebab(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestAttachmentStateNames checks the states are in kebab case.
func TestAttachmentStateNames(t *testing.T) {
	got := AttachmentStateNames()
	for _, want := range []string{"available", "pending-acceptance", "rolling-back"} {
		if !strings.Contains(","+strings.Join(got, ",")+",", ","+want+",") {
			t.Errorf("AttachmentStateNames() = %q, want it to have %q", got, want)
		}
	}
}

// TestAttachmentResults_Search checks Search with a fake client, a bad filter and a client that
// cannot be built.
func TestAttachmentResults_Search(t *testing.T) {
	runSearchCases(t, attachmentFixture, 5, map[string][]string{FilterKeyAttachmentState: {"x"}},
		func(filters map[string][]string) common.Results {
			return NewAttachments("network", "eu-central-1", filters, "")
		})
}

// TestAttachmentResults_accessors checks Len, the BaseResults getters, the headers and the rows.
func TestAttachmentResults_accessors(t *testing.T) {
	r := NewAttachments("network", "eu-central-1", nil, "name")
	r.Data = []attachmentRow{{ID: "tgw-attach-1"}, {ID: "tgw-attach-2"}}
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "network" || r.GetRegion() != "eu-central-1" || r.GetSortField() != "name" {
		t.Errorf("getters = %q %q %q, want network eu-central-1 name", r.GetProfile(), r.GetRegion(), r.GetSortField())
	}
	wantHeaders := []interface{}{
		"Attachment ID\nName", "TGW ID", "Type", "Resource ID", "State", "Associated RT\nName", "Propagated RTs",
		"Owner ID\nName", "Tags",
	}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders() = %q, want %q", got, wantHeaders)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestGetAttachmentSortFields checks a valid and an invalid attachment sort field, and the list of
// names.
func TestGetAttachmentSortFields(t *testing.T) {
	got, err := GetAttachmentSortFields("associated-route-table")
	if err != nil || got["associated-route-table"] != "AssocRTName" {
		t.Errorf("GetAttachmentSortFields(associated-route-table) = %v, %v, want AssocRTName", got, err)
	}
	if _, err := GetAttachmentSortFields("nope"); err == nil {
		t.Error("GetAttachmentSortFields(nope) error = nil, want an error")
	}
	if names := AttachmentSortFieldNames(); len(names) == 0 || names[0] != "associated-route-table" {
		t.Errorf("AttachmentSortFieldNames() = %q, want associated-route-table first", names)
	}
}
