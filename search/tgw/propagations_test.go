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

	"github.com/aws/smithy-go"
)

// propagationFixture returns attachmentFixture with propagations: the prod VPC propagates to dev
// and prod, the Direct Connect gateway to prod and, disabled, to dev; the VPN and the peering
// propagate nowhere.
func propagationFixture() *fakeEC2 {
	f := attachmentFixture()
	f.propagations = map[string][]string{
		"tgw-rtb-prod": {"tgw-attach-vpc1/enabled", "tgw-attach-dx1/enabled"},
		"tgw-rtb-dev":  {"tgw-attach-vpc1/enabled", "tgw-attach-dx1/disabled"},
	}
	return f
}

// collectPropagations runs collect with --propagations over f with the filters, sorted by ID.
func collectPropagations(t *testing.T, f *fakeEC2, filters map[string][]string) *AttachmentResults {
	t.Helper()
	q, err := newAttachmentQuery(filters)
	if err != nil {
		t.Fatalf("newAttachmentQuery(%v) error = %v", filters, err)
	}
	r := NewAttachments("network", "eu-central-1", filters, "id")
	r.Propagations = true
	r.collect(context.Background(), f, q)
	return r
}

// propagatedRTs returns attachment ID: Propagated RTs for each row.
func propagatedRTs(rows []attachmentRow) map[string][]string {
	out := map[string][]string{}
	for i := range rows {
		out[rows[i].ID] = rows[i].PropagatedRTs
	}
	return out
}

// TestAttachmentResults_collect_propagations checks the Propagated RTs of each attachment, read
// with one call per attachment when there are fewer attachments than route tables and one call per
// route table otherwise, and the attachments --propagated-route-table-ids keeps.
func TestAttachmentResults_collect_propagations(t *testing.T) {
	vpc1 := []string{"tgw-rtb-dev (dev)", "tgw-rtb-prod (prod)"}
	dx1 := []string{"tgw-rtb-dev (dev, disabled)", "tgw-rtb-prod (prod)"}
	tests := []struct {
		name                       string
		filters                    map[string][]string
		want                       map[string][]string
		wantAttCalls, wantRTBCalls int
	}{
		{
			name: "one attachment, one call", filters: map[string][]string{FilterKeyResourceID: {"vpc-0prod"}},
			want: map[string][]string{"tgw-attach-vpc1": vpc1}, wantAttCalls: 1,
		},
		{
			name: "every attachment, one call per route table", filters: map[string][]string{},
			want: map[string][]string{
				"tgw-attach-vpc1": vpc1, "tgw-attach-dx1": dx1, "tgw-attach-vpn1": nil, "tgw-attach-peer": nil,
				"tgw-attach-vpc9": nil,
			},
			wantRTBCalls: 2,
		},
		{
			name:    "propagated route table, in any state",
			filters: map[string][]string{FilterKeyPropagatedRouteTableID: {"tgw-rtb-dev"}},
			want:    map[string][]string{"tgw-attach-vpc1": vpc1, "tgw-attach-dx1": dx1}, wantRTBCalls: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := propagationFixture()
			r := collectPropagations(t, f, tt.filters)
			if len(r.Errors) > 0 {
				t.Fatalf("collect(%v) errors = %v", tt.filters, r.Errors)
			}
			if got := propagatedRTs(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) = %q, want %q", tt.filters, got, tt.want)
			}
			if f.attachmentPropagationCalls != tt.wantAttCalls || f.tablePropagationCalls != tt.wantRTBCalls {
				t.Errorf("collect(%v) calls = %d per attachment, %d per route table, want %d, %d", tt.filters,
					f.attachmentPropagationCalls, f.tablePropagationCalls, tt.wantAttCalls, tt.wantRTBCalls)
			}
		})
	}
}

// TestAttachmentResults_collect_propagationsSharedTGW checks that, with one call per attachment,
// the attachment to a transit gateway whose route tables the profile does not see (tgw-0b, shared
// by another account) is not read: AWS would refuse it and stop the other calls.
func TestAttachmentResults_collect_propagationsSharedTGW(t *testing.T) {
	f := propagationFixture()
	f.tables = append(f.tables, table("tgw-rtb-spare", "spare"))
	f.propagationsErr = map[string]error{
		"tgw-attach-vpc9": &smithy.GenericAPIError{Code: errCodeUnauthorized, Message: "no"},
	}
	filters := map[string][]string{FilterKeyAttachmentID: {"tgw-attach-vpc1", "tgw-attach-vpc9"}}
	r := collectPropagations(t, f, filters)
	want := map[string][]string{"tgw-attach-vpc1": {"tgw-rtb-dev (dev)", "tgw-rtb-prod (prod)"}, "tgw-attach-vpc9": nil}
	if got := propagatedRTs(r.Data); len(r.Errors) > 0 || !reflect.DeepEqual(got, want) {
		t.Errorf("collect(%v) = %q, errors %q, want %q and no error", filters, got, r.Errors, want)
	}
	if f.attachmentPropagationCalls != 1 || f.tablePropagationCalls != 0 {
		t.Errorf("collect(%v) calls = %d per attachment, %d per route table, want 1, 0", filters,
			f.attachmentPropagationCalls, f.tablePropagationCalls)
	}
}

// TestAttachmentResults_collect_propagationsJSON checks the propagations JSON keeps the ID, name
// and state of every route table.
func TestAttachmentResults_collect_propagationsJSON(t *testing.T) {
	r := collectPropagations(t, propagationFixture(), map[string][]string{FilterKeyAttachmentType: {"dx"}})
	want := []propagation{
		{RouteTableID: "tgw-rtb-dev", RouteTableName: "dev", State: "disabled"},
		{RouteTableID: "tgw-rtb-prod", RouteTableName: "prod", State: "enabled"},
	}
	if len(r.Data) != 1 || !reflect.DeepEqual(r.Data[0].Propagations, want) {
		t.Errorf("collect(dx) propagations = %+v, want %+v", r.Data, want)
	}
}

// TestAttachmentResults_collect_noPropagations checks that without --propagations nothing reads
// the propagations.
func TestAttachmentResults_collect_noPropagations(t *testing.T) {
	f := propagationFixture()
	r := collectAttachments(t, f, map[string][]string{}, "")
	if f.attachmentPropagationCalls+f.tablePropagationCalls != 0 {
		t.Errorf("collect() propagation calls = %d, want 0", f.attachmentPropagationCalls+f.tablePropagationCalls)
	}
	for i := range r.Data {
		if r.Data[i].PropagatedRTs != nil || r.Data[i].Propagations != nil {
			t.Errorf("collect() %s propagations = %q, want none", r.Data[i].ID, r.Data[i].PropagatedRTs)
		}
	}
}

// TestAttachmentResults_collect_propagationErrors checks that a failed propagation call is recorded
// and the next one is read, an unauthorized one stops them, and a failed route table call reads no
// propagations; the rows are kept unless the propagated route table filter needs them.
func TestAttachmentResults_collect_propagationErrors(t *testing.T) {
	boom := errors.New("boom")
	denied := &smithy.GenericAPIError{Code: errCodeUnauthorized, Message: "no"}
	tests := []struct {
		name         string
		change       func(*fakeEC2)
		filters      map[string][]string
		wantLen      int
		wantCalls    int
		wantErrors   []string
		wantPropagRT string
	}{
		{
			name:    "one route table fails",
			change:  func(f *fakeEC2) { f.propagationsErr = map[string]error{"tgw-rtb-prod": boom} },
			filters: map[string][]string{}, wantLen: 5, wantCalls: 2, wantPropagRT: "tgw-rtb-dev (dev)",
			wantErrors: []string{"reading the propagations into tgw-rtb-prod: boom"},
		},
		{
			name:    "unauthorized stops",
			change:  func(f *fakeEC2) { f.propagationsErr = map[string]error{"tgw-rtb-prod": denied} },
			filters: map[string][]string{}, wantLen: 5, wantCalls: 1,
			wantErrors: []string{"reading the propagations into tgw-rtb-prod: api error UnauthorizedOperation: no"},
		},
		{
			name:    "one attachment fails",
			change:  func(f *fakeEC2) { f.propagationsErr = map[string]error{"tgw-attach-vpc1": boom} },
			filters: map[string][]string{FilterKeyResourceID: {"vpc-0prod"}}, wantLen: 1, wantCalls: 1,
			wantErrors: []string{"reading the propagations of tgw-attach-vpc1: boom"},
		},
		{
			name: "route tables fail", change: func(f *fakeEC2) { f.tablesErr = boom },
			filters:    map[string][]string{FilterKeyPropagatedRouteTableID: {"tgw-rtb-dev"}},
			wantErrors: []string{"describing the transit gateway route tables: boom"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := propagationFixture()
			tt.change(f)
			r := collectPropagations(t, f, tt.filters)
			if r.Len() != tt.wantLen || !reflect.DeepEqual(r.Errors, tt.wantErrors) {
				t.Errorf("collect(%v) = %d rows, errors %q, want %d, %q",
					tt.filters, r.Len(), r.Errors, tt.wantLen, tt.wantErrors)
			}
			if got := f.attachmentPropagationCalls + f.tablePropagationCalls; got != tt.wantCalls {
				t.Errorf("collect(%v) propagation calls = %d, want %d", tt.filters, got, tt.wantCalls)
			}
			if tt.wantPropagRT == "" {
				return
			}
			if got := propagatedRTs(r.Data)["tgw-attach-vpc1"]; strings.Join(got, ",") != tt.wantPropagRT {
				t.Errorf("collect(%v) tgw-attach-vpc1 = %q, want %q", tt.filters, got, tt.wantPropagRT)
			}
		})
	}
}

// TestPropagation_cell checks the line of a propagation with and without a name and state.
func TestPropagation_cell(t *testing.T) {
	tests := []struct {
		name string
		p    propagation
		want string
	}{
		{
			name: "enabled with name", p: propagation{RouteTableID: "rtb-1", RouteTableName: "prod", State: "enabled"},
			want: "rtb-1 (prod)",
		},
		{name: "enabled without name", p: propagation{RouteTableID: "rtb-1", State: "enabled"}, want: "rtb-1"},
		{name: "disabled without name", p: propagation{RouteTableID: "rtb-1", State: "disabled"}, want: "rtb-1 (disabled)"},
		{
			name: "enabling with name", p: propagation{RouteTableID: "rtb-1", RouteTableName: "dev", State: "enabling"},
			want: "rtb-1 (dev, enabling)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.cell(); got != tt.want {
				t.Errorf("cell(%+v) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}
