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

package subnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockDataRow1 returns a new public subnet row.
func mockDataRow1() dataRow {
	return dataRow{
		SubnetID:         "subnet-0000000000000000a",
		Name:             "public-a",
		VpcID:            "vpc-1",
		CidrBlock:        "10.0.1.0/24",
		AvailabilityZone: "us-east-1a",
		AvailableIPs:     251,
		State:            "available",
		MapPublicIP:      "true",
		DefaultForAz:     "false",
		OwnerID:          "123456789012",
		Tags:             map[string]string{"Name": "public-a", "Tier": "public"},
	}
}

// mockDataRow2 returns a new private subnet row.
func mockDataRow2() dataRow {
	return dataRow{
		SubnetID:         "subnet-0000000000000000b",
		Name:             "private-b",
		VpcID:            "vpc-1",
		CidrBlock:        "10.0.2.0/24",
		AvailabilityZone: "us-east-1b",
		AvailableIPs:     30,
		State:            "available",
		MapPublicIP:      "false",
		DefaultForAz:     "false",
		OwnerID:          "123456789012",
		Tags:             map[string]string{"Name": "private-b"},
	}
}

func mockResults() *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   "default",
			Region:    "us-east-1",
			Errors:    []string{},
			SortField: "id",
		},
		Data: []dataRow{mockDataRow1(), mockDataRow2()},
		Filters: map[string][]string{
			"subnet-id": {"subnet-0000000000000000a"},
			"vpc-id":    {"vpc-1"},
		},
	}
}

// TestNew tests the New function.
func TestNew(t *testing.T) {
	filters := map[string][]string{"subnet-id": {"subnet-1"}}
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

// TestResults_accessors tests Len, GetHeaders, GetRows and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := mockResults()
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "default" || r.GetRegion() != "us-east-1" || r.GetSortField() != "id" {
		t.Errorf("getters = %q %q %q, want default us-east-1 id", r.GetProfile(), r.GetRegion(), r.GetSortField())
	}
	if got := r.GetErrors(); len(got) != 0 {
		t.Errorf("GetErrors() = %v, want empty", got)
	}
	wantHeaders := []interface{}{
		"ID", "Name", "VPC ID", "CIDR", "AZ", "Available IPs", "State",
		"Public IP on Launch", "Default for AZ", "Owner ID", "Owner", "Tags",
	}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders()\n%#v\nwant\n%#v", got, wantHeaders)
	}
	wantRows := []interface{}{mockDataRow1(), mockDataRow2()}
	if got := r.GetRows(); !reflect.DeepEqual(got, wantRows) {
		t.Errorf("GetRows()\n%#v\nwant\n%#v", got, wantRows)
	}
}

// TestResults_getFilters tests the getFilters function.
func TestResults_getFilters(t *testing.T) {
	tests := []struct {
		name          string
		filters       map[string][]string
		wantSubnetIDs []string
		wantFilters   map[string][]string
		wantErr       bool
	}{
		{
			name:          "subnet-id becomes SubnetIds",
			filters:       map[string][]string{"subnet-id": {"subnet-1", "subnet-2"}},
			wantSubnetIDs: []string{"subnet-1", "subnet-2"},
			wantFilters:   map[string][]string{},
		},
		{
			name:        "tag:Name and tag are expanded",
			filters:     map[string][]string{"tag:Name": {"public-*"}, "tag": {"Tier=public:private"}},
			wantFilters: map[string][]string{"tag:Name": {"public-*"}, "tag:Tier": {"public", "private"}},
		},
		{
			name:        "availability-zone letters get the region prefix",
			filters:     map[string][]string{"availability-zone": {"a", "b"}},
			wantFilters: map[string][]string{"availability-zone": {"us-east-1a", "us-east-1b"}},
		},
		{
			name:    "cidr-block, vpc-id and booleans pass through",
			filters: map[string][]string{FilterKeyCIDR: {"10.0.1.0/24"}, "vpc-id": {"vpc-1"}, "default-for-az": {"true"}},
			wantFilters: map[string][]string{
				FilterKeyCIDR: {"10.0.1.0/24"}, "vpc-id": {"vpc-1"}, "default-for-az": {"true"},
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
			got, err := New("default", "us-east-1", tt.filters, "id").getFilters()
			if (err != nil) != tt.wantErr {
				t.Fatalf("getFilters() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got.SubnetIds, tt.wantSubnetIDs) {
				t.Errorf("getFilters() SubnetIds = %v, want %v", got.SubnetIds, tt.wantSubnetIDs)
			}
			gotFilters := map[string][]string{}
			for _, f := range got.Filters {
				gotFilters[*f.Name] = f.Values
			}
			if !reflect.DeepEqual(gotFilters, tt.wantFilters) {
				t.Errorf("getFilters() Filters = %v, want %v", gotFilters, tt.wantFilters)
			}
		})
	}
}

// TestParseSubnet tests parseSubnet with populated and nil fields.
func TestParseSubnet(t *testing.T) {
	yes, no := true, false
	var count int32 = 251
	full := types.Subnet{
		SubnetId:                aws.String("subnet-1"),
		VpcId:                   aws.String("vpc-1"),
		CidrBlock:               aws.String("10.0.1.0/24"),
		AvailabilityZone:        aws.String("us-east-1a"),
		AvailableIpAddressCount: &count,
		State:                   types.SubnetStateAvailable,
		MapPublicIpOnLaunch:     &yes,
		DefaultForAz:            &no,
		OwnerId:                 aws.String("123456789012"),
		Tags:                    []types.Tag{{Key: aws.String("Name"), Value: aws.String("public-a")}},
	}
	tests := []struct {
		name   string
		subnet types.Subnet
		want   dataRow
	}{
		{
			name:   "all fields",
			subnet: full,
			want: dataRow{
				SubnetID: "subnet-1", Name: "public-a", VpcID: "vpc-1", CidrBlock: "10.0.1.0/24",
				AvailabilityZone: "us-east-1a", AvailableIPs: 251, State: "available",
				MapPublicIP: "true", DefaultForAz: "false", OwnerID: "123456789012",
				Tags: map[string]string{"Name": "public-a"},
			},
		},
		{
			name:   "nil pointers",
			subnet: types.Subnet{},
			want:   dataRow{Tags: map[string]string{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSubnet(&tt.subnet); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSubnet()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_sortResults tests string and numeric sorting, and the invalid field error.
func TestResults_sortResults(t *testing.T) {
	big := dataRow{SubnetID: "subnet-big", AvailableIPs: 1000, Name: "z"}
	tests := []struct {
		name    string
		field   string
		want    []string
		wantErr bool
	}{
		{name: "id", field: "id", want: []string{"subnet-0000000000000000a", "subnet-0000000000000000b", "subnet-big"}},
		{name: "name", field: "name", want: []string{"subnet-0000000000000000b", "subnet-0000000000000000a", "subnet-big"}},
		{
			name:  "available-ips is numeric (30 < 251 < 1000)",
			field: "available-ips",
			want:  []string{"subnet-0000000000000000b", "subnet-0000000000000000a", "subnet-big"},
		},
		{name: "invalid", field: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := mockResults()
			r.Data = []dataRow{big, mockDataRow2(), mockDataRow1()}
			err := r.sortResults(tt.field)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortResults(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			got := []string{r.Data[0].SubnetID, r.Data[1].SubnetID, r.Data[2].SubnetID}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortResults(%q) order = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// TestGetSortFields tests GetSortFields and SortFieldNames.
func TestGetSortFields(t *testing.T) {
	want := map[string]string{
		"id": "SubnetID", "name": "Name", "vpc-id": "VpcID", "cidr": "CidrBlock", "az": "AvailabilityZone",
		"available-ips": "AvailableIPs", "state": "State", "public-ip": "MapPublicIP",
		"default": "DefaultForAz", "owner": "OwnerID", "owner-name": "OwnerName",
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
	wantNames := []string{
		"available-ips", "az", "cidr", "default", "id", "name", "owner", "owner-name", "public-ip", "state", "vpc-id",
	}
	if names := SortFieldNames(); !reflect.DeepEqual(names, wantNames) {
		t.Errorf("SortFieldNames() = %v, want %v", names, wantNames)
	}
}

// fakeDescribeSubnets is a DescribeSubnetsAPIClient that serves one page of subnets per entry of
// pages and records the inputs it receives.
type fakeDescribeSubnets struct {
	pages  [][]types.Subnet
	err    error
	inputs []*ec2.DescribeSubnetsInput
}

func (f *fakeDescribeSubnets) DescribeSubnets(
	_ context.Context, in *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeSubnetsOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	out := &ec2.DescribeSubnetsOutput{}
	page := len(f.inputs) - 1
	if page < len(f.pages) {
		out.Subnets = f.pages[page]
	}
	if page < len(f.pages)-1 {
		out.NextToken = aws.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

// subnet returns a subnet with the given ID, VPC ID and CIDR block. Empty values stay nil.
func subnet(id, vpcID, cidr string) types.Subnet {
	s := types.Subnet{SubnetId: aws.String(id)}
	if vpcID != "" {
		s.VpcId = aws.String(vpcID)
	}
	if cidr != "" {
		s.CidrBlock = aws.String(cidr)
	}
	return s
}

// collectCase is one table entry of TestResults_collect.
type collectCase struct {
	name        string
	sortField   string
	input       *ec2.DescribeSubnetsInput
	client      *fakeDescribeSubnets
	wantIDs     []string
	wantErrors  []string
	wantCalls   int
	wantMaxSize *int32
}

func collectCases() []collectCase {
	twoPages := [][]types.Subnet{{subnet("subnet-c", "", ""), subnet("subnet-a", "", "")}, {subnet("subnet-b", "", "")}}
	return []collectCase{
		{
			name: "follows every page and asks for pages", client: &fakeDescribeSubnets{pages: twoPages},
			wantIDs: []string{"subnet-c", "subnet-a", "subnet-b"}, wantCalls: 2, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name: "rows are sorted by the sort field", sortField: "id", client: &fakeDescribeSubnets{pages: twoPages},
			wantIDs: []string{"subnet-a", "subnet-b", "subnet-c"}, wantCalls: 2, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name:      "named subnet IDs send no page size",
			input:     &ec2.DescribeSubnetsInput{SubnetIds: []string{"subnet-a"}},
			client:    &fakeDescribeSubnets{pages: [][]types.Subnet{{subnet("subnet-a", "", "")}}},
			wantIDs:   []string{"subnet-a"},
			wantCalls: 1,
		},
		{
			name: "empty result", client: &fakeDescribeSubnets{},
			wantIDs: []string{}, wantCalls: 1, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name: "describe error is reported with no rows", client: &fakeDescribeSubnets{err: errors.New("boom")},
			wantIDs: []string{}, wantErrors: []string{"error describing subnets: boom"},
			wantCalls: 1, wantMaxSize: aws.Int32(pageSize),
		},
		{
			name: "unknown sort field is reported and the rows kept", sortField: "nope",
			client:  &fakeDescribeSubnets{pages: [][]types.Subnet{{subnet("subnet-a", "", "")}}},
			wantIDs: []string{"subnet-a"},
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
				input = &ec2.DescribeSubnetsInput{Filters: common.FilterDefault("vpc-id", []string{"vpc-1"})}
			}
			r := New("default", "us-east-1", nil, tt.sortField)

			r.collect(context.Background(), tt.client, input)

			got := []string{}
			for i := range r.Data {
				got = append(got, r.Data[i].SubnetID)
			}
			if !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("rows = %v, want %v", got, tt.wantIDs)
			}
			if !reflect.DeepEqual(r.Errors, append([]string{}, tt.wantErrors...)) {
				t.Errorf("Errors = %q, want %q", r.Errors, tt.wantErrors)
			}
			if len(tt.client.inputs) != tt.wantCalls {
				t.Fatalf("DescribeSubnets calls = %d, want %d", len(tt.client.inputs), tt.wantCalls)
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

// emptyAwsConfig points the AWS config and credentials files at an empty file, so no profile
// exists and nothing is read from the user's files.
func emptyAwsConfig(t *testing.T) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "config")
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)
}

// TestResults_Search_earlyErrors tests the errors Search reports before any EC2 call.
func TestResults_Search_earlyErrors(t *testing.T) {
	emptyAwsConfig(t)

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

// mustNets parses CIDRs for tests.
func mustNets(t *testing.T, cidrs ...string) []*net.IPNet {
	t.Helper()
	nets, err := common.ParseIPv4CIDRs(cidrs)
	if err != nil {
		t.Fatal(err)
	}
	return nets
}

// regionSubnets is a VPC with a primary (10.121.224.0/20) and a secondary (100.64.0.0/16)
// CIDR block, plus a subnet of another VPC and one without a CIDR block, over two pages.
func regionSubnets() [][]types.Subnet {
	return [][]types.Subnet{
		{
			subnet("subnet-p1", "vpc-1", "10.121.224.0/24"),
			subnet("subnet-p2", "vpc-1", "10.121.225.0/24"),
			subnet("subnet-s1", "vpc-1", "100.64.0.0/18"),
		},
		{
			subnet("subnet-o1", "vpc-2", "100.64.64.0/18"),
			subnet("subnet-x", "vpc-3", ""),
		},
	}
}

// TestInCIDRs checks which subnets and VPCs a range resolves into, and that every subnet of the
// region is listed in pages, with no filter.
func TestInCIDRs(t *testing.T) {
	tests := []struct {
		name        string
		cidrs       []string
		wantSubnets []string
		wantVPCs    []string
	}{
		{name: "vpc primary block", cidrs: []string{"10.121.224.0/20"},
			wantSubnets: []string{"subnet-p1", "subnet-p2"}, wantVPCs: []string{"vpc-1"}},
		{name: "vpc secondary block", cidrs: []string{"100.64.0.0/18"},
			wantSubnets: []string{"subnet-s1"}, wantVPCs: []string{"vpc-1"}},
		{name: "range across vpcs", cidrs: []string{"100.64.0.0/16"},
			wantSubnets: []string{"subnet-o1", "subnet-s1"}, wantVPCs: []string{"vpc-1", "vpc-2"}},
		{name: "range smaller than a subnet", cidrs: []string{"10.121.225.16/28"},
			wantSubnets: []string{"subnet-p2"}, wantVPCs: []string{"vpc-1"}},
		{name: "no overlap", cidrs: []string{"192.168.0.0/16"}, wantSubnets: []string{}, wantVPCs: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeDescribeSubnets{pages: regionSubnets()}
			subnets, vpcs, err := inCIDRs(context.Background(), client, "default", "us-east-1", mustNets(t, tt.cidrs...))
			if err != nil {
				t.Fatalf("inCIDRs() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(subnets, tt.wantSubnets) || !reflect.DeepEqual(vpcs, tt.wantVPCs) {
				t.Errorf("inCIDRs() = %v, %v; want %v, %v", subnets, vpcs, tt.wantSubnets, tt.wantVPCs)
			}
			if len(client.inputs) != 2 {
				t.Fatalf("DescribeSubnets calls = %d, want 2", len(client.inputs))
			}
			for _, in := range client.inputs {
				if len(in.Filters) != 0 || len(in.SubnetIds) != 0 || !reflect.DeepEqual(in.MaxResults, aws.Int32(pageSize)) {
					t.Errorf("input Filters = %v, SubnetIds = %v, MaxResults = %v; want none, none, %d",
						in.Filters, in.SubnetIds, in.MaxResults, pageSize)
				}
			}
		})
	}
}

// TestInCIDRs_error checks that describe errors are wrapped and returned.
func TestInCIDRs_error(t *testing.T) {
	client := &fakeDescribeSubnets{err: errors.New("boom")}
	_, _, err := inCIDRs(context.Background(), client, "default", "us-east-1", mustNets(t, "10.0.1.0/24"))
	want := "searching subnets by CIDR: error describing subnets: boom"
	if err == nil || err.Error() != want {
		t.Errorf("inCIDRs() error = %v, want %q", err, want)
	}
}

// TestInCIDRs_awsConfigError checks that a config error is wrapped and returned.
func TestInCIDRs_awsConfigError(t *testing.T) {
	emptyAwsConfig(t)
	_, _, err := InCIDRs(context.Background(), "awss-test-missing-profile", "us-east-1", mustNets(t, "10.0.1.0/24"))
	if err == nil || !strings.HasPrefix(err.Error(), "searching subnets by CIDR: error getting aws config:") {
		t.Errorf("InCIDRs() error = %v, want a wrapped aws config error", err)
	}
}

// TestInCIDRs_empty checks that no AWS config is loaded when no CIDR is given.
func TestInCIDRs_empty(t *testing.T) {
	emptyAwsConfig(t)
	// A missing profile would fail if InCIDRs loaded the AWS config.
	subnets, vpcs, err := InCIDRs(context.Background(), "awss-test-missing-profile", "us-east-1", nil)
	if err != nil || !reflect.DeepEqual(subnets, []string{}) || !reflect.DeepEqual(vpcs, []string{}) {
		t.Errorf("InCIDRs(nil) = %v, %v, %v; want empty, empty, nil", subnets, vpcs, err)
	}
}

// TestResults_collect_ownerNames checks that a known owner gets its name from AccountNames, that
// an unknown owner keeps only its ID, and that no map means no names.
func TestResults_collect_ownerNames(t *testing.T) {
	owned := func(id, owner string) types.Subnet {
		s := subnet(id, "vpc-1", "10.0.0.0/24")
		s.OwnerId = aws.String(owner)
		return s
	}
	tests := []struct {
		name  string
		names map[string]string
		want  []string
	}{
		{
			name: "known and unknown owners", names: map[string]string{"111111111111": "network"},
			want: []string{"network", ""},
		},
		{name: "no account names", names: nil, want: []string{"", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", nil, "id")
			r.AccountNames = tt.names
			client := &fakeDescribeSubnets{
				pages: [][]types.Subnet{{owned("subnet-a", "111111111111"), owned("subnet-b", "222222222222")}},
			}

			r.collect(context.Background(), client, &ec2.DescribeSubnetsInput{})

			got := []string{}
			for i := range r.Data {
				got = append(got, r.Data[i].OwnerName)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OwnerName of the rows = %q, want %q", got, tt.want)
			}
		})
	}
}
