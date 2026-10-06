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
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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

// fakeDescribeVpcs is a DescribeVpcsAPIClient that serves one page of VPCs per entry of pages and
// records the inputs it receives.
type fakeDescribeVpcs struct {
	pages  [][]types.Vpc
	err    error
	inputs []*ec2.DescribeVpcsInput
}

func (f *fakeDescribeVpcs) DescribeVpcs(
	_ context.Context, in *ec2.DescribeVpcsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeVpcsOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	out := &ec2.DescribeVpcsOutput{}
	page := len(f.inputs) - 1
	if page < len(f.pages) {
		out.Vpcs = f.pages[page]
	}
	if page < len(f.pages)-1 {
		out.NextToken = common.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

// vpc returns a VPC with the given ID.
func vpc(id string) types.Vpc { return types.Vpc{VpcId: common.String(id)} }

// collectCase is one table entry of TestResults_collect.
type collectCase struct {
	name        string
	sortField   string
	input       *ec2.DescribeVpcsInput
	client      *fakeDescribeVpcs
	wantIDs     []string
	wantErrors  []string
	wantCalls   int
	wantMaxSize *int32
}

func collectCases() []collectCase {
	twoPages := [][]types.Vpc{{vpc("vpc-c"), vpc("vpc-a")}, {vpc("vpc-b")}}
	return []collectCase{
		{
			name: "follows every page and asks for pages", client: &fakeDescribeVpcs{pages: twoPages},
			wantIDs: []string{"vpc-c", "vpc-a", "vpc-b"}, wantCalls: 2, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name: "rows are sorted by the sort field", sortField: "id", client: &fakeDescribeVpcs{pages: twoPages},
			wantIDs: []string{"vpc-a", "vpc-b", "vpc-c"}, wantCalls: 2, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name:   "named VPC IDs send no page size",
			input:  &ec2.DescribeVpcsInput{VpcIds: []string{"vpc-a"}},
			client: &fakeDescribeVpcs{pages: [][]types.Vpc{{vpc("vpc-a")}}}, wantIDs: []string{"vpc-a"}, wantCalls: 1,
		},
		{
			name: "empty result", client: &fakeDescribeVpcs{},
			wantIDs: []string{}, wantCalls: 1, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name: "describe error is reported with no rows", client: &fakeDescribeVpcs{err: errors.New("boom")},
			wantIDs: []string{}, wantErrors: []string{"error describing vpcs: boom"},
			wantCalls: 1, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name: "unknown sort field is reported and the rows kept", sortField: "nope",
			client:  &fakeDescribeVpcs{pages: [][]types.Vpc{{vpc("vpc-a")}}},
			wantIDs: []string{"vpc-a"},
			wantErrors: []string{
				"invalid sort field: nope. The options are: " + strings.Join(common.SortFieldNames(dataRow{}), ", "),
			},
			wantCalls: 1, wantMaxSize: aws.Int32(pageSize),
		},
	}
}

// TestResults_collect tests the paginated describe, the page size and the sorting through a
// fake EC2 client.
func TestResults_collect(t *testing.T) {
	for _, tt := range collectCases() {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			if input == nil {
				input = &ec2.DescribeVpcsInput{Filters: common.FilterDefault("state", []string{"available"})}
			}
			r := New("default", "us-east-1", nil, tt.sortField)

			r.collect(context.Background(), tt.client, input)

			got := []string{}
			for i := range r.Data {
				got = append(got, r.Data[i].VpcID)
			}
			if !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("rows = %v, want %v", got, tt.wantIDs)
			}
			if !reflect.DeepEqual(r.Errors, append([]string{}, tt.wantErrors...)) {
				t.Errorf("Errors = %q, want %q", r.Errors, tt.wantErrors)
			}
			if len(tt.client.inputs) != tt.wantCalls {
				t.Fatalf("DescribeVpcs calls = %d, want %d", len(tt.client.inputs), tt.wantCalls)
			}
			for _, in := range tt.client.inputs {
				if !reflect.DeepEqual(in.MaxResults, tt.wantMaxSize) || !reflect.DeepEqual(in.Filters, input.Filters) {
					t.Errorf("input MaxResults = %v, Filters = %#v; want %v, %#v",
						in.MaxResults, in.Filters, tt.wantMaxSize, input.Filters)
				}
			}
			if input.MaxResults != nil {
				t.Errorf("collect changed the caller's input")
			}
		})
	}
}

// TestResults_Search_earlyErrors tests the errors Search reports before any EC2 call.
func TestResults_Search_earlyErrors(t *testing.T) {
	// An empty AWS config file: the profile does not exist and nothing is read from the user's files.
	empty := filepath.Join(t.TempDir(), "config")
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	tests := []struct {
		name       string
		filters    map[string][]string
		wantPrefix string
	}{
		{
			name:       "invalid tag filter",
			filters:    map[string][]string{"tag": {"no-equals-sign"}},
			wantPrefix: "error building filters:",
		},
		{
			name:       "unknown profile",
			filters:    map[string][]string{},
			wantPrefix: "error getting aws config:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("awss-test-missing-profile", "us-east-1", tt.filters, "")
			r.Search(context.Background())
			if len(r.Errors) != 1 || !strings.HasPrefix(r.Errors[0], tt.wantPrefix) {
				t.Errorf("Errors = %q, want one error starting with %q", r.Errors, tt.wantPrefix)
			}
		})
	}
}
