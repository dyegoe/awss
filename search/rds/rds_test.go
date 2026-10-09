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

package rds

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/smithy-go"
)

// fakeRDS is a fake RDS client: it returns one page per call and records the inputs.
type fakeRDS struct {
	pages  [][]types.DBInstance
	err    error
	inputs []*rds.DescribeDBInstancesInput
}

func (f *fakeRDS) DescribeDBInstances(
	_ context.Context, in *rds.DescribeDBInstancesInput, _ ...func(*rds.Options),
) (*rds.DescribeDBInstancesOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	out := &rds.DescribeDBInstancesOutput{}
	page := len(f.inputs) - 1
	if page < len(f.pages) {
		out.DBInstances = f.pages[page]
	}
	if page < len(f.pages)-1 {
		out.Marker = aws.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

// instance returns an available DB instance with the given engine, version and tags.
func instance(id, engine, version string, tags ...string) types.DBInstance {
	db := types.DBInstance{
		DBInstanceIdentifier: aws.String(id), Engine: aws.String(engine), EngineVersion: aws.String(version),
		DBInstanceClass: aws.String("db.t4g.micro"), DBInstanceStatus: aws.String("available"),
	}
	for _, kv := range tags {
		k, v, _ := strings.Cut(kv, "=")
		db.TagList = append(db.TagList, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return db
}

// fleet returns one page of instances that the filter tests pick from.
func fleet() [][]types.DBInstance {
	return [][]types.DBInstance{
		{
			instance("app-pg13", "postgres", "13.15", "Env=prod", "Team=app"),
			instance("app-pg16", "postgres", "16.4", "Env=dev", "Team=app"),
		},
		{
			instance("data-mysql", "mysql", "8.0.39", "Env=prod", "Team=data"),
			instance("legacy-pg13", "postgres", "13.4"),
		},
	}
}

// ids returns the IDs of the rows.
func ids(r *Results) []string {
	got := []string{}
	for i := range r.Data {
		got = append(got, r.Data[i].ID)
	}
	return got
}

// TestResults_collect_localFilters checks the filters awss matches on the results: name globs and
// regular expressions, engine version globs, tags with AND, OR and globs, and their combination.
func TestResults_collect_localFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		regex   bool
		want    []string
	}{
		{name: "no filter, every page", filters: map[string][]string{},
			want: []string{"app-pg13", "app-pg16", "data-mysql", "legacy-pg13"}},
		{name: "name glob", filters: map[string][]string{"name": {"app-*"}}, want: []string{"app-pg13", "app-pg16"}},
		{name: "name regex", filters: map[string][]string{"name": {"pg13$"}}, regex: true,
			want: []string{"app-pg13", "legacy-pg13"}},
		{name: "version glob", filters: map[string][]string{"engine-version": {"13*"}},
			want: []string{"app-pg13", "legacy-pg13"}},
		{name: "tag", filters: map[string][]string{"tag": {"Env=prod"}}, want: []string{"app-pg13", "data-mysql"}},
		{name: "tags AND", filters: map[string][]string{"tag": {"Env=prod", "Team=app"}}, want: []string{"app-pg13"}},
		{name: "tag values OR", filters: map[string][]string{"tag": {"Env=prod:dev"}},
			want: []string{"app-pg13", "app-pg16", "data-mysql"}},
		{name: "tag value glob", filters: map[string][]string{"tag": {"Team=da*"}}, want: []string{"data-mysql"}},
		{name: "version and tag", filters: map[string][]string{"engine-version": {"13*"}, "tag": {"Env=prod"}},
			want: []string{"app-pg13"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", tt.filters, "id", tt.regex)
			input, local, err := r.getFilters()
			if err != nil {
				t.Fatalf("getFilters() error = %v", err)
			}
			r.collect(context.Background(), &fakeRDS{pages: fleet()}, input, local)

			if got := ids(r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) IDs = %v, want %v", tt.filters, got, tt.want)
			}
		})
	}
}

// TestResults_getFilters checks that the AWS filters go into the request, that the local ones do
// not, and that the filters map is not changed.
func TestResults_getFilters(t *testing.T) {
	filters := func() map[string][]string {
		return map[string][]string{
			"db-instance-id": {"a", "b"}, "engine": {"postgres"}, "db-cluster-id": {"c1"},
			"name": {"app-*"}, "engine-version": {"13*"}, "tag": {"Env=prod"},
		}
	}
	r := New("default", "us-east-1", filters(), "", false)

	input, _, err := r.getFilters()
	if err != nil {
		t.Fatalf("getFilters() error = %v", err)
	}

	want := []types.Filter{
		{Name: aws.String("db-instance-id"), Values: []string{"a", "b"}},
		{Name: aws.String("engine"), Values: []string{"postgres"}},
		{Name: aws.String("db-cluster-id"), Values: []string{"c1"}},
	}
	if !reflect.DeepEqual(input.Filters, want) {
		t.Errorf("getFilters() Filters = %v, want %v", input.Filters, want)
	}
	if !reflect.DeepEqual(r.Filters, filters()) {
		t.Errorf("getFilters() changed the filters to %v", r.Filters)
	}
}

// TestResults_getFilters_errors checks the invalid local filters.
func TestResults_getFilters_errors(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		regex   bool
		wantErr string
	}{
		{name: "bad name glob", filters: map[string][]string{"name": {"[a"}}, wantErr: "names: invalid pattern"},
		{name: "bad name regex", filters: map[string][]string{"name": {"("}}, regex: true, wantErr: "names: invalid regular"},
		{name: "bad version glob", filters: map[string][]string{"engine-version": {"[1"}}, wantErr: "engine versions:"},
		{name: "bad tag", filters: map[string][]string{"tag": {"Env"}}, wantErr: "tags: invalid tag format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := New("default", "us-east-1", tt.filters, "", tt.regex).getFilters()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("getFilters(%v) error = %v, want it to contain %q", tt.filters, err, tt.wantErr)
			}
		})
	}
}

// TestResults_collect_pageSize checks that every DescribeDBInstances call asks for pageSize records
// and that the caller's input is not changed.
func TestResults_collect_pageSize(t *testing.T) {
	client := &fakeRDS{pages: fleet()}
	input := &rds.DescribeDBInstancesInput{}
	r := New("default", "us-east-1", nil, "", false)
	_, local, _ := r.getFilters()

	r.collect(context.Background(), client, input, local)

	if len(client.inputs) != 2 {
		t.Fatalf("DescribeDBInstances calls = %d, want 2", len(client.inputs))
	}
	for i, in := range client.inputs {
		if got := aws.ToInt32(in.MaxRecords); got != pageSize {
			t.Errorf("call %d MaxRecords = %d, want %d", i, got, pageSize)
		}
	}
	if input.MaxRecords != nil {
		t.Error("collect() changed the caller's input")
	}
}

// TestResults_collect_errors checks that an API error and a bad sort field are recorded in the
// result set.
func TestResults_collect_errors(t *testing.T) {
	tests := []struct {
		name      string
		client    *fakeRDS
		sortField string
		wantLen   int
		wantErr   string
	}{
		{
			name:    "API error",
			client:  &fakeRDS{err: &smithy.GenericAPIError{Code: "AccessDenied", Message: "no"}},
			wantErr: "error describing DB instances: api error AccessDenied: no",
		},
		{name: "bad sort field keeps the rows", client: &fakeRDS{pages: fleet()}, sortField: "size", wantLen: 4,
			wantErr: "invalid sort field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", nil, tt.sortField, false)
			input, local, _ := r.getFilters()
			r.collect(context.Background(), tt.client, input, local)
			if r.Len() != tt.wantLen {
				t.Errorf("collect() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], tt.wantErr) {
				t.Errorf("collect() errors = %q, want one containing %q", r.Errors, tt.wantErr)
			}
		})
	}
}

// TestParseInstance checks the row of a DB instance, with every field and with nil fields.
func TestParseInstance(t *testing.T) {
	full := instance("aurora-1", "aurora-postgresql", "16.4", "Env=prod")
	full.MultiAZ = aws.Bool(false)
	full.AvailabilityZone = aws.String("eu-west-1a")
	full.Endpoint = &types.Endpoint{Address: aws.String("aurora-1.xyz.rds.amazonaws.com"), Port: aws.Int32(5432)}
	full.DBSubnetGroup = &types.DBSubnetGroup{VpcId: aws.String("vpc-1")}
	full.PubliclyAccessible = aws.Bool(false)
	full.StorageEncrypted = aws.Bool(true)
	full.DBClusterIdentifier = aws.String("aurora")
	full.StorageType = aws.String("aurora")
	full.AllocatedStorage = aws.Int32(1)
	full.TagList = append(full.TagList, types.Tag{Value: aws.String("no-key")})

	tests := []struct {
		name string
		db   types.DBInstance
		want dataRow
	}{
		{
			name: "every field",
			db:   full,
			want: dataRow{
				ID: "aurora-1", Engine: "aurora-postgresql", EngineVersion: "16.4", Class: "db.t4g.micro",
				Status: "available", MultiAZ: "false", AvailabilityZone: "eu-west-1a",
				Endpoint: "aurora-1.xyz.rds.amazonaws.com:5432", VpcID: "vpc-1", Public: "false", Encrypted: "true",
				ClusterID: "aurora", StorageType: "aurora", StorageGiB: 1, Tags: map[string]string{"Env": "prod"},
			},
		},
		{
			name: "nil fields: no endpoint, no cluster, no tags",
			db:   types.DBInstance{DBInstanceIdentifier: aws.String("creating"), Endpoint: &types.Endpoint{}},
			want: dataRow{ID: "creating", Tags: map[string]string{}},
		},
		{
			name: "endpoint without a port",
			db:   types.DBInstance{Endpoint: &types.Endpoint{Address: aws.String("host")}},
			want: dataRow{Endpoint: "host", Tags: map[string]string{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseInstance(&tt.db); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInstance() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestGetSortFields checks the sort fields of rds and that the rows sort by them.
func TestGetSortFields(t *testing.T) {
	want := []string{"az", "class", "cluster", "engine", "id", "status", "version", "vpc"}
	if got := SortFieldNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
	if _, err := GetSortFields("size"); err == nil {
		t.Error("GetSortFields(size) error = nil, want invalid sort field")
	}

	r := New("default", "us-east-1", nil, "version", false)
	_, local, _ := r.getFilters()
	r.collect(context.Background(), &fakeRDS{pages: fleet()}, &rds.DescribeDBInstancesInput{}, local)
	if got, want := ids(r), []string{"data-mysql", "legacy-pg13", "app-pg13", "app-pg16"}; !reflect.DeepEqual(got, want) {
		t.Errorf("sorted by version = %v, want %v", got, want)
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
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
	if got := len(r.GetHeaders()); got != 15 {
		t.Errorf("len(GetHeaders()) = %d, want 15", got)
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
