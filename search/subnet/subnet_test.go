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
	"reflect"
	"testing"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var mockDataRow1 = dataRow{
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

var mockDataRow2 = dataRow{
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
		"Public IP on Launch", "Default for AZ", "Owner ID", "Tags",
	}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders()\n%#v\nwant\n%#v", got, wantHeaders)
	}
	wantRows := []interface{}{mockDataRow1, mockDataRow2}
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
		SubnetId:                common.String("subnet-1"),
		VpcId:                   common.String("vpc-1"),
		CidrBlock:               common.String("10.0.1.0/24"),
		AvailabilityZone:        common.String("us-east-1a"),
		AvailableIpAddressCount: &count,
		State:                   types.SubnetStateAvailable,
		MapPublicIpOnLaunch:     &yes,
		DefaultForAz:            &no,
		OwnerId:                 common.String("123456789012"),
		Tags:                    []types.Tag{{Key: common.String("Name"), Value: common.String("public-a")}},
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
			r.Data = []dataRow{big, mockDataRow2, mockDataRow1}
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
		"default": "DefaultForAz", "owner": "OwnerID",
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
	wantNames := []string{"available-ips", "az", "cidr", "default", "id", "name", "owner", "public-ip", "state", "vpc-id"}
	if names := SortFieldNames(); !reflect.DeepEqual(names, wantNames) {
		t.Errorf("SortFieldNames() = %v, want %v", names, wantNames)
	}
}

// mockSearch replaces the AWS call of IDsByCIDR for the duration of the test.
//
// It returns a getter for the filters the last search was called with.
func mockSearch(t *testing.T, fill func(r *Results)) func() map[string][]string {
	t.Helper()
	old := searchFn
	t.Cleanup(func() { searchFn = old })
	var used map[string][]string
	searchFn = func(_ context.Context, r *Results) {
		used = r.Filters
		fill(r)
	}
	return func() map[string][]string { return used }
}

// TestIDsByCIDR_found checks that matching subnet IDs are returned and the CIDR filter is used.
func TestIDsByCIDR_found(t *testing.T) {
	used := mockSearch(t, func(r *Results) {
		r.Data = append(r.Data, dataRow{SubnetID: "subnet-1"}, dataRow{SubnetID: "subnet-2"})
	})
	got, err := IDsByCIDR(context.Background(), "default", "us-east-1", []string{"10.0.1.0/24"})
	if err != nil {
		t.Fatalf("IDsByCIDR() unexpected error: %v", err)
	}
	if want := []string{"subnet-1", "subnet-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDsByCIDR() = %v, want %v", got, want)
	}
	if want := map[string][]string{FilterKeyCIDR: {"10.0.1.0/24"}}; !reflect.DeepEqual(used(), want) {
		t.Errorf("IDsByCIDR() searched with %v, want %v", used(), want)
	}
}

// TestIDsByCIDR_noMatch checks that an empty (non-nil) slice is returned when nothing matches.
func TestIDsByCIDR_noMatch(t *testing.T) {
	mockSearch(t, func(_ *Results) {})
	got, err := IDsByCIDR(context.Background(), "default", "us-east-1", []string{"10.9.0.0/24"})
	if err != nil {
		t.Fatalf("IDsByCIDR() unexpected error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("IDsByCIDR() = %#v, want empty non-nil slice", got)
	}
}

// TestIDsByCIDR_error checks that search errors are wrapped and returned.
func TestIDsByCIDR_error(t *testing.T) {
	mockSearch(t, func(r *Results) { r.Errors = append(r.Errors, "boom") })
	_, err := IDsByCIDR(context.Background(), "default", "us-east-1", []string{"10.0.1.0/24"})
	if err == nil || err.Error() != "searching subnets by CIDR: boom" {
		t.Errorf("IDsByCIDR() error = %v, want %q", err, "searching subnets by CIDR: boom")
	}
	if errors.Is(err, nil) {
		t.Error("IDsByCIDR() must return a non-nil error")
	}
}

// TestIDsByCIDR_empty checks that no search runs when no CIDR is given.
func TestIDsByCIDR_empty(t *testing.T) {
	called := false
	mockSearch(t, func(_ *Results) { called = true })
	got, err := IDsByCIDR(context.Background(), "default", "us-east-1", nil)
	if err != nil || len(got) != 0 || called {
		t.Errorf("IDsByCIDR(nil) = %v, %v, called=%v; want empty, nil, false", got, err, called)
	}
}
