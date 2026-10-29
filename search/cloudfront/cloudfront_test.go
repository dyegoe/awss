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

package cloudfront

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/smithy-go"
)

// fakeCloudFront is a fake CloudFront client: it returns one page of distributions per call and
// records the inputs. tags holds the tags of each distribution ARN; tagErr fails those of an ARN.
type fakeCloudFront struct {
	pages  [][]types.DistributionSummary
	err    error
	inputs []*cloudfront.ListDistributionsInput

	mu      sync.Mutex
	tags    map[string]map[string]string
	tagErr  map[string]error
	tagCall int
}

func (f *fakeCloudFront) ListDistributions(
	_ context.Context, in *cloudfront.ListDistributionsInput, _ ...func(*cloudfront.Options),
) (*cloudfront.ListDistributionsOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	page := len(f.inputs) - 1
	list := &types.DistributionList{IsTruncated: aws.Bool(page < len(f.pages)-1)}
	if page < len(f.pages) {
		list.Items = f.pages[page]
	}
	if page < len(f.pages)-1 {
		list.NextMarker = aws.String(fmt.Sprint(page + 1))
	}
	return &cloudfront.ListDistributionsOutput{DistributionList: list}, nil
}

func (f *fakeCloudFront) ListTagsForResource(
	_ context.Context, in *cloudfront.ListTagsForResourceInput, _ ...func(*cloudfront.Options),
) (*cloudfront.ListTagsForResourceOutput, error) {
	f.mu.Lock()
	f.tagCall++
	f.mu.Unlock()
	arn := aws.ToString(in.Resource)
	if err := f.tagErr[arn]; err != nil {
		return nil, err
	}
	out := &cloudfront.ListTagsForResourceOutput{Tags: &types.Tags{}}
	for k, v := range f.tags[arn] {
		out.Tags.Items = append(out.Tags.Items, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return out, nil
}

// distribution returns a deployed, enabled distribution with these aliases and origins.
func distribution(id string, aliases []string, origins ...string) types.DistributionSummary {
	d := types.DistributionSummary{
		Id:         aws.String(id),
		ARN:        aws.String("arn:aws:cloudfront::111111111111:distribution/" + id),
		DomainName: aws.String(strings.ToLower(id) + ".cloudfront.net"),
		Status:     aws.String("Deployed"),
		Enabled:    aws.Bool(true),
		Aliases:    &types.Aliases{Items: aliases},
		Origins:    &types.Origins{},
	}
	for _, o := range origins {
		d.Origins.Items = append(d.Origins.Items, types.Origin{DomainName: aws.String(o)})
	}
	return d
}

// arn returns the ARN distribution gives the distribution id.
func arn(id string) string { return "arn:aws:cloudfront::111111111111:distribution/" + id }

// accessDenied returns the error of a missing permission.
func accessDenied() error {
	return &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized"}
}

// mockClient makes newClient return client, or err, for the duration of the test.
func mockClient(t *testing.T, client *fakeCloudFront, err error) {
	t.Helper()
	old := newClient
	t.Cleanup(func() { newClient = old })
	newClient = func(string) (cloudfrontAPI, error) {
		if client == nil {
			return nil, err
		}
		return client, err
	}
}

// ids returns the IDs of the rows, in order.
func ids(rows []dataRow) []string {
	out := []string{}
	for i := range rows {
		out = append(out, rows[i].ID)
	}
	return out
}

// collectWith runs collect on a fresh result set with these filters and returns it.
func collectWith(t *testing.T, client *fakeCloudFront, filters map[string][]string, regex, showTags bool) *Results {
	t.Helper()
	local, err := newLocalFilters(filters, regex)
	if err != nil {
		t.Fatalf("newLocalFilters(%v) error = %v", filters, err)
	}
	r := New("dev", Region, filters, "id", regex, showTags)
	r.tagInterval = time.Millisecond
	r.collect(context.Background(), client, local)
	return r
}

// TestResults_collect checks the rows of every page, the page size, the sort, a page without a
// list, and that a failed call is an error naming the profile, never an empty list.
func TestResults_collect(t *testing.T) {
	tests := []struct {
		name      string
		client    *fakeCloudFront
		wantIDs   []string
		wantError string
	}{
		{
			name: "every page, sorted by id",
			client: &fakeCloudFront{pages: [][]types.DistributionSummary{
				{distribution("E3", nil)}, {}, {distribution("E1", nil), distribution("E2", nil)},
			}},
			wantIDs: []string{"E1", "E2", "E3"},
		},
		{name: "no distributions", client: &fakeCloudFront{}, wantIDs: []string{}},
		{
			name: "access denied", client: &fakeCloudFront{err: accessDenied()}, wantIDs: []string{},
			wantError: `error with profile "dev": listing the distributions`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := collectWith(t, tt.client, map[string][]string{}, false, false)
			if got := ids(r.Data); !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("collect() ids = %v, want %v", got, tt.wantIDs)
			}
			if got := strings.Join(r.Errors, ";"); !strings.HasPrefix(got, tt.wantError) || (tt.wantError == "") != (got == "") {
				t.Errorf("collect() errors = %q, want prefix %q", got, tt.wantError)
			}
			for i, in := range tt.client.inputs {
				if got := aws.ToInt32(in.MaxItems); got != pageSize {
					t.Errorf("ListDistributions() call %d MaxItems = %d, want %d", i, got, pageSize)
				}
			}
		})
	}
}

// TestResults_collect_nilList checks that a page without a DistributionList is skipped.
func TestResults_collect_nilList(t *testing.T) {
	r := New("dev", Region, map[string][]string{}, "", false, false)
	local, err := newLocalFilters(map[string][]string{}, false)
	if err != nil {
		t.Fatalf("newLocalFilters() error = %v", err)
	}
	if err := r.collectDistributions(context.Background(), nilListClient{}, local); err != nil {
		t.Fatalf("collectDistributions() error = %v", err)
	}
	if r.Len() != 0 {
		t.Errorf("collectDistributions() rows = %d, want 0", r.Len())
	}
}

// nilListClient returns one page without a DistributionList.
type nilListClient struct{}

func (nilListClient) ListDistributions(
	context.Context, *cloudfront.ListDistributionsInput, ...func(*cloudfront.Options),
) (*cloudfront.ListDistributionsOutput, error) {
	return &cloudfront.ListDistributionsOutput{}, nil
}

// TestResults_collect_filters checks the ID, domain and origin filters, with globs and regular
// expressions, and that they combine with AND.
func TestResults_collect_filters(t *testing.T) {
	client := func() *fakeCloudFront {
		return &fakeCloudFront{pages: [][]types.DistributionSummary{{
			distribution("E1", []string{"www.example.com", "example.com"}, "site.s3.amazonaws.com"),
			distribution("E2", []string{"api.example.org"}, "alb-1.eu-west-1.elb.amazonaws.com"),
			distribution("E3", nil, "site.s3.amazonaws.com", "backup.s3.amazonaws.com"),
		}}}
	}
	tests := []struct {
		name    string
		filters map[string][]string
		regex   bool
		want    []string
	}{
		{name: "no filter", filters: map[string][]string{}, want: []string{"E1", "E2", "E3"}},
		{name: "ids, exact", filters: map[string][]string{FilterKeyID: {"E2", "E3", "E"}}, want: []string{"E2", "E3"}},
		{name: "domain, an alias", filters: map[string][]string{FilterKeyDomain: {"www.example.com"}}, want: []string{"E1"}},
		{name: "domain glob", filters: map[string][]string{FilterKeyDomain: {"*.example.*"}}, want: []string{"E1", "E2"}},
		{
			name: "domain, the cloudfront.net name", filters: map[string][]string{FilterKeyDomain: {"e3.cloudfront.net"}},
			want: []string{"E3"},
		},
		{
			name: "domain regex", filters: map[string][]string{FilterKeyDomain: {`\.org$`}}, regex: true,
			want: []string{"E2"},
		},
		{
			name: "origin glob", filters: map[string][]string{FilterKeyOrigin: {"*.s3.amazonaws.com"}},
			want: []string{"E1", "E3"},
		},
		{name: "origin, a second origin", filters: map[string][]string{FilterKeyOrigin: {"backup.*"}}, want: []string{"E3"}},
		{
			name:    "domain and origin combine with AND",
			filters: map[string][]string{FilterKeyDomain: {"*.example.com"}, FilterKeyOrigin: {"*.s3.amazonaws.com"}},
			want:    []string{"E1"},
		},
		{name: "nothing matches", filters: map[string][]string{FilterKeyDomain: {"nope.example.net"}}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client()
			r := collectWith(t, c, tt.filters, tt.regex, false)
			if got := ids(r.Data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collect(%v) ids = %v, want %v", tt.filters, got, tt.want)
			}
			if c.tagCall != 0 {
				t.Errorf("collect(%v) tag calls = %d, want 0 without tags", tt.filters, c.tagCall)
			}
		})
	}
}

// TestResults_collect_tags checks that tags are fetched only with ShowTags or a tag filter, that
// a tag filter keeps the matching distributions, and that a failed tag call is reported and the
// row, whose tags are unknown, does not match a tag filter.
func TestResults_collect_tags(t *testing.T) {
	client := func() *fakeCloudFront {
		return &fakeCloudFront{
			pages: [][]types.DistributionSummary{{distribution("E1", nil), distribution("E2", nil), distribution("E3", nil)}},
			tags: map[string]map[string]string{
				arn("E1"): {"Env": "prod", "Team": "web"},
				arn("E2"): {"Env": "stg"},
			},
			tagErr: map[string]error{arn("E3"): accessDenied()},
		}
	}
	tests := []struct {
		name      string
		filters   map[string][]string
		showTags  bool
		wantIDs   []string
		wantCalls int
		wantTags  map[string]string
		wantError string
	}{
		{name: "no tags asked", filters: map[string][]string{}, wantIDs: []string{"E1", "E2", "E3"}},
		{
			name: "show tags", filters: map[string][]string{}, showTags: true, wantIDs: []string{"E1", "E2", "E3"},
			wantCalls: 3, wantTags: map[string]string{"Env": "prod", "Team": "web"},
			wantError: "error getting tags of distribution E3",
		},
		{
			name: "tag filter fetches the tags", filters: map[string][]string{FilterKeyTag: {"Env=prod:dev"}},
			wantIDs: []string{"E1"}, wantCalls: 3, wantTags: map[string]string{"Env": "prod", "Team": "web"},
			wantError: "error getting tags of distribution E3",
		},
		{
			name: "tag glob", filters: map[string][]string{FilterKeyTag: {"Env=*"}},
			wantIDs: []string{"E1", "E2"}, wantCalls: 3, wantTags: map[string]string{"Env": "prod", "Team": "web"},
			wantError: "error getting tags of distribution E3",
		},
		{
			name:    "tags only for the rows that passed the other filters",
			filters: map[string][]string{FilterKeyID: {"E2"}, FilterKeyTag: {"Env=stg"}},
			wantIDs: []string{"E2"}, wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client()
			r := collectWith(t, c, tt.filters, false, tt.showTags)
			if got := ids(r.Data); !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("collect(%v) ids = %v, want %v", tt.filters, got, tt.wantIDs)
			}
			if c.tagCall != tt.wantCalls {
				t.Errorf("collect(%v) tag calls = %d, want %d", tt.filters, c.tagCall, tt.wantCalls)
			}
			if tt.wantTags != nil && len(r.Data) > 0 && !reflect.DeepEqual(r.Data[0].Tags, tt.wantTags) {
				t.Errorf("collect(%v) tags of %s = %v, want %v", tt.filters, r.Data[0].ID, r.Data[0].Tags, tt.wantTags)
			}
			if got := strings.Join(r.Errors, ";"); !strings.HasPrefix(got, tt.wantError) || (tt.wantError == "") != (got == "") {
				t.Errorf("collect(%v) errors = %q, want prefix %q", tt.filters, got, tt.wantError)
			}
		})
	}
}

// TestDistributionTags checks that an answer without tags is an empty map and that a tag without
// a key is skipped.
func TestDistributionTags(t *testing.T) {
	tests := []struct {
		name string
		out  *cloudfront.ListTagsForResourceOutput
		want map[string]string
	}{
		{name: "nil output", out: nil, want: map[string]string{}},
		{name: "nil tags", out: &cloudfront.ListTagsForResourceOutput{}, want: map[string]string{}},
		{
			name: "tag without a key skipped",
			out: &cloudfront.ListTagsForResourceOutput{Tags: &types.Tags{Items: []types.Tag{
				{Key: aws.String("Env"), Value: aws.String("prod")}, {Value: aws.String("orphan")}, {Key: aws.String("Empty")},
			}}},
			want: map[string]string{"Env": "prod", "Empty": ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := distributionTags(context.Background(), outputClient{tt.out}, &dataRow{ID: "E1"})
			if err != nil {
				t.Fatalf("distributionTags() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("distributionTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

// outputClient answers every ListTagsForResource call with out.
type outputClient struct {
	out *cloudfront.ListTagsForResourceOutput
}

func (c outputClient) ListTagsForResource(
	context.Context, *cloudfront.ListTagsForResourceInput, ...func(*cloudfront.Options),
) (*cloudfront.ListTagsForResourceOutput, error) {
	return c.out, nil
}

// TestResults_collectTags_pace checks that the tag calls are paced, and that no new call starts
// once the context ends.
func TestResults_collectTags_pace(t *testing.T) {
	tests := []struct {
		name        string
		interval    time.Duration
		cancelled   bool
		wantCalls   int
		wantAtLeast time.Duration
	}{
		{name: "paced", interval: 30 * time.Millisecond, wantCalls: 4, wantAtLeast: 90 * time.Millisecond},
		{name: "cancelled", interval: time.Hour, cancelled: true, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeCloudFront{}
			r := New("dev", Region, nil, "", false, true)
			r.tagInterval = tt.interval
			r.Data = []dataRow{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelled {
				cancel()
			}

			start := time.Now()
			r.collectTags(ctx, client)

			if client.tagCall != tt.wantCalls {
				t.Errorf("collectTags() calls = %d, want %d", client.tagCall, tt.wantCalls)
			}
			if got := time.Since(start); got < tt.wantAtLeast {
				t.Errorf("collectTags() took %s, want at least %s", got, tt.wantAtLeast)
			}
		})
	}
}

// TestParseDistribution checks the row of a distribution, with every field and with nil fields.
func TestParseDistribution(t *testing.T) {
	full := distribution("E1ABC", []string{"www.example.com"}, "site.s3.amazonaws.com", "alb.elb.amazonaws.com")
	full.Comment = aws.String("main site")
	full.PriceClass = types.PriceClassPriceClass100
	full.HttpVersion = types.HttpVersionHttp2and3
	full.IsIPV6Enabled = aws.Bool(true)
	full.WebACLId = aws.String("arn:aws:wafv2:us-east-1:111111111111:global/webacl/site/1")
	full.Staging = aws.Bool(false)
	full.ViewerCertificate = &types.ViewerCertificate{
		ACMCertificateArn: aws.String("arn:aws:acm:us-east-1:111111111111:certificate/1"),
	}
	full.LastModifiedTime = aws.Time(time.Date(2025, 2, 3, 5, 6, 7, 0, time.FixedZone("CET", 3600)))

	iam := distribution("E2", nil)
	iam.ViewerCertificate = &types.ViewerCertificate{IAMCertificateId: aws.String("ASCA1")}

	tests := []struct {
		name string
		d    types.DistributionSummary
		want dataRow
	}{
		{
			name: "every field",
			d:    full,
			want: dataRow{
				ID: "E1ABC", Domain: "e1abc.cloudfront.net", Aliases: []string{"www.example.com"},
				Origins: []string{"site.s3.amazonaws.com", "alb.elb.amazonaws.com"}, Status: "Deployed", Enabled: "true",
				ARN: arn("E1ABC"), Comment: "main site", PriceClass: "PriceClass_100", HTTPVersion: "http2and3",
				IPv6: "true", WebACLID: "arn:aws:wafv2:us-east-1:111111111111:global/webacl/site/1", Staging: "false",
				Certificate: "arn:aws:acm:us-east-1:111111111111:certificate/1", Modified: "2025-02-03T04:06:07Z",
			},
		},
		{
			name: "IAM certificate, no aliases, no origins",
			d:    iam,
			want: dataRow{
				ID: "E2", Domain: "e2.cloudfront.net", Status: "Deployed", Enabled: "true", ARN: arn("E2"),
				Certificate: "ASCA1",
			},
		},
		{name: "nil fields", d: types.DistributionSummary{}, want: dataRow{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseDistribution(&tt.d); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDistribution() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestDataRow_jsonKeys checks the table columns, and that the JSON-only fields are JSON keys
// left out when empty.
func TestDataRow_jsonKeys(t *testing.T) {
	wantHeaders := []interface{}{"ID", "Domain", "Aliases", "Origins", "Status", "Enabled", "Tags"}
	if got := common.Headers(dataRow{}); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("common.Headers(dataRow{}) = %v, want %v", got, wantHeaders)
	}
	jsonOnly := []string{
		"arn", "comment", "price_class", "http_version", "ipv6", "web_acl_id", "certificate", "staging", "modified",
	}
	tests := []struct {
		name string
		row  dataRow
		want bool
	}{
		{
			name: "set",
			row: dataRow{
				ARN: "a", Comment: "c", PriceClass: "p", HTTPVersion: "h", IPv6: "true", WebACLID: "w",
				Certificate: "c", Staging: "false", Modified: "m",
			},
			want: true,
		},
		{name: "empty", row: dataRow{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.row)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			keys := map[string]any{}
			if err := json.Unmarshal(b, &keys); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			for _, key := range jsonOnly {
				if _, got := keys[key]; got != tt.want {
					t.Errorf("json.Marshal(%+v) has %q = %v, want %v", tt.row, key, got, tt.want)
				}
			}
		})
	}
}

// TestNewLocalFilters_errors checks that an invalid domain, origin or tag filter is an error
// naming the filter.
func TestNewLocalFilters_errors(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string][]string
		regex   bool
		want    string
	}{
		{name: "domain glob", filters: map[string][]string{FilterKeyDomain: {"[bad"}}, want: "domains"},
		{name: "origin regex", filters: map[string][]string{FilterKeyOrigin: {"(bad"}}, regex: true, want: "origins"},
		{name: "tag", filters: map[string][]string{FilterKeyTag: {"no-value"}}, want: "tags"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newLocalFilters(tt.filters, tt.regex)
			if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
				t.Errorf("newLocalFilters(%v) error = %v, want prefix %q", tt.filters, err, tt.want)
			}
		})
	}
}

// TestGetSortFields checks the sort fields of cloudfront and that the rows sort by them.
func TestGetSortFields(t *testing.T) {
	if got, want := SortFieldNames(), []string{"domain", "id", "modified", "status"}; !slices.Equal(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
	if _, err := GetSortFields("aliases"); err == nil {
		t.Error("GetSortFields(aliases) error = nil, want an error")
	}
	r := New("dev", Region, nil, "", false, false)
	r.Data = []dataRow{{ID: "E1", Modified: "2025-03-01T00:00:00Z"}, {ID: "E2", Modified: "2024-01-01T00:00:00Z"}}
	if err := r.sortResults("modified"); err != nil {
		t.Fatalf("sortResults(modified) error = %v", err)
	}
	if got := ids(r.Data); !slices.Equal(got, []string{"E2", "E1"}) {
		t.Errorf("sortResults(modified) ids = %v, want [E2 E1]", got)
	}
	if err := r.sortResults("nope"); err == nil {
		t.Error("sortResults(nope) error = nil, want an error")
	}
}

// TestResults_collect_badSortField checks that an unknown sort field is an error in the result set.
func TestResults_collect_badSortField(t *testing.T) {
	local, err := newLocalFilters(map[string][]string{}, false)
	if err != nil {
		t.Fatalf("newLocalFilters() error = %v", err)
	}
	r := New("dev", Region, map[string][]string{}, "nope", false, false)
	r.collect(context.Background(), &fakeCloudFront{}, local)
	if len(r.Errors) != 1 {
		t.Errorf("collect() errors = %q, want one sort error", r.Errors)
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("dev", Region, nil, "id", false, false)
	r.Data = []dataRow{{ID: "1"}, {ID: "2"}}
	r.AddError("e")
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "dev" || r.GetRegion() != Region || r.GetSortField() != "id" {
		t.Errorf("getters = %q %q %q, want dev %s id", r.GetProfile(), r.GetRegion(), r.GetSortField(), Region)
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"e"}) {
		t.Errorf("GetErrors() = %q, want [e]", got)
	}
	if got := len(r.GetHeaders()); got != 7 {
		t.Errorf("len(GetHeaders()) = %d, want 7", got)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestResults_Search checks that Search lists through the client of the profile, and reports an
// invalid filter or a client that cannot be built.
func TestResults_Search(t *testing.T) {
	tests := []struct {
		name      string
		filters   map[string][]string
		client    *fakeCloudFront
		clientErr error
		wantLen   int
		wantError string
	}{
		{
			name: "lists the distributions", filters: map[string][]string{},
			client: &fakeCloudFront{pages: [][]types.DistributionSummary{{distribution("E1", nil)}}}, wantLen: 1,
		},
		{
			name: "bad filter", filters: map[string][]string{FilterKeyDomain: {"[bad"}}, client: &fakeCloudFront{},
			wantError: "error building filters: domains",
		},
		{
			name: "no client", filters: map[string][]string{}, clientErr: errors.New("no profile"),
			wantError: "error getting aws config: no profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient(t, tt.client, tt.clientErr)
			r := New("dev", Region, tt.filters, "", false, false)
			r.Search(context.Background())
			if r.Len() != tt.wantLen {
				t.Errorf("Search() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			if got := strings.Join(r.Errors, ";"); !strings.HasPrefix(got, tt.wantError) || (tt.wantError == "") != (got == "") {
				t.Errorf("Search() errors = %q, want prefix %q", got, tt.wantError)
			}
		})
	}
}

// TestNewClient checks that a profile missing from the AWS config file is an error before any
// call, and that the client tries a throttled call maxAttempts times.
func TestNewClient(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	if _, err := newClient("awss-test-missing-profile"); err == nil {
		t.Error("newClient(missing profile) error = nil, want an error")
	}
	client, err := newClient("")
	if err != nil {
		t.Fatalf("newClient(default) error = %v, want nil", err)
	}
	if got := client.(*cloudfront.Client).Options().Retryer.MaxAttempts(); got != maxAttempts {
		t.Errorf("newClient() MaxAttempts = %d, want %d", got, maxAttempts)
	}
}
