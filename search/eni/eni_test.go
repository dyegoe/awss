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

// Package eni contains the search for ENIs.
//
// It implements the common.Results interface
package eni

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

// TestNew tests the New function.
func TestNew(t *testing.T) {
	type args struct {
		profile        string
		region         string
		filters        map[string][]string
		sortField      string
		noInstanceName bool
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
				filters:   map[string][]string{"network-interface-id": {"eni-1234567890abcdef0"}},
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
				Filters: map[string][]string{"network-interface-id": {"eni-1234567890abcdef0"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.args.profile, tt.args.region, tt.args.filters, tt.args.sortField, tt.args.noInstanceName)
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
		"network-interface-id": {"eni-1234567890abcdef0"},
		"tag":                  {"key=value:value3", "key2=value2"},
		"availability-zone":    {"a", "b"},
		"private-ip-address":   {"172.16.0.1"},
		"owner-id":             {"123456789012"},
	},
}

var mockDataRow1 = &dataRow{
	InterfaceInfo:      *mockENIInfo1,
	PrivateIPAddresses: []string{"172.16.0.1", "172.16.0.2"},
	PublicIPAddresses:  []string{"51.52.53.54", "51.52.53.55"},
	Tags: map[string]string{
		"Name":        "instance-name-1",
		"Environment": "test",
	},
}

var mockDataRow2 = &dataRow{
	InterfaceInfo:      *mockENIInfo2,
	PrivateIPAddresses: []string{"172.16.1.1", "172.16.1.2"},
	PublicIPAddresses:  []string{"51.52.54.54", "51.52.54.55"},
	Tags: map[string]string{
		"Name":        "instance-name-2",
		"Environment": "prod",
	},
}

var mockENIInfo1 = &eniInfo{
	NetworkInterfaceID: "eni-1234567890abcdef0",
	InterfaceType:      "interface-type-1",
	AvailabilityZone:   "us-east-1a",
	Status:             "status-1",
	SubnetID:           "subnet-1234567890abcdef0",
	InstanceID:         "i-1234567890abcdef0",
	InstanceName:       "instance-name-1",
}

var mockENIInfo2 = &eniInfo{
	NetworkInterfaceID: "eni-1234567890abcdef1",
	InterfaceType:      "interface-type-2",
	AvailabilityZone:   "us-east-1b",
	Status:             "status-2",
	SubnetID:           "subnet-1234567890abcdef1",
	InstanceID:         "i-1234567890abcdef1",
	InstanceName:       "instance-name-2",
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
			want:    []interface{}{"Interface Info", "Private IPs", "Public IPs", "Tags"},
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
		want    *ec2.DescribeNetworkInterfacesInput
		wantErr bool
	}{
		{
			name:    "mutiple filters",
			results: mockResults,
			want: &ec2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: []string{"eni-1234567890abcdef0"},
				Filters: []types.Filter{
					{Name: common.String("tag:key"), Values: []string{"value", "value3"}},
					{Name: common.String("tag:key2"), Values: []string{"value2"}},
					{Name: common.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b"}},
					{Name: common.String("private-ip-address"), Values: []string{"172.16.0.1"}},
					{Name: common.String("owner-id"), Values: []string{"123456789012"}},
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

// TestResults_sortResults tests sorting on the nested InterfaceInfo fields.
func TestResults_sortResults(t *testing.T) {
	r := New("default", "us-east-1", map[string][]string{}, "id", false)
	r.Data = []dataRow{
		{InterfaceInfo: eniInfo{NetworkInterfaceID: "eni-b", SubnetID: "subnet-2", OwnerID: "333333333333"}},
		{InterfaceInfo: eniInfo{NetworkInterfaceID: "eni-a", SubnetID: "subnet-3", OwnerID: "111111111111"}},
		{InterfaceInfo: eniInfo{NetworkInterfaceID: "eni-c", SubnetID: "subnet-1", OwnerID: "222222222222"}},
	}
	tests := []struct {
		name    string
		field   string
		want    []string
		wantErr bool
	}{
		{name: "id", field: "id", want: []string{"eni-a", "eni-b", "eni-c"}},
		{name: "subnet-id", field: "subnet-id", want: []string{"eni-c", "eni-b", "eni-a"}},
		{name: "owner", field: "owner", want: []string{"eni-a", "eni-c", "eni-b"}},
		{name: "invalid", field: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.sortResults(tt.field)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortResults(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			got := make([]string, 0, len(r.Data))
			for i := range r.Data {
				got = append(got, r.Data[i].InterfaceInfo.NetworkInterfaceID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortResults(%q) order = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// TestParseENIRow tests the conversion of a NetworkInterface into a dataRow.
func TestParseENIRow(t *testing.T) {
	tests := []struct {
		name string
		eni  *types.NetworkInterface
		want eniInfo
	}{
		{
			name: "owner and requester",
			eni: &types.NetworkInterface{
				NetworkInterfaceId: common.String("eni-1"),
				OwnerId:            common.String("111111111111"),
				RequesterId:        common.String("AROAEXAMPLE:lambda"),
				RequesterManaged:   aws.Bool(true),
			},
			want: eniInfo{
				NetworkInterfaceID: "eni-1",
				OwnerID:            "111111111111",
				RequesterID:        "AROAEXAMPLE:lambda",
				RequesterManaged:   true,
			},
		},
		{
			name: "nil owner and requester",
			eni:  &types.NetworkInterface{NetworkInterfaceId: common.String("eni-2")},
			want: eniInfo{NetworkInterfaceID: "eni-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseENIRow(tt.eni).InterfaceInfo
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseENIRow().InterfaceInfo = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// fakeEC2 is an ec2API that serves the given pages of network interfaces and instances, and
// records the inputs it receives.
type fakeEC2 struct {
	eniPages   [][]types.NetworkInterface
	eniErr     error
	eniInputs  []*ec2.DescribeNetworkInterfacesInput
	instances  []types.Instance
	instErr    error
	instInputs []*ec2.DescribeInstancesInput
}

func (f *fakeEC2) DescribeNetworkInterfaces(
	_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options),
) (*ec2.DescribeNetworkInterfacesOutput, error) {
	f.eniInputs = append(f.eniInputs, in)
	if f.eniErr != nil {
		return nil, f.eniErr
	}
	out := &ec2.DescribeNetworkInterfacesOutput{}
	page := len(f.eniInputs) - 1
	if page < len(f.eniPages) {
		out.NetworkInterfaces = f.eniPages[page]
	}
	if page < len(f.eniPages)-1 {
		out.NextToken = common.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

func (f *fakeEC2) DescribeInstances(
	_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	f.instInputs = append(f.instInputs, in)
	if f.instErr != nil {
		return nil, f.instErr
	}
	return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{{Instances: f.instances}}}, nil
}

// iface returns a network interface, attached to instanceID unless it is empty.
func iface(id, instanceID string) types.NetworkInterface {
	n := types.NetworkInterface{NetworkInterfaceId: common.String(id)}
	if instanceID != "" {
		n.Attachment = &types.NetworkInterfaceAttachment{InstanceId: common.String(instanceID)}
	}
	return n
}

// instance returns an instance with the given ID and Name tag.
func instance(id, name string) types.Instance {
	return types.Instance{
		InstanceId: common.String(id),
		Tags:       []types.Tag{{Key: common.String("Name"), Value: common.String(name)}},
	}
}

// collectCase is one table entry of TestResults_collect.
type collectCase struct {
	name           string
	sortField      string
	noInstanceName bool
	client         *fakeEC2
	wantIDs        []string
	wantNames      []string
	wantErrors     []string
	wantLookupIDs  []string // InstanceIds sent to DescribeInstances; nil means no call
}

func collectCases() []collectCase {
	return append(collectOKCases(), collectErrorCases()...)
}

// collectErrorCases are the collectCase entries where an AWS call fails.
func collectErrorCases() []collectCase {
	return []collectCase{
		{
			name:       "describe error is reported with no rows",
			client:     &fakeEC2{eniErr: errors.New("boom")},
			wantIDs:    []string{},
			wantNames:  []string{},
			wantErrors: []string{"error describing network interfaces: boom"},
		},
		{
			name: "failed name lookup keeps the rows",
			client: &fakeEC2{
				eniPages: [][]types.NetworkInterface{{iface("eni-a", "i-1")}},
				instErr:  errors.New("denied"),
			},
			wantIDs:       []string{"eni-a"},
			wantNames:     []string{""},
			wantErrors:    []string{"error searching instance names: denied"},
			wantLookupIDs: []string{"i-1"},
		},
	}
}

// collectOKCases are the collectCase entries where every AWS call succeeds.
func collectOKCases() []collectCase {
	webDB := []types.Instance{instance("i-1", "web"), instance("i-2", "db")}
	return []collectCase{
		{
			name: "follows every page and fills instance names",
			client: &fakeEC2{
				eniPages:  [][]types.NetworkInterface{{iface("eni-a", "i-1"), iface("eni-b", "")}, {iface("eni-c", "i-2")}},
				instances: webDB,
			},
			wantIDs:       []string{"eni-a", "eni-b", "eni-c"},
			wantNames:     []string{"web", "", "db"},
			wantLookupIDs: []string{"i-1", "i-2"},
		},
		{
			name: "an instance with several interfaces is looked up once",
			client: &fakeEC2{
				eniPages:  [][]types.NetworkInterface{{iface("eni-a", "i-1"), iface("eni-b", "i-1")}},
				instances: webDB[:1],
			},
			wantIDs:       []string{"eni-a", "eni-b"},
			wantNames:     []string{"web", "web"},
			wantLookupIDs: []string{"i-1"},
		},
		{
			name:      "empty result makes no instance lookup",
			client:    &fakeEC2{},
			wantIDs:   []string{},
			wantNames: []string{},
		},
		{
			name:           "no-instance-name skips the lookup",
			noInstanceName: true,
			client:         &fakeEC2{eniPages: [][]types.NetworkInterface{{iface("eni-a", "i-1")}}},
			wantIDs:        []string{"eni-a"},
			wantNames:      []string{""},
		},
		{
			name:      "rows are sorted by the sort field",
			sortField: "instance-name",
			client: &fakeEC2{
				eniPages:  [][]types.NetworkInterface{{iface("eni-a", "i-1"), iface("eni-b", "i-2")}},
				instances: webDB,
			},
			wantIDs:       []string{"eni-b", "eni-a"},
			wantNames:     []string{"db", "web"},
			wantLookupIDs: []string{"i-1", "i-2"},
		},
	}
}

// TestResults_collect tests the paginated describe, the instance-name lookup and the sorting
// through a fake EC2 client.
func TestResults_collect(t *testing.T) {
	for _, tt := range collectCases() {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", nil, tt.sortField, tt.noInstanceName)
			input := &ec2.DescribeNetworkInterfacesInput{Filters: common.FilterDefault("vpc-id", []string{"vpc-1"})}

			r.collect(context.Background(), tt.client, input)

			ids, names := []string{}, []string{}
			for _, row := range r.Data {
				ids = append(ids, row.InterfaceInfo.NetworkInterfaceID)
				names = append(names, row.InterfaceInfo.InstanceName)
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) || !reflect.DeepEqual(names, tt.wantNames) {
				t.Errorf("rows = %v %v, want %v %v", ids, names, tt.wantIDs, tt.wantNames)
			}
			if !reflect.DeepEqual(r.Errors, append([]string{}, tt.wantErrors...)) {
				t.Errorf("Errors = %q, want %q", r.Errors, tt.wantErrors)
			}
			for _, in := range tt.client.eniInputs {
				if !reflect.DeepEqual(in.Filters, input.Filters) {
					t.Errorf("DescribeNetworkInterfaces filters = %#v, want %#v", in.Filters, input.Filters)
				}
			}
			assertLookup(t, tt.client.instInputs, tt.wantLookupIDs)
		})
	}
}

// assertLookup checks the DescribeInstances calls: none when want is nil, else one call with want.
func assertLookup(t *testing.T, inputs []*ec2.DescribeInstancesInput, want []string) {
	t.Helper()
	if want == nil {
		if len(inputs) != 0 {
			t.Errorf("DescribeInstances called %d times, want none", len(inputs))
		}
		return
	}
	if len(inputs) != 1 || !reflect.DeepEqual(inputs[0].InstanceIds, want) {
		t.Errorf("DescribeInstances inputs = %#v, want one call with %v", inputs, want)
	}
}

// TestParseENIRow_ips tests that private IPs and their associated public IPs are collected.
func TestParseENIRow_ips(t *testing.T) {
	eni := &types.NetworkInterface{
		PrivateIpAddresses: []types.NetworkInterfacePrivateIpAddress{
			{
				PrivateIpAddress: common.String("10.0.0.1"),
				Association:      &types.NetworkInterfaceAssociation{PublicIp: common.String("203.0.113.1")},
			},
			{PrivateIpAddress: common.String("10.0.0.2")},
		},
	}
	row := parseENIRow(eni)
	if want := []string{"10.0.0.1", "10.0.0.2"}; !reflect.DeepEqual(row.PrivateIPAddresses, want) {
		t.Errorf("PrivateIPAddresses = %v, want %v", row.PrivateIPAddresses, want)
	}
	if want := []string{"203.0.113.1"}; !reflect.DeepEqual(row.PublicIPAddresses, want) {
		t.Errorf("PublicIPAddresses = %v, want %v", row.PublicIPAddresses, want)
	}
}

// TestResults_collect_badSortField tests that an unknown sort field is reported and the rows kept.
func TestResults_collect_badSortField(t *testing.T) {
	r := New("default", "us-east-1", nil, "nope", true)
	client := &fakeEC2{eniPages: [][]types.NetworkInterface{{iface("eni-a", "")}}}

	r.collect(context.Background(), client, &ec2.DescribeNetworkInterfacesInput{})

	if len(r.Data) != 1 || len(r.Errors) != 1 {
		t.Errorf("Data = %v, Errors = %q, want one row and one error", r.Data, r.Errors)
	}
}

// TestSortFieldNames tests that every sort tag of eniInfo is listed.
func TestSortFieldNames(t *testing.T) {
	want := []string{"az", "id", "instance-id", "instance-name", "owner", "status", "subnet-id", "type"}
	if got := SortFieldNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
}

// TestResults_Search_earlyErrors tests the errors Search reports before any AWS call.
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
			r := New("awss-test-missing-profile", "us-east-1", tt.filters, "", false)
			r.Search(context.Background())
			if len(r.Errors) != 1 || !strings.HasPrefix(r.Errors[0], tt.wantPrefix) {
				t.Errorf("Errors = %q, want one error starting with %q", r.Errors, tt.wantPrefix)
			}
		})
	}
}

// TestResults_collectENIs_pageSize tests that a page size is sent unless interface IDs are named,
// since AWS rejects MaxResults together with NetworkInterfaceIds.
func TestResults_collectENIs_pageSize(t *testing.T) {
	tests := []struct {
		name  string
		input *ec2.DescribeNetworkInterfacesInput
		want  *int32
	}{
		{name: "no IDs asks for pages", input: &ec2.DescribeNetworkInterfacesInput{}, want: aws.Int32(pageSize)},
		{
			name:  "named IDs send no page size",
			input: &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: []string{"eni-a"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeEC2{}
			r := New("default", "us-east-1", nil, "", true)
			if _, err := r.collectENIs(context.Background(), client, tt.input); err != nil {
				t.Fatalf("collectENIs() error = %v", err)
			}
			if got := client.eniInputs[0].MaxResults; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MaxResults = %v, want %v", got, tt.want)
			}
			if tt.input.MaxResults != nil {
				t.Errorf("collectENIs changed the caller's input")
			}
		})
	}
}
