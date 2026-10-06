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

// Package ec2 contains the EC2 search functions.
//
// It implements the common.Results interface
package ec2

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	type args struct {
		profile   string
		region    string
		filters   map[string][]string
		sortField string
	}
	tests := []struct {
		name string
		args args
		want *Results
	}{
		{
			name: "TestNew",
			args: args{
				profile:   "default",
				region:    "us-east-1",
				filters:   map[string][]string{"tag:Name": {"test"}},
				sortField: "id",
			},
			want: &Results{
				BaseResults: common.BaseResults{
					Profile:   "default",
					Region:    "us-east-1",
					Errors:    []string{},
					SortField: "id",
				},
				Data:    []dataRow{},
				Filters: map[string][]string{"tag:Name": {"test"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.args.profile, tt.args.region, tt.args.filters, tt.args.sortField)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

var mockResultsEmpty = &Results{
	BaseResults: common.BaseResults{
		Profile: "",
		Region:  "",
		Errors:  []string{},
	},
	Data:    []dataRow{},
	Filters: map[string][]string{},
}

var mockResults = &Results{
	BaseResults: common.BaseResults{
		Profile: "default",
		Region:  "us-east-1",
		Errors: []string{
			"error1",
			"error2",
		},
		SortField: "id",
	},
	Data: []dataRow{
		*mockDataRow1,
		*mockDataRow2,
	},
	Filters: map[string][]string{
		"instance-id":                    {"i-1234567890abcdef0", "i-0987654321fedcba9"},
		"tag:Name":                       {"instance-name-1", "instance-name-2"},
		"tag":                            {"key=value:value3", "key2=value2"},
		"availability-zone":              {"a", "b"},
		"instance-state-name":            {"running", "stopped"},
		"block-device-mapping.volume-id": {"vol-1234567890abcdef0"},
	},
}

var mockDataRow1 = &dataRow{
	InstanceID:        "i-1234567890abcdef0",
	InstanceName:      "instance-name-1",
	InstanceType:      "t3.micro",
	AvailabilityZone:  "us-east-1a",
	InstanceState:     "running",
	PrivateIPAddress:  "172.16.0.1",
	PublicIPAddress:   "52.53.54.55",
	NetworkInterfaces: []string{"eni-1234567890abcdef0"},
	Volumes:           []string{"vol-1234567890abcdef0"},
	Tags: map[string]string{
		"Name":        "instance-name-1",
		"Environment": "test",
	},
}

var mockDataRow2 = &dataRow{
	InstanceID:        "i-1234567890abcdef1",
	InstanceName:      "instance-name-2",
	InstanceType:      "t3.medium",
	AvailabilityZone:  "us-east-1b",
	InstanceState:     "running",
	PrivateIPAddress:  "172.16.0.2",
	PublicIPAddress:   "52.53.54.56",
	NetworkInterfaces: []string{"eni-1234567890abcdef1"},
	Tags: map[string]string{
		"Name":        "instance-name-2",
		"Environment": "prod",
	},
}

// // TestResults_Search tests the Search function.
// func TestResults_Search(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		results *Results
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			tt.results.Search()
// 		})
// 	}
// }

// TestResults_Len tests the Len function.
func TestResults_Len(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    int
	}{
		{
			name:    "TestResults_Len",
			results: mockResults,
			want:    2,
		},
		{
			name:    "TestResults_Len_Empty",
			results: mockResultsEmpty,
			want:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.Len(); got != tt.want {
				t.Errorf("Results.Len()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetProfile tests the GetProfile function.
func TestResults_GetProfile(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    string
	}{
		{
			name:    "TestResults_GetProfile",
			results: mockResults,
			want:    "default",
		},
		{
			name:    "TestResults_GetProfile_Empty",
			results: mockResultsEmpty,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetProfile(); got != tt.want {
				t.Errorf("Results.GetProfile()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetRegion tests the GetRegion function.
func TestResults_GetRegion(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    string
	}{
		{
			name:    "TestResults_GetRegion",
			results: mockResults,
			want:    "us-east-1",
		},
		{
			name:    "TestResults_GetRegion_Empty",
			results: mockResultsEmpty,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetRegion(); got != tt.want {
				t.Errorf("Results.GetRegion()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetErrors tests the GetErrors function.
func TestResults_GetErrors(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    []string
	}{
		{
			name:    "TestResults_GetErrors",
			results: mockResults,
			want:    []string{"error1", "error2"},
		},
		{
			name:    "TestResults_GetErrors_Empty",
			results: mockResultsEmpty,
			want:    []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetErrors(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Results.GetErrors()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetSortField tests the GetSortField function.
func TestResults_GetSortField(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    string
	}{
		{
			name:    "TestResults_GetSortField",
			results: mockResults,
			want:    "id",
		},
		{
			name:    "TestResults_GetSortField_Empty",
			results: mockResultsEmpty,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetSortField(); got != tt.want {
				t.Errorf("Results.GetSortField()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetHeaders tests the GetHeaders function.
func TestResults_GetHeaders(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    []interface{}
	}{
		{
			name:    "TestResults_GetHeaders",
			results: mockResults,
			want:    []interface{}{"ID", "Name", "Type", "AZ", "State", "Private IP", "Public IP", "ENIs", "Volumes", "Tags"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetHeaders(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Results.GetHeaders()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_GetRows tests the GetRows method
func TestResults_GetRows(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    []interface{}
	}{
		{
			name:    "TestResults_GetRows",
			results: mockResults,
			want: []interface{}{
				*mockDataRow1,
				*mockDataRow2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.results.GetRows(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Results.GetRows()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestResults_getFilters tests the getFilters function.
func TestResults_getFilters(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    *ec2.DescribeInstancesInput
		wantErr bool
	}{
		{
			name:    "multiple filters",
			results: mockResults,
			want: &ec2.DescribeInstancesInput{
				InstanceIds: []string{"i-1234567890abcdef0", "i-0987654321fedcba9"},
				Filters: []types.Filter{
					{Name: common.String("tag:Name"), Values: []string{"instance-name-1", "instance-name-2"}},
					{Name: common.String("tag:key"), Values: []string{"value", "value3"}},
					{Name: common.String("tag:key2"), Values: []string{"value2"}},
					{Name: common.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b"}},
					{Name: common.String("instance-state-name"), Values: []string{"running", "stopped"}},
					{Name: common.String("block-device-mapping.volume-id"), Values: []string{"vol-1234567890abcdef0"}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.results.getFilters()
			if (err != nil) != tt.wantErr {
				t.Errorf("Results.getFilters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, i := range got.Filters {
				for _, j := range tt.want.Filters {
					if *i.Name != *j.Name {
						continue
					}
					if !reflect.DeepEqual(i.Values, j.Values) {
						t.Errorf("Results.getFilters()\n%#v\nwant\n%#v", got, tt.want)
					}
				}
			}
		})
	}
}

// TestResults_sortResults tests the sortResults function.
func TestResults_sortResults(t *testing.T) {
	type args struct {
		field string
	}
	tests := []struct {
		name    string
		results *Results
		args    args
		wantErr bool
	}{
		{
			name:    "type",
			results: mockResults,
			args: args{
				field: "type",
			},
			wantErr: false,
		},
		{
			name:    "invalid",
			results: mockResults,
			args: args{
				field: "invalid",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.results.sortResults(tt.args.field); (err != nil) != tt.wantErr {
				t.Errorf("Results.sortResults() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetSortFields tests the getSortFields function.
func TestGetSortFields(t *testing.T) {
	type args struct {
		f string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "TestGetSortFields",
			args: args{
				f: "id",
			},
			want: map[string]string{
				"id":         "InstanceID",
				"name":       "InstanceName",
				"type":       "InstanceType",
				"az":         "AvailabilityZone",
				"state":      "InstanceState",
				"private-ip": "PrivateIPAddress",
				"public-ip":  "PublicIPAddress",
				"enis":       "NetworkInterfaces",
				"volumes":    "Volumes",
			},
			wantErr: false,
		},
		{
			name: "TestGetSortFields",
			args: args{
				f: "invalid",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSortFields(tt.args.f)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSortFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSortFields()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestParseVolumeIDs tests parseVolumeIDs with nil and populated block device mappings.
func TestParseVolumeIDs(t *testing.T) {
	tests := []struct {
		name     string
		mappings []types.InstanceBlockDeviceMapping
		want     []string
	}{
		{
			name:     "no mappings returns nil",
			mappings: nil,
			want:     nil,
		},
		{
			name: "nil ebs block is skipped",
			mappings: []types.InstanceBlockDeviceMapping{
				{DeviceName: common.String("/dev/sda1"), Ebs: nil},
			},
			want: nil,
		},
		{
			name: "nil volume id is skipped",
			mappings: []types.InstanceBlockDeviceMapping{
				{DeviceName: common.String("/dev/sda1"), Ebs: &types.EbsInstanceBlockDevice{VolumeId: nil}},
			},
			want: nil,
		},
		{
			name: "two volumes",
			mappings: []types.InstanceBlockDeviceMapping{
				{DeviceName: common.String("/dev/sda1"), Ebs: &types.EbsInstanceBlockDevice{VolumeId: common.String("vol-1")}},
				{DeviceName: common.String("/dev/sdf"), Ebs: nil},
				{DeviceName: common.String("/dev/sdg"), Ebs: &types.EbsInstanceBlockDevice{VolumeId: common.String("vol-2")}},
			},
			want: []string{"vol-1", "vol-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseVolumeIDs(tt.mappings); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseVolumeIDs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestParseInstance_volumes tests that parseInstance fills the Volumes column and tolerates nil fields.
func TestParseInstance_volumes(t *testing.T) {
	inst := types.Instance{
		InstanceId: common.String("i-1"),
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{Ebs: &types.EbsInstanceBlockDevice{VolumeId: common.String("vol-a")}},
			{Ebs: nil},
		},
	}
	got := parseInstance(&inst)
	if got.InstanceID != "i-1" {
		t.Errorf("InstanceID = %q, want %q", got.InstanceID, "i-1")
	}
	if !reflect.DeepEqual(got.Volumes, []string{"vol-a"}) {
		t.Errorf("Volumes = %#v, want %#v", got.Volumes, []string{"vol-a"})
	}
	if got.AvailabilityZone != "" || got.InstanceState != "" {
		t.Errorf("nil Placement/State must yield empty strings, got az=%q state=%q", got.AvailabilityZone, got.InstanceState)
	}
}

// TestResults_sortResults_sliceField checks that sorting by a slice column reorders rows.
func TestResults_sortResults_sliceField(t *testing.T) {
	r := New("default", "us-east-1", map[string][]string{}, "enis")
	r.Data = []dataRow{
		{InstanceID: "i-1", NetworkInterfaces: []string{"eni-zzz"}, Volumes: []string{"vol-b"}},
		{InstanceID: "i-2", NetworkInterfaces: []string{"eni-aaa"}, Volumes: []string{"vol-d", "vol-c"}},
		{InstanceID: "i-3", NetworkInterfaces: nil, Volumes: nil},
	}
	tests := []struct {
		name  string
		field string
		want  []string
	}{
		{name: "enis", field: "enis", want: []string{"i-3", "i-2", "i-1"}},
		{name: "volumes", field: "volumes", want: []string{"i-3", "i-1", "i-2"}},
		{name: "id", field: "id", want: []string{"i-1", "i-2", "i-3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := r.sortResults(tt.field); err != nil {
				t.Fatalf("sortResults(%q) error = %v", tt.field, err)
			}
			got := make([]string, 0, len(r.Data))
			for i := range r.Data {
				got = append(got, r.Data[i].InstanceID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortResults(%q) order = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// mockSubnetsInCIDRs replaces the subnet lookup for the duration of the test.
//
// It returns the given ids and error, and records whether it was called.
func mockSubnetsInCIDRs(t *testing.T, subnetIDs, vpcIDs []string, lookupErr error) (called *bool) {
	t.Helper()
	old := subnetsInCIDRs
	t.Cleanup(func() { subnetsInCIDRs = old })
	called = new(bool)
	subnetsInCIDRs = func(_ context.Context, _, _ string, _ []*net.IPNet) ([]string, []string, error) {
		*called = true
		return subnetIDs, vpcIDs, lookupErr
	}
	return called
}

// manyIDs returns n IDs with the given prefix.
func manyIDs(prefix string, n int) []string {
	ids := make([]string, 0, n)
	for i := range n {
		ids = append(ids, fmt.Sprintf("%s-%d", prefix, i))
	}
	return ids
}

// resolveCIDRCase is one table entry of TestResults_resolveCIDRFilter.
type resolveCIDRCase struct {
	name       string
	filters    map[string][]string
	subnetIDs  []string
	vpcIDs     []string
	lookupErr  error
	want       map[string][]string
	wantNets   []string
	wantErr    string
	wantCalled bool
}

// resolveCIDRCases covers resolveCIDRFilter: subnets, the VPC fallback for many subnets, and errors.
func resolveCIDRCases() []resolveCIDRCase {
	base := map[string][]string{"cidr": {"100.64.0.0/16"}, "instance-state-name": {"running"}}
	running := []string{"running"}
	return []resolveCIDRCase{
		{
			name:    "no cidr filter returns filters untouched and calls nothing",
			filters: map[string][]string{"instance-id": {"i-1"}},
			want:    map[string][]string{"instance-id": {"i-1"}},
		},
		{
			name: "overlapping subnets become network-interface.subnet-id", filters: base,
			subnetIDs: []string{"subnet-1", "subnet-2"}, vpcIDs: []string{"vpc-1"},
			want:     map[string][]string{filterKeySubnetID: {"subnet-1", "subnet-2"}, "instance-state-name": running},
			wantNets: []string{"100.64.0.0/16"}, wantCalled: true,
		},
		{
			name: "too many subnets fall back to their vpcs", filters: base,
			subnetIDs: manyIDs("subnet", maxCIDRSubnets+1), vpcIDs: []string{"vpc-1", "vpc-2"},
			want:     map[string][]string{filterKeyVpcID: {"vpc-1", "vpc-2"}, "instance-state-name": running},
			wantNets: []string{"100.64.0.0/16"}, wantCalled: true,
		},
		{
			name: "no overlapping subnet is an error", filters: base,
			subnetIDs: []string{}, vpcIDs: []string{},
			wantErr: "no subnet found in CIDR 100.64.0.0/16 in us-east-1", wantCalled: true,
		},
		{
			name: "lookup error is returned", filters: base, lookupErr: errors.New("boom"),
			wantErr: "resolving CIDR filter: boom", wantCalled: true,
		},
		{
			name:    "wildcard is rejected before any lookup",
			filters: map[string][]string{"cidr": {"10.0.*"}},
			wantErr: "resolving CIDR filter: invalid CIDR 10.0.*: it must be an IPv4 range such as 10.0.0.0/16",
		},
	}
}

// TestResults_resolveCIDRFilter tests the CIDR resolution and the untouched shared filters map.
func TestResults_resolveCIDRFilter(t *testing.T) {
	for _, tt := range resolveCIDRCases() {
		t.Run(tt.name, func(t *testing.T) {
			called := mockSubnetsInCIDRs(t, tt.subnetIDs, tt.vpcIDs, tt.lookupErr)
			original := map[string][]string{}
			for k, v := range tt.filters {
				original[k] = append([]string(nil), v...)
			}
			r := New("default", "us-east-1", tt.filters, "id")

			got, nets, err := r.resolveCIDRFilter(context.Background())
			if *called != tt.wantCalled {
				t.Errorf("subnet lookup called = %v, want %v", *called, tt.wantCalled)
			}
			if !reflect.DeepEqual(r.Filters, original) {
				t.Errorf("resolveCIDRFilter() mutated the shared filters: %v, want %v", r.Filters, original)
			}
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("resolveCIDRFilter() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCIDRFilter() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveCIDRFilter() = %v, want %v", got, tt.want)
			}
			if gotNets := netStrings(nets); !reflect.DeepEqual(gotNets, tt.wantNets) {
				t.Errorf("resolveCIDRFilter() nets = %v, want %v", gotNets, tt.wantNets)
			}
		})
	}
}

// netStrings returns the networks as strings, or nil when there are none.
func netStrings(nets []*net.IPNet) []string {
	var out []string
	for _, n := range nets {
		out = append(out, n.String())
	}
	return out
}

// TestResults_Search_cidrNoMatch checks Search records the error and makes no AWS call when no CIDR matches.
func TestResults_Search_cidrNoMatch(t *testing.T) {
	mockSubnetsInCIDRs(t, []string{}, []string{}, nil)
	r := New("default", "us-east-1", map[string][]string{"cidr": {"10.9.0.0/16"}}, "id")

	r.Search(context.Background())

	if len(r.Data) != 0 {
		t.Errorf("Search() Data = %v, want empty", r.Data)
	}
	want := "no subnet found in CIDR 10.9.0.0/16 in us-east-1"
	if len(r.Errors) != 1 || r.Errors[0] != want {
		t.Errorf("Search() Errors = %v, want [%q]", r.Errors, want)
	}
}

// instanceWithIPs returns an instance whose network interfaces have the given private IPs, one ENI per slice.
func instanceWithIPs(id string, enis ...[]string) types.Instance {
	inst := types.Instance{InstanceId: common.String(id)}
	for _, ips := range enis {
		eni := types.InstanceNetworkInterface{}
		for _, ip := range ips {
			eni.PrivateIpAddresses = append(eni.PrivateIpAddresses,
				types.InstancePrivateIpAddress{PrivateIpAddress: common.String(ip)})
		}
		inst.NetworkInterfaces = append(inst.NetworkInterfaces, eni)
	}
	return inst
}

// TestResults_collectInstances tests that only instances with a private IP in the ranges are kept.
func TestResults_collectInstances(t *testing.T) {
	reservations := []types.Reservation{{Instances: []types.Instance{
		instanceWithIPs("i-primary-only", []string{"10.121.224.10"}),
		instanceWithIPs("i-both", []string{"10.121.224.11"}, []string{"100.64.1.5"}),
		instanceWithIPs("i-secondary-ip", []string{"10.121.224.12", "100.64.2.9"}),
		instanceWithIPs("i-no-ip"),
	}}}
	tests := []struct {
		name  string
		cidrs []string
		want  []string
	}{
		{name: "no ranges keeps every instance", want: []string{"i-primary-only", "i-both", "i-secondary-ip", "i-no-ip"}},
		{name: "primary vpc block", cidrs: []string{"10.121.224.0/20"},
			want: []string{"i-primary-only", "i-both", "i-secondary-ip"}},
		{name: "secondary vpc block", cidrs: []string{"100.64.0.0/16"}, want: []string{"i-both", "i-secondary-ip"}},
		{name: "range smaller than a subnet", cidrs: []string{"100.64.2.0/28"}, want: []string{"i-secondary-ip"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nets, err := common.ParseIPv4CIDRs(tt.cidrs)
			if err != nil {
				t.Fatal(err)
			}
			r := New("default", "us-east-1", map[string][]string{}, "id")
			r.collectInstances(reservations, nets)
			got := make([]string, 0, len(r.Data))
			for i := range r.Data {
				got = append(got, r.Data[i].InstanceID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectInstances() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFiltersToInput_rejectsRawCIDR checks an unresolved cidr key can never reach AWS as a filter name.
func TestFiltersToInput_rejectsRawCIDR(t *testing.T) {
	_, err := filtersToInput(map[string][]string{"cidr": {"10.0.1.0/24"}}, "us-east-1")
	if err == nil {
		t.Fatal("filtersToInput() error = nil, want error for unresolved cidr filter")
	}
	if !strings.Contains(err.Error(), "must be resolved") {
		t.Errorf("filtersToInput() error = %v, want mention of resolution", err)
	}
}

// TestFiltersToInput_subnetAndVpc checks the resolved keys pass through as AWS filters.
func TestFiltersToInput_subnetAndVpc(t *testing.T) {
	input, err := filtersToInput(map[string][]string{"subnet-id": {"subnet-1"}, "vpc-id": {"vpc-1"}}, "us-east-1")
	if err != nil {
		t.Fatalf("filtersToInput() error = %v", err)
	}
	got := map[string][]string{}
	for _, f := range input.Filters {
		got[*f.Name] = f.Values
	}
	want := map[string][]string{"subnet-id": {"subnet-1"}, "vpc-id": {"vpc-1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filtersToInput() filters = %v, want %v", got, want)
	}
}

// fakeDescribeInstances is a DescribeInstancesAPIClient that serves one page of instances per
// entry of pages and records the inputs it receives.
type fakeDescribeInstances struct {
	pages  [][]types.Instance
	err    error
	inputs []*ec2.DescribeInstancesInput
}

func (f *fakeDescribeInstances) DescribeInstances(
	_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	page := len(f.inputs) - 1
	out := &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{{Instances: f.pages[page]}}}
	if page < len(f.pages)-1 {
		out.NextToken = common.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

// namedInstance returns an instance with the given ID and, when name is not empty, a Name tag.
func namedInstance(id, name string) types.Instance {
	inst := types.Instance{InstanceId: common.String(id)}
	if name != "" {
		inst.Tags = []types.Tag{{Key: common.String("Name"), Value: common.String(name)}}
	}
	return inst
}

// TestInstanceNames tests the instance-name lookup through a fake client.
func TestInstanceNames(t *testing.T) {
	tests := []struct {
		name      string
		ids       []string
		client    *fakeDescribeInstances
		want      map[string]string
		wantErr   string
		wantCalls int
		wantIDs   []string
	}{
		{
			name: "follows every page and dedupes the IDs",
			ids:  []string{"i-2", "i-1", "i-2"},
			client: &fakeDescribeInstances{pages: [][]types.Instance{
				{namedInstance("i-1", "web")}, {namedInstance("i-2", "")},
			}},
			want:      map[string]string{"i-1": "web", "i-2": ""},
			wantCalls: 2,
			wantIDs:   []string{"i-1", "i-2"},
		},
		{
			name:   "no IDs makes no call",
			client: &fakeDescribeInstances{},
			want:   map[string]string{},
		},
		{
			name:      "api error is wrapped",
			ids:       []string{"i-1"},
			client:    &fakeDescribeInstances{err: errors.New("denied")},
			wantErr:   "error searching instance names: denied",
			wantCalls: 1,
			wantIDs:   []string{"i-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InstanceNames(context.Background(), tt.client, tt.ids)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("InstanceNames() error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InstanceNames() = %v, %v, want %v", got, err, tt.want)
			}
			if len(tt.client.inputs) != tt.wantCalls {
				t.Fatalf("DescribeInstances calls = %d, want %d", len(tt.client.inputs), tt.wantCalls)
			}
			if tt.wantCalls > 0 && !reflect.DeepEqual(tt.client.inputs[0].InstanceIds, tt.wantIDs) {
				t.Errorf("InstanceIds = %v, want %v", tt.client.inputs[0].InstanceIds, tt.wantIDs)
			}
		})
	}
}
