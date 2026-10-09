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

package rdscluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	searchRDS "github.com/dyegoe/awss/search/rds"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/smithy-go"
)

// fakeRDS is a fake RDS client: one page of clusters per call, the subnet groups in two pages,
// and the inputs recorded.
type fakeRDS struct {
	pages     [][]types.DBCluster
	err       error
	inputs    []*rds.DescribeDBClustersInput
	groups    []types.DBSubnetGroup
	groupErr  error
	groupReqs []*rds.DescribeDBSubnetGroupsInput
}

func (f *fakeRDS) DescribeDBClusters(
	_ context.Context, in *rds.DescribeDBClustersInput, _ ...func(*rds.Options),
) (*rds.DescribeDBClustersOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	out := &rds.DescribeDBClustersOutput{}
	page := len(f.inputs) - 1
	if page < len(f.pages) {
		out.DBClusters = f.pages[page]
	}
	if page < len(f.pages)-1 {
		out.Marker = aws.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

func (f *fakeRDS) DescribeDBSubnetGroups(
	_ context.Context, in *rds.DescribeDBSubnetGroupsInput, _ ...func(*rds.Options),
) (*rds.DescribeDBSubnetGroupsOutput, error) {
	f.groupReqs = append(f.groupReqs, in)
	if f.groupErr != nil {
		return nil, f.groupErr
	}
	// The first page holds the first group; the marker asks for the rest.
	if in.Marker == nil && len(f.groups) > 1 {
		return &rds.DescribeDBSubnetGroupsOutput{DBSubnetGroups: f.groups[:1], Marker: aws.String("rest")}, nil
	}
	if in.Marker != nil {
		return &rds.DescribeDBSubnetGroupsOutput{DBSubnetGroups: f.groups[1:]}, nil
	}
	return &rds.DescribeDBSubnetGroupsOutput{DBSubnetGroups: f.groups}, nil
}

// cluster returns an available cluster in subnet group "sg-<n>", with one writer and readers
// reader instances.
func cluster(id, engine, version string, n, readers int, tags ...string) types.DBCluster {
	c := types.DBCluster{
		DBClusterIdentifier: aws.String(id), Engine: aws.String(engine), EngineVersion: aws.String(version),
		Status: aws.String("available"), DBSubnetGroup: aws.String(fmt.Sprintf("sg-%d", n)),
		DBClusterMembers: []types.DBClusterMember{
			{DBInstanceIdentifier: aws.String(id + "-w"), IsClusterWriter: aws.Bool(true)},
		},
	}
	for i := range readers {
		c.DBClusterMembers = append(c.DBClusterMembers, types.DBClusterMember{
			DBInstanceIdentifier: aws.String(fmt.Sprintf("%s-r%d", id, i)), IsClusterWriter: aws.Bool(false),
		})
	}
	for _, kv := range tags {
		k, v, _ := strings.Cut(kv, "=")
		c.TagList = append(c.TagList, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return c
}

// fakeClient returns a client with two pages of clusters and the subnet groups of two VPCs.
func fakeClient() *fakeRDS {
	return &fakeRDS{
		pages: [][]types.DBCluster{
			{
				cluster("app-pg13", "aurora-postgresql", "13.15", 1, 2, "Env=prod"),
				cluster("app-pg16", "aurora-postgresql", "16.4", 1, 0, "Env=dev"),
			},
			{cluster("data-my", "aurora-mysql", "8.0.mysql_aurora.3.05.2", 2, 1, "Env=prod", "Team=data")},
		},
		groups: []types.DBSubnetGroup{
			{DBSubnetGroupName: aws.String("sg-1"), VpcId: aws.String("vpc-1")},
			{DBSubnetGroupName: aws.String("sg-2"), VpcId: aws.String("vpc-2")},
		},
	}
}

// run collects with filters and returns the result set.
func run(t *testing.T, client *fakeRDS, filters map[string][]string, sortField string, regex bool) *Results {
	t.Helper()
	r := New("default", "us-east-1", filters, sortField, regex)
	local, err := searchRDS.NewLocalFilters(filters, regex)
	if err != nil {
		t.Fatalf("NewLocalFilters(%v) error = %v", filters, err)
	}
	r.collect(context.Background(), client, local)
	return r
}

// ids returns the IDs of the rows.
func ids(r *Results) []string {
	got := []string{}
	for i := range r.Data {
		got = append(got, r.Data[i].ID)
	}
	return got
}

// TestResults_collect_filters checks the AWS filters in the request and the filters awss matches:
// name glob and regex, version glob, tags with AND and OR.
func TestResults_collect_filters(t *testing.T) {
	tests := []struct {
		name       string
		filters    map[string][]string
		regex      bool
		want       []string
		wantFilter []types.Filter
	}{
		{name: "no filter, every page", filters: map[string][]string{}, want: []string{"app-pg13", "app-pg16", "data-my"}},
		{
			name: "AWS filters in the request", filters: map[string][]string{"db-cluster-id": {"a"}, "engine": {"aurora-mysql"}},
			want: []string{"app-pg13", "app-pg16", "data-my"},
			wantFilter: []types.Filter{
				{Name: aws.String("db-cluster-id"), Values: []string{"a"}},
				{Name: aws.String("engine"), Values: []string{"aurora-mysql"}},
			},
		},
		{name: "name glob", filters: map[string][]string{"name": {"app-*"}}, want: []string{"app-pg13", "app-pg16"}},
		{name: "name regex", filters: map[string][]string{"name": {"my$"}}, regex: true, want: []string{"data-my"}},
		{name: "version glob", filters: map[string][]string{"engine-version": {"8.0.*"}}, want: []string{"data-my"}},
		{name: "tags AND", filters: map[string][]string{"tag": {"Env=prod", "Team=data"}}, want: []string{"data-my"}},
		{name: "tag values OR", filters: map[string][]string{"tag": {"Env=dev:prod"}},
			want: []string{"app-pg13", "app-pg16", "data-my"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakeClient()
			r := run(t, client, tt.filters, "id", tt.regex)

			if got := ids(r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) IDs = %v, want %v", tt.filters, got, tt.want)
			}
			if got := client.inputs[0].Filters; !reflect.DeepEqual(got, tt.wantFilter) {
				t.Errorf("collect(%v) request filters = %v, want %v", tt.filters, got, tt.wantFilter)
			}
		})
	}
}

// TestResults_collect_pageSize checks that every DescribeDBClusters and DescribeDBSubnetGroups
// call asks for pageSize records.
func TestResults_collect_pageSize(t *testing.T) {
	client := fakeClient()
	run(t, client, map[string][]string{}, "", false)

	if len(client.inputs) != 2 || len(client.groupReqs) != 2 {
		t.Fatalf("calls = %d clusters, %d subnet groups, want 2 and 2", len(client.inputs), len(client.groupReqs))
	}
	for i := range 2 {
		if aws.ToInt32(client.inputs[i].MaxRecords) != pageSize || aws.ToInt32(client.groupReqs[i].MaxRecords) != pageSize {
			t.Errorf("call %d MaxRecords = %v and %v, want %d", i,
				aws.ToInt32(client.inputs[i].MaxRecords), aws.ToInt32(client.groupReqs[i].MaxRecords), pageSize)
		}
	}
}

// TestResults_collect_vpc checks the subnet-group join: the VPC of each cluster, a group not
// found, a failed call that keeps the rows, and no subnet-group call without a cluster.
func TestResults_collect_vpc(t *testing.T) {
	tests := []struct {
		name       string
		client     func() *fakeRDS
		filters    map[string][]string
		wantVPCs   []string
		wantErrors []string
		wantCalls  int
	}{
		{name: "joined", client: fakeClient, wantVPCs: []string{"vpc-1", "vpc-1", "vpc-2"}, wantCalls: 2},
		{
			name: "group not found",
			client: func() *fakeRDS {
				c := fakeClient()
				c.groups = c.groups[:1]
				return c
			},
			wantVPCs: []string{"vpc-1", "vpc-1", ""}, wantCalls: 1,
		},
		{
			name: "failed call keeps the rows",
			client: func() *fakeRDS {
				c := fakeClient()
				c.groupErr = &smithy.GenericAPIError{Code: "AccessDenied", Message: "no"}
				return c
			},
			wantVPCs:   []string{"", "", ""},
			wantErrors: []string{"error describing DB subnet groups, VPC left empty: api error AccessDenied: no"},
			wantCalls:  1,
		},
		{name: "no cluster, no call", client: fakeClient, filters: map[string][]string{"name": {"none"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.client()
			if tt.filters == nil {
				tt.filters = map[string][]string{}
			}
			r := run(t, client, tt.filters, "id", false)

			var got []string
			for i := range r.Data {
				got = append(got, r.Data[i].VpcID)
			}
			if !reflect.DeepEqual(got, tt.wantVPCs) {
				t.Errorf("collect() VPCs = %v, want %v", got, tt.wantVPCs)
			}
			if !reflect.DeepEqual(r.Errors, append([]string{}, tt.wantErrors...)) {
				t.Errorf("collect() errors = %q, want %q", r.Errors, tt.wantErrors)
			}
			if len(client.groupReqs) != tt.wantCalls {
				t.Errorf("DescribeDBSubnetGroups calls = %d, want %d", len(client.groupReqs), tt.wantCalls)
			}
		})
	}
}

// TestResults_collect_errors checks that a DescribeDBClusters error and a bad sort field are
// recorded in the result set.
func TestResults_collect_errors(t *testing.T) {
	failing := fakeClient()
	failing.err = &smithy.GenericAPIError{Code: "AccessDenied", Message: "no"}
	tests := []struct {
		name      string
		client    *fakeRDS
		sortField string
		wantLen   int
		wantErr   string
	}{
		{name: "API error", client: failing, wantErr: "error describing DB clusters: api error AccessDenied: no"},
		{name: "bad sort field keeps the rows", client: fakeClient(), sortField: "port", wantLen: 3,
			wantErr: "invalid sort field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, tt.client, map[string][]string{}, tt.sortField, false)
			if r.Len() != tt.wantLen {
				t.Errorf("collect() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], tt.wantErr) {
				t.Errorf("collect() errors = %q, want one containing %q", r.Errors, tt.wantErr)
			}
		})
	}
}

// TestParseCluster checks the row of a cluster: every field, the writer and reader counts, a
// missing reader endpoint and nil fields.
func TestParseCluster(t *testing.T) {
	full := cluster("app", "aurora-postgresql", "16.4", 1, 2, "Env=prod")
	full.Endpoint = aws.String("app.cluster-x.rds.amazonaws.com")
	full.ReaderEndpoint = aws.String("app.cluster-ro-x.rds.amazonaws.com")
	full.Port = aws.Int32(5432)
	full.MultiAZ = aws.Bool(true)
	full.StorageEncrypted = aws.Bool(true)
	full.DeletionProtection = aws.Bool(false)

	tests := []struct {
		name string
		c    types.DBCluster
		want dataRow
	}{
		{
			name: "every field",
			c:    full,
			want: dataRow{
				ID: "app", Engine: "aurora-postgresql", EngineVersion: "16.4", Status: "available",
				MembersSummary: "1 writer, 2 readers",
				Members:        []member{{ID: "app-w", Writer: true}, {ID: "app-r0"}, {ID: "app-r1"}},
				WriterEndpoint: "app.cluster-x.rds.amazonaws.com", ReaderEndpoint: "app.cluster-ro-x.rds.amazonaws.com",
				Port: 5432, MultiAZ: "true", SubnetGroup: "sg-1", Encrypted: "true", DeletionProtection: "false",
				Tags: map[string]string{"Env": "prod"},
			},
		},
		{
			name: "one reader, no reader endpoint",
			c:    cluster("solo", "aurora-mysql", "8.0", 1, 1),
			want: dataRow{
				ID: "solo", Engine: "aurora-mysql", EngineVersion: "8.0", Status: "available",
				MembersSummary: "1 writer, 1 reader", Members: []member{{ID: "solo-w", Writer: true}, {ID: "solo-r0"}},
				SubnetGroup: "sg-1", Tags: map[string]string{},
			},
		},
		{
			name: "nil fields, no members",
			c:    types.DBCluster{DBClusterMembers: []types.DBClusterMember{{}}},
			want: dataRow{MembersSummary: "1 reader", Members: []member{{}}, Tags: map[string]string{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCluster(&tt.c); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCluster() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestMembersSummary checks the writer and reader counts of the table.
func TestMembersSummary(t *testing.T) {
	tests := []struct {
		name             string
		writers, readers int
		want             string
	}{
		{name: "no instance", want: ""},
		{name: "writer only", writers: 1, want: "1 writer"},
		{name: "writer and readers", writers: 1, readers: 2, want: "1 writer, 2 readers"},
		{name: "readers only", readers: 3, want: "3 readers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := membersSummary(tt.writers, tt.readers); got != tt.want {
				t.Errorf("membersSummary(%d, %d) = %q, want %q", tt.writers, tt.readers, got, tt.want)
			}
		})
	}
}

// TestGetSortFields checks the sort fields and the natural version order.
func TestGetSortFields(t *testing.T) {
	want := []string{"engine", "id", "status", "version", "vpc"}
	if got := SortFieldNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
	if _, err := GetSortFields("port"); err == nil {
		t.Error("GetSortFields(port) error = nil, want invalid sort field")
	}
	r := run(t, fakeClient(), map[string][]string{}, "version", false)
	if got, want := ids(r), []string{"data-my", "app-pg13", "app-pg16"}; !reflect.DeepEqual(got, want) {
		t.Errorf("sorted by version = %v, want %v", got, want)
	}
}

// TestResults_accessors checks Len, the BaseResults getters and the table headers.
func TestResults_accessors(t *testing.T) {
	r := New("default", "us-east-1", nil, "id", false)
	r.Data = []dataRow{{ID: "a"}, {ID: "b"}}
	r.AddError("e")
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "default" || r.GetRegion() != "us-east-1" || r.GetSortField() != "id" {
		t.Errorf("getters = %q %q %q, want default us-east-1 id", r.GetProfile(), r.GetRegion(), r.GetSortField())
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"e"}) {
		t.Errorf("GetErrors() = %q, want [e]", got)
	}
	// The table stays narrow: the endpoints, port, Multi-AZ, encryption and protection are JSON only.
	wantHeaders := []interface{}{"ID", "Engine", "Version", "Status", "Members", "VPC", "Tags"}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders() = %v, want %v", got, wantHeaders)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestResults_Search_earlyErrors checks the errors Search reports before any AWS call.
func TestResults_Search_earlyErrors(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	tests := []struct {
		name       string
		filters    map[string][]string
		wantPrefix string
	}{
		{name: "invalid tag filter", filters: map[string][]string{"tag": {"Env"}}, wantPrefix: "error building filters:"},
		{name: "unknown profile", filters: map[string][]string{}, wantPrefix: "error getting aws config:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("awss-test-missing-profile", "us-east-1", tt.filters, "", false)
			r.Search(context.Background())
			if len(r.Errors) != 1 || !strings.HasPrefix(r.Errors[0], tt.wantPrefix) {
				t.Errorf("Search() errors = %q, want one starting with %q", r.Errors, tt.wantPrefix)
			}
		})
	}
}
