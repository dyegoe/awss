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

// Package ebs contains the search for EBS volumes.
//
// It implements the common.Results interface.
package ebs

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
	tests := []struct {
		name           string
		profile        string
		region         string
		filters        map[string][]string
		sortField      string
		noInstanceName bool
		want           *Results
	}{
		{
			name:      "TestNew",
			profile:   "default",
			region:    "us-east-1",
			filters:   map[string][]string{"volume-id": {"vol-1234567890abcdef0"}},
			sortField: "id",
			want: &Results{
				BaseResults: common.BaseResults{
					Profile:   "default",
					Region:    "us-east-1",
					Errors:    []string{},
					SortField: "id",
				},
				Data:    []dataRow{},
				Filters: map[string][]string{"volume-id": {"vol-1234567890abcdef0"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.profile, tt.region, tt.filters, tt.sortField, tt.noInstanceName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New()\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// mockResultsEmpty returns a new empty result set. Each test gets its own: some sort it in place.
func mockResultsEmpty() *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile: "",
			Region:  "",
			Errors:  []string{},
		},
		Data:    []dataRow{},
		Filters: map[string][]string{},
	}
}

// mockResults returns a new result set with two rows. Each test gets its own: some sort it in place.
func mockResults() *Results {
	return &Results{
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
			mockDataRow1(),
			mockDataRow2(),
		},
		Filters: map[string][]string{
			"volume-id":         {"vol-1234567890abcdef0"},
			"tag":               {"key=value:value3", "key2=value2"},
			"availability-zone": {"a", "b"},
			"status":            {"available"},
		},
	}
}

// mockDataRow1 returns a new volume row.
func mockDataRow1() dataRow {
	return dataRow{
		VolumeID:         "vol-1234567890abcdef0",
		Size:             100,
		VolumeType:       "gp3",
		State:            "in-use",
		AvailabilityZone: "us-east-1a",
		Encrypted:        "true",
		InstanceID:       "i-1234567890abcdef0",
		InstanceName:     "instance-name-1",
		Device:           "/dev/sda1",
		Tags: map[string]string{
			"Name":        "volume-1",
			"Environment": "test",
		},
	}
}

// mockDataRow2 returns a new volume row.
func mockDataRow2() dataRow {
	return dataRow{
		VolumeID:         "vol-1234567890abcdef1",
		Size:             200,
		VolumeType:       "io2",
		State:            "available",
		AvailabilityZone: "us-east-1b",
		Encrypted:        "false",
		InstanceID:       "",
		InstanceName:     "",
		Device:           "",
		Tags: map[string]string{
			"Name":        "volume-2",
			"Environment": "prod",
		},
	}
}

// TestResults_accessors tests Len and the BaseResults getters on a full and an empty result set.
func TestResults_accessors(t *testing.T) {
	tests := []struct {
		name          string
		results       *Results
		wantLen       int
		wantProfile   string
		wantRegion    string
		wantSortField string
		wantErrors    []string
	}{
		{
			name: "two rows with errors", results: mockResults(),
			wantLen:       2,
			wantProfile:   "default",
			wantRegion:    "us-east-1",
			wantSortField: "id",
			wantErrors:    []string{"error1", "error2"},
		},
		{
			name: "empty", results: mockResultsEmpty(),
			wantLen:       0,
			wantProfile:   "",
			wantRegion:    "",
			wantSortField: "",
			wantErrors:    []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.results
			if got := r.Len(); got != tt.wantLen {
				t.Errorf("Len() = %d, want %d", got, tt.wantLen)
			}
			if got := r.GetProfile(); got != tt.wantProfile {
				t.Errorf("GetProfile() = %q, want %q", got, tt.wantProfile)
			}
			if got := r.GetRegion(); got != tt.wantRegion {
				t.Errorf("GetRegion() = %q, want %q", got, tt.wantRegion)
			}
			if got := r.GetSortField(); got != tt.wantSortField {
				t.Errorf("GetSortField() = %q, want %q", got, tt.wantSortField)
			}
			if got := r.GetErrors(); !reflect.DeepEqual(got, tt.wantErrors) {
				t.Errorf("GetErrors() = %q, want %q", got, tt.wantErrors)
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
			results: mockResults(),
			want: []interface{}{
				"ID", "Size (GiB)", "Type", "State", "AZ",
				"Encrypted", "Instance ID", "Instance Name", "Device", "Tags",
			},
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

// TestResults_GetRows tests the GetRows method.
func TestResults_GetRows(t *testing.T) {
	tests := []struct {
		name    string
		results *Results
		want    []interface{}
	}{
		{
			name:    "TestResults_GetRows",
			results: mockResults(),
			want: []interface{}{
				mockDataRow1(),
				mockDataRow2(),
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
		want    *ec2.DescribeVolumesInput
	}{
		{
			name:    "multiple filters",
			results: mockResults(),
			want: &ec2.DescribeVolumesInput{
				VolumeIds: []string{"vol-1234567890abcdef0"},
				Filters: []types.Filter{
					{Name: common.String("tag:key"), Values: []string{"value", "value3"}},
					{Name: common.String("tag:key2"), Values: []string{"value2"}},
					{Name: common.String("availability-zone"), Values: []string{"us-east-1a", "us-east-1b"}},
					{Name: common.String("status"), Values: []string{"available"}},
				},
			},
		},
		{
			name: "empty filters",
			results: &Results{
				BaseResults: common.BaseResults{},
				Data:        []dataRow{},
				Filters:     map[string][]string{},
			},
			want: &ec2.DescribeVolumesInput{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.results.getFilters()
			if err != nil {
				t.Fatalf("Results.getFilters() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got.VolumeIds, tt.want.VolumeIds) {
				t.Errorf("VolumeIds = %v, want %v", got.VolumeIds, tt.want.VolumeIds)
			}
			if len(got.Filters) != len(tt.want.Filters) {
				t.Fatalf("Filters count = %d, want %d", len(got.Filters), len(tt.want.Filters))
			}
			gotByName := make(map[string][]string, len(got.Filters))
			for _, f := range got.Filters {
				gotByName[*f.Name] = f.Values
			}
			for _, wf := range tt.want.Filters {
				gv, ok := gotByName[*wf.Name]
				if !ok {
					t.Errorf("missing filter %q", *wf.Name)
					continue
				}
				if !reflect.DeepEqual(gv, wf.Values) {
					t.Errorf("filter %q values = %v, want %v", *wf.Name, gv, wf.Values)
				}
			}
		})
	}
}

// TestResults_getFilters_malformedTag tests getFilters with a malformed tag filter.
func TestResults_getFilters_malformedTag(t *testing.T) {
	r := &Results{
		BaseResults: common.BaseResults{},
		Data:        []dataRow{},
		Filters:     map[string][]string{"tag": {"invalid"}},
	}
	_, err := r.getFilters()
	if err == nil {
		t.Error("Results.getFilters() expected error for malformed tag, got nil")
	}
}

// TestResults_sortResults tests the sortResults function.
func TestResults_sortResults(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		wantErr   bool
		wantFirst string // expected VolumeID of first row after sort
	}{
		{
			name:      "sort by id ascending",
			field:     "id",
			wantFirst: "vol-1234567890abcdef0",
		},
		{
			name:      "sort by size ascending (numeric)",
			field:     "size",
			wantFirst: "vol-1234567890abcdef0",
		},
		{
			name:      "sort by state ascending",
			field:     "state",
			wantFirst: "vol-1234567890abcdef1",
		},
		{
			name:    "invalid field",
			field:   "invalid",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Results{
				Data: []dataRow{
					{VolumeID: "vol-1234567890abcdef1", Size: 200, State: "available"},
					{VolumeID: "vol-1234567890abcdef0", Size: 100, State: "in-use"},
				},
			}
			err := r.sortResults(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("sortResults(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
				return
			}
			if !tt.wantErr && r.Data[0].VolumeID != tt.wantFirst {
				t.Errorf("sortResults(%q) first row = %s, want %s", tt.field, r.Data[0].VolumeID, tt.wantFirst)
			}
		})
	}
}

// TestResults_sortResults_sizeNumeric verifies size sorts numerically, not lexicographically.
func TestResults_sortResults_sizeNumeric(t *testing.T) {
	r := &Results{
		Data: []dataRow{
			{VolumeID: "vol-a", Size: 100},
			{VolumeID: "vol-b", Size: 20},
			{VolumeID: "vol-c", Size: 1000},
		},
	}
	if err := r.sortResults("size"); err != nil {
		t.Fatalf("sortResults(size) unexpected error: %v", err)
	}
	want := []int32{20, 100, 1000}
	for i, w := range want {
		if r.Data[i].Size != w {
			t.Errorf("sortResults(size) index %d = %d, want %d", i, r.Data[i].Size, w)
		}
	}
}

func parseVolumeBaseVol() types.Volume {
	strPtr := func(s string) *string { return &s }
	b, i := true, int32(50)
	return types.Volume{
		VolumeId:         strPtr("vol-abc123"),
		VolumeType:       types.VolumeTypeGp3,
		State:            types.VolumeStateInUse,
		AvailabilityZone: strPtr("us-east-1a"),
		Size:             &i,
		Encrypted:        &b,
	}
}

func parseVolumeBaseRow() dataRow {
	return dataRow{
		VolumeID: "vol-abc123", VolumeType: "gp3", State: "in-use",
		AvailabilityZone: "us-east-1a", Size: 50, Encrypted: "true",
		Tags: map[string]string{},
	}
}

// TestParseVolume_noAttachments tests parseVolume when the volume has no attachments.
func TestParseVolume_noAttachments(t *testing.T) {
	vol := parseVolumeBaseVol()
	got := parseVolume(&vol)
	if len(got) != 1 {
		t.Fatalf("parseVolume() len = %d, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0], parseVolumeBaseRow()) {
		t.Errorf("parseVolume()\n%#v\nwant\n%#v", got[0], parseVolumeBaseRow())
	}
}

// TestParseVolume_singleAttachment tests parseVolume when the volume has one attachment.
func TestParseVolume_singleAttachment(t *testing.T) {
	s := func(v string) *string { return &v }
	vol := parseVolumeBaseVol()
	vol.Attachments = []types.VolumeAttachment{
		{InstanceId: s("i-111"), Device: s("/dev/sda1")},
	}
	got := parseVolume(&vol)
	if len(got) != 1 {
		t.Fatalf("parseVolume() len = %d, want 1", len(got))
	}
	want := parseVolumeBaseRow()
	want.InstanceID, want.Device = "i-111", "/dev/sda1"
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("parseVolume()\n%#v\nwant\n%#v", got[0], want)
	}
}

// TestParseVolume_multiAttach tests parseVolume for Multi-Attach volumes (one row per attachment).
func TestParseVolume_multiAttach(t *testing.T) {
	s := func(v string) *string { return &v }
	vol := parseVolumeBaseVol()
	vol.Attachments = []types.VolumeAttachment{
		{InstanceId: s("i-111"), Device: s("/dev/sda1")},
		{InstanceId: s("i-222"), Device: s("/dev/sdb1")},
	}
	got := parseVolume(&vol)
	if len(got) != 2 {
		t.Fatalf("parseVolume() len = %d, want 2", len(got))
	}
	for i, tc := range []struct{ id, dev string }{{"i-111", "/dev/sda1"}, {"i-222", "/dev/sdb1"}} {
		want := parseVolumeBaseRow()
		want.InstanceID, want.Device = tc.id, tc.dev
		if !reflect.DeepEqual(got[i], want) {
			t.Errorf("parseVolume() row[%d]\n%#v\nwant\n%#v", i, got[i], want)
		}
	}
}

// TestGetSortFields tests the GetSortFields function.
func TestGetSortFields(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		wantErr bool
	}{
		{name: "valid sort field id", field: "id"},
		{name: "valid sort field size", field: "size"},
		{name: "valid sort field type", field: "type"},
		{name: "valid sort field state", field: "state"},
		{name: "valid sort field az", field: "az"},
		{name: "valid sort field encrypted", field: "encrypted"},
		{name: "valid sort field instance-id", field: "instance-id"},
		{name: "valid sort field instance-name", field: "instance-name"},
		{name: "valid sort field device", field: "device"},
		{name: "invalid sort field", field: "invalid", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetSortFields(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSortFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// fakeEC2 is an ec2API that serves the given pages of volumes and the given instances, and
// records the inputs it receives.
type fakeEC2 struct {
	volPages   [][]types.Volume
	volErr     error
	volInputs  []*ec2.DescribeVolumesInput
	instances  []types.Instance
	instErr    error
	instInputs []*ec2.DescribeInstancesInput
}

func (f *fakeEC2) DescribeVolumes(
	_ context.Context, in *ec2.DescribeVolumesInput, _ ...func(*ec2.Options),
) (*ec2.DescribeVolumesOutput, error) {
	f.volInputs = append(f.volInputs, in)
	if f.volErr != nil {
		return nil, f.volErr
	}
	out := &ec2.DescribeVolumesOutput{}
	page := len(f.volInputs) - 1
	if page < len(f.volPages) {
		out.Volumes = f.volPages[page]
	}
	if page < len(f.volPages)-1 {
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

// volume returns a volume attached to each of instanceIDs, or unattached when there are none.
func volume(id string, instanceIDs ...string) types.Volume {
	v := types.Volume{VolumeId: common.String(id)}
	for _, inst := range instanceIDs {
		v.Attachments = append(v.Attachments, types.VolumeAttachment{InstanceId: common.String(inst)})
	}
	return v
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
	wantRows       []string // "volume/instance-name" per row
	wantErrors     []string
	wantLookupIDs  []string // InstanceIds sent to DescribeInstances; nil means no call
}

func collectCases() []collectCase {
	return append(collectOKCases(), collectErrorCases()...)
}

// collectOKCases are the collectCase entries where every AWS call succeeds.
func collectOKCases() []collectCase {
	webDB := []types.Instance{instance("i-1", "web"), instance("i-2", "db")}
	return []collectCase{
		{
			name: "follows every page and fills instance names",
			client: &fakeEC2{
				volPages:  [][]types.Volume{{volume("vol-a", "i-1"), volume("vol-b")}, {volume("vol-c", "i-2")}},
				instances: webDB,
			},
			wantRows:      []string{"vol-a/web", "vol-b/", "vol-c/db"},
			wantLookupIDs: []string{"i-1", "i-2"},
		},
		{
			name: "multi-attach volume gives one row per instance, each instance looked up once",
			client: &fakeEC2{
				volPages:  [][]types.Volume{{volume("vol-a", "i-1", "i-2"), volume("vol-b", "i-1")}},
				instances: webDB,
			},
			wantRows:      []string{"vol-a/web", "vol-a/db", "vol-b/web"},
			wantLookupIDs: []string{"i-1", "i-2"},
		},
		{
			name:     "empty result makes no instance lookup",
			client:   &fakeEC2{},
			wantRows: []string{},
		},
		{
			name:           "no-instance-name skips the lookup",
			noInstanceName: true,
			client:         &fakeEC2{volPages: [][]types.Volume{{volume("vol-a", "i-1")}}},
			wantRows:       []string{"vol-a/"},
		},
		{
			name:      "rows are sorted by the sort field",
			sortField: "instance-name",
			client: &fakeEC2{
				volPages:  [][]types.Volume{{volume("vol-a", "i-1"), volume("vol-b", "i-2")}},
				instances: webDB,
			},
			wantRows:      []string{"vol-b/db", "vol-a/web"},
			wantLookupIDs: []string{"i-1", "i-2"},
		},
	}
}

// collectErrorCases are the collectCase entries where an AWS call fails or the sort field is bad.
func collectErrorCases() []collectCase {
	return []collectCase{
		{
			name:       "describe error is reported with no rows",
			client:     &fakeEC2{volErr: errors.New("boom")},
			wantRows:   []string{},
			wantErrors: []string{"error describing volumes: boom"},
		},
		{
			name: "failed name lookup keeps the rows",
			client: &fakeEC2{
				volPages: [][]types.Volume{{volume("vol-a", "i-1")}},
				instErr:  errors.New("denied"),
			},
			wantRows:      []string{"vol-a/"},
			wantErrors:    []string{"error searching instance names: denied"},
			wantLookupIDs: []string{"i-1"},
		},
		{
			name:           "unknown sort field is reported and the rows kept",
			sortField:      "nope",
			noInstanceName: true,
			client:         &fakeEC2{volPages: [][]types.Volume{{volume("vol-a")}}},
			wantRows:       []string{"vol-a/"},
			wantErrors: []string{
				"invalid sort field: nope. The options are: " +
					"az, device, encrypted, id, instance-id, instance-name, size, state, type",
			},
		},
	}
}

// TestResults_collect tests the paginated describe, the instance-name lookup and the sorting
// through a fake EC2 client.
func TestResults_collect(t *testing.T) {
	for _, tt := range collectCases() {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", nil, tt.sortField, tt.noInstanceName)
			input := &ec2.DescribeVolumesInput{Filters: common.FilterDefault("volume-type", []string{"gp3"})}

			r.collect(context.Background(), tt.client, input)

			rows := []string{}
			for _, row := range r.Data {
				rows = append(rows, row.VolumeID+"/"+row.InstanceName)
			}
			if !reflect.DeepEqual(rows, tt.wantRows) {
				t.Errorf("rows = %v, want %v", rows, tt.wantRows)
			}
			if !reflect.DeepEqual(r.Errors, append([]string{}, tt.wantErrors...)) {
				t.Errorf("Errors = %q, want %q", r.Errors, tt.wantErrors)
			}
			for _, in := range tt.client.volInputs {
				if !reflect.DeepEqual(in.Filters, input.Filters) {
					t.Errorf("DescribeVolumes filters = %#v, want %#v", in.Filters, input.Filters)
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

// TestResults_collectVolumeRows_pageSize tests that a page size is sent unless volume IDs are
// named, since AWS rejects MaxResults together with VolumeIds.
func TestResults_collectVolumeRows_pageSize(t *testing.T) {
	tests := []struct {
		name  string
		input *ec2.DescribeVolumesInput
		want  *int32
	}{
		{name: "no IDs asks for pages", input: &ec2.DescribeVolumesInput{}, want: aws.Int32(pageSize)},
		{name: "named IDs send no page size", input: &ec2.DescribeVolumesInput{VolumeIds: []string{"vol-a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeEC2{}
			r := New("default", "us-east-1", nil, "", true)
			if _, err := r.collectVolumeRows(context.Background(), client, tt.input); err != nil {
				t.Fatalf("collectVolumeRows() error = %v", err)
			}
			if got := client.volInputs[0].MaxResults; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MaxResults = %v, want %v", got, tt.want)
			}
			if tt.input.MaxResults != nil {
				t.Errorf("collectVolumeRows changed the caller's input")
			}
		})
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
