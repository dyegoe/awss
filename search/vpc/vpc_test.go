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

package vpc

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var mockDataRow1 = dataRow{
	VpcID:         "vpc-0000000000000000a",
	Name:          "prod",
	CidrBlock:     "10.0.0.0/16",
	CidrBlocks:    []string{"10.0.0.0/16", "10.1.0.0/16"},
	State:         "available",
	IsDefault:     "false",
	OwnerID:       "123456789012",
	DhcpOptionsID: "dopt-1",
	Tags:          map[string]string{"Name": "prod", "Environment": "prod"},
}

var mockDataRow2 = dataRow{
	VpcID:      "vpc-0000000000000000b",
	Name:       "dev",
	CidrBlock:  "172.16.0.0/16",
	CidrBlocks: []string{"172.16.0.0/16"},
	State:      "available",
	IsDefault:  "true",
	OwnerID:    "123456789012",
	Tags:       map[string]string{"Name": "dev"},
}

func mockResults() *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   "default",
			Region:    "us-east-1",
			Errors:    []string{},
			SortField: "id",
		},
		Data: []dataRow{mockDataRow1, mockDataRow2},
		Filters: map[string][]string{
			"vpc-id":   {"vpc-0000000000000000a"},
			"tag:Name": {"prod", "dev"},
			"tag":      {"Environment=prod:dev"},
			"state":    {"available"},
		},
	}
}

// TestNew tests the New function.
func TestNew(t *testing.T) {
	filters := map[string][]string{"vpc-id": {"vpc-1"}}
	want := &Results{
		BaseResults: common.BaseResults{
			Profile:   "default",
			Region:    "us-east-1",
			Errors:    []string{},
			SortField: "id",
		},
		Data:    []dataRow{},
		Filters: filters,
	}
	if got := New("default", "us-east-1", filters, "id"); !reflect.DeepEqual(got, want) {
		t.Errorf("New()\n%#v\nwant\n%#v", got, want)
	}
}

// TestResults_accessors tests Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := mockResults()
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if got := r.GetProfile(); got != "default" {
		t.Errorf("GetProfile() = %q, want default", got)
	}
	if got := r.GetRegion(); got != "us-east-1" {
		t.Errorf("GetRegion() = %q, want us-east-1", got)
	}
	if got := r.GetSortField(); got != "id" {
		t.Errorf("GetSortField() = %q, want id", got)
	}
	if got := r.GetErrors(); len(got) != 0 {
		t.Errorf("GetErrors() = %v, want empty", got)
	}
}

// TestResults_GetHeaders tests the GetHeaders function.
func TestResults_GetHeaders(t *testing.T) {
	want := []interface{}{"ID", "Name", "CIDR", "CIDR Blocks", "State", "Default", "Owner ID", "DHCP Options ID", "Tags"}
	if got := mockResults().GetHeaders(); !reflect.DeepEqual(got, want) {
		t.Errorf("GetHeaders()\n%#v\nwant\n%#v", got, want)
	}
}

// TestResults_GetRows tests the GetRows function.
func TestResults_GetRows(t *testing.T) {
	want := []interface{}{mockDataRow1, mockDataRow2}
	if got := mockResults().GetRows(); !reflect.DeepEqual(got, want) {
		t.Errorf("GetRows()\n%#v\nwant\n%#v", got, want)
	}
}

// TestResults_getFilters tests the getFilters function.
func TestResults_getFilters(t *testing.T) {
	tests := []struct {
		name        string
		filters     map[string][]string
		wantVpcIDs  []string
		wantFilters map[string][]string
		wantErr     bool
	}{
		{
			name:        "empty filters",
			filters:     map[string][]string{},
			wantVpcIDs:  nil,
			wantFilters: map[string][]string{},
		},
		{
			name:       "vpc-id becomes VpcIds",
			filters:    map[string][]string{"vpc-id": {"vpc-1", "vpc-2"}},
			wantVpcIDs: []string{"vpc-1", "vpc-2"},
		},
		{
			name:        "tag:Name and tag are expanded",
			filters:     map[string][]string{"tag:Name": {"prod"}, "tag": {"Environment=prod:dev"}},
			wantFilters: map[string][]string{"tag:Name": {"prod"}, "tag:Environment": {"prod", "dev"}},
		},
		{
			name:    "cidr and state pass through",
			filters: map[string][]string{FilterKeyCIDR: {"10.0.0.0/16"}, "state": {"available"}, "is-default": {"true"}},
			wantFilters: map[string][]string{
				FilterKeyCIDR: {"10.0.0.0/16"}, "state": {"available"}, "is-default": {"true"},
			},
		},
		{
			name:    "malformed tag returns error",
			filters: map[string][]string{"tag": {"NoEqualsSign"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", tt.filters, "id")
			got, err := r.getFilters()
			if (err != nil) != tt.wantErr {
				t.Fatalf("getFilters() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got.VpcIds, tt.wantVpcIDs) {
				t.Errorf("getFilters() VpcIds = %v, want %v", got.VpcIds, tt.wantVpcIDs)
			}
			gotFilters := map[string][]string{}
			for _, f := range got.Filters {
				gotFilters[*f.Name] = f.Values
			}
			if tt.wantFilters != nil && !reflect.DeepEqual(gotFilters, tt.wantFilters) {
				t.Errorf("getFilters() Filters = %v, want %v", gotFilters, tt.wantFilters)
			}
		})
	}
}

// TestParseVpc tests parseVpc with populated and nil fields.
func TestParseVpc(t *testing.T) {
	isDefault := false
	tests := []struct {
		name string
		vpc  types.Vpc
		want dataRow
	}{
		{
			name: "all fields",
			vpc: types.Vpc{
				VpcId:         common.String("vpc-1"),
				CidrBlock:     common.String("10.0.0.0/16"),
				State:         types.VpcStateAvailable,
				IsDefault:     &isDefault,
				OwnerId:       common.String("123456789012"),
				DhcpOptionsId: common.String("dopt-1"),
				CidrBlockAssociationSet: []types.VpcCidrBlockAssociation{
					{CidrBlock: common.String("10.0.0.0/16")},
					{CidrBlock: nil},
					{CidrBlock: common.String("10.1.0.0/16")},
				},
				Tags: []types.Tag{{Key: common.String("Name"), Value: common.String("prod")}},
			},
			want: dataRow{
				VpcID:         "vpc-1",
				Name:          "prod",
				CidrBlock:     "10.0.0.0/16",
				CidrBlocks:    []string{"10.0.0.0/16", "10.1.0.0/16"},
				State:         "available",
				IsDefault:     "false",
				OwnerID:       "123456789012",
				DhcpOptionsID: "dopt-1",
				Tags:          map[string]string{"Name": "prod"},
			},
		},
		{
			name: "nil pointers and no associations",
			vpc:  types.Vpc{},
			want: dataRow{Tags: map[string]string{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseVpc(&tt.vpc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseVpc()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_sortResults tests sorting by string and slice fields, and the invalid field error.
func TestResults_sortResults(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		want    []string
		wantErr bool
	}{
		{name: "id", field: "id", want: []string{"vpc-0000000000000000a", "vpc-0000000000000000b"}},
		{name: "name", field: "name", want: []string{"vpc-0000000000000000b", "vpc-0000000000000000a"}},
		{name: "cidrs slice", field: "cidrs", want: []string{"vpc-0000000000000000a", "vpc-0000000000000000b"}},
		{name: "default", field: "default", want: []string{"vpc-0000000000000000a", "vpc-0000000000000000b"}},
		{name: "invalid", field: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := mockResults()
			r.Data = []dataRow{mockDataRow2, mockDataRow1}
			err := r.sortResults(tt.field)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortResults(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			got := []string{r.Data[0].VpcID, r.Data[1].VpcID}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortResults(%q) order = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// TestGetSortFields tests GetSortFields and SortFieldNames.
func TestGetSortFields(t *testing.T) {
	want := map[string]string{
		"id": "VpcID", "name": "Name", "cidr": "CidrBlock", "cidrs": "CidrBlocks",
		"state": "State", "default": "IsDefault", "owner": "OwnerID", "dhcp": "DhcpOptionsID",
	}
	got, err := GetSortFields("id")
	if err != nil {
		t.Fatalf("GetSortFields(id) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetSortFields()\n%#v\nwant\n%#v", got, want)
	}
	if _, err := GetSortFields("invalid"); err == nil {
		t.Error("GetSortFields(invalid) error = nil, want error")
	}
	wantNames := []string{"cidr", "cidrs", "default", "dhcp", "id", "name", "owner", "state"}
	if names := SortFieldNames(); !reflect.DeepEqual(names, wantNames) {
		t.Errorf("SortFieldNames() = %v, want %v", names, wantNames)
	}
}

// idsByCIDRCase is one table entry of TestIDsByCIDR.
type idsByCIDRCase struct {
	name     string
	cidrs    []string
	mock     func(ctx context.Context, r *Results)
	want     []string
	wantErr  string
	wantCall bool
}

var idsByCIDRCases = []idsByCIDRCase{
	{
		name:  "no cidrs skips the search",
		cidrs: []string{},
		want:  []string{},
	},
	{
		name:  "returns the ids found",
		cidrs: []string{"10.0.0.0/16"},
		mock: func(_ context.Context, r *Results) {
			r.Data = append(r.Data, dataRow{VpcID: "vpc-1"}, dataRow{VpcID: "vpc-2"})
		},
		want:     []string{"vpc-1", "vpc-2"},
		wantCall: true,
	},
	{
		name:     "no match returns empty slice",
		cidrs:    []string{"10.9.0.0/16"},
		mock:     func(_ context.Context, _ *Results) {},
		want:     []string{},
		wantCall: true,
	},
	{
		name:  "search errors are returned",
		cidrs: []string{"10.0.0.0/16"},
		mock: func(_ context.Context, r *Results) {
			r.Errors = append(r.Errors, "boom")
		},
		wantErr:  "searching VPCs by CIDR: boom",
		wantCall: true,
	},
}

// TestIDsByCIDR tests the CIDR to VPC ID lookup with the AWS call mocked.
func TestIDsByCIDR(t *testing.T) {
	for _, tt := range idsByCIDRCases {
		t.Run(tt.name, func(t *testing.T) {
			old := searchFn
			t.Cleanup(func() { searchFn = old })
			called := false
			var gotFilters map[string][]string
			searchFn = func(ctx context.Context, r *Results) {
				called = true
				gotFilters = r.Filters
				if tt.mock != nil {
					tt.mock(ctx, r)
				}
			}

			got, err := IDsByCIDR(context.Background(), "default", "us-east-1", tt.cidrs)
			if called != tt.wantCall {
				t.Errorf("search called = %v, want %v", called, tt.wantCall)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("IDsByCIDR() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("IDsByCIDR() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IDsByCIDR() = %v, want %v", got, tt.want)
			}
			wantFilters := map[string][]string{FilterKeyCIDR: tt.cidrs}
			if tt.wantCall && !reflect.DeepEqual(gotFilters, wantFilters) {
				t.Errorf("IDsByCIDR() searched with filters %v, want %v", gotFilters, wantFilters)
			}
		})
	}
}
