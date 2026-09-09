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

package s3

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeListBuckets is a ListBucketsAPIClient that serves the given pages and records the inputs.
type fakeListBuckets struct {
	pages  [][]types.Bucket
	err    error
	inputs []*s3.ListBucketsInput
}

func (f *fakeListBuckets) ListBuckets(
	_ context.Context, in *s3.ListBucketsInput, _ ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	page := len(f.inputs) - 1
	out := &s3.ListBucketsOutput{Buckets: f.pages[page]}
	if page < len(f.pages)-1 {
		out.ContinuationToken = common.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

func bucket(name string) types.Bucket {
	return types.Bucket{Name: common.String(name), BucketRegion: common.String("us-east-1")}
}

// TestNew tests the New function.
func TestNew(t *testing.T) {
	filters := map[string][]string{FilterKeyName: {"prod-*"}}
	want := &Results{
		BaseResults: common.BaseResults{
			Profile:   "default",
			Region:    "us-east-1",
			Errors:    []string{},
			SortField: "name",
		},
		Data:    []dataRow{},
		Filters: filters,
		Regex:   true,
	}
	if got := New("default", "us-east-1", filters, "name", true); !reflect.DeepEqual(got, want) {
		t.Errorf("New()\n%#v\nwant\n%#v", got, want)
	}
}

// TestResults_accessors tests Len, GetHeaders, GetRows and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("default", "us-east-1", map[string][]string{}, "name", false)
	r.Data = []dataRow{{Name: "a"}, {Name: "b"}}
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "default" || r.GetRegion() != "us-east-1" || r.GetSortField() != "name" {
		t.Errorf("getters = %q %q %q, want default us-east-1 name", r.GetProfile(), r.GetRegion(), r.GetSortField())
	}
	wantHeaders := []interface{}{"Name", "Region", "Created", "ARN"}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders() = %#v, want %#v", got, wantHeaders)
	}
	wantRows := []interface{}{dataRow{Name: "a"}, dataRow{Name: "b"}}
	if got := r.GetRows(); !reflect.DeepEqual(got, wantRows) {
		t.Errorf("GetRows() = %#v, want %#v", got, wantRows)
	}
}

// TestParseBucket tests parseBucket with populated and nil fields.
func TestParseBucket(t *testing.T) {
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.FixedZone("CET", 3600))
	tests := []struct {
		name   string
		bucket types.Bucket
		want   dataRow
	}{
		{
			name: "all fields, date in UTC",
			bucket: types.Bucket{
				Name:         common.String("prod-logs"),
				BucketRegion: common.String("eu-west-1"),
				BucketArn:    common.String("arn:aws:s3:::prod-logs"),
				CreationDate: &created,
			},
			want: dataRow{
				Name: "prod-logs", Region: "eu-west-1", ARN: "arn:aws:s3:::prod-logs",
				CreationDate: "2024-01-02T02:04:05Z",
			},
		},
		{name: "nil pointers", bucket: types.Bucket{}, want: dataRow{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseBucket(&tt.bucket); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseBucket() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// collectCase is one table entry of TestResults_collectBuckets.
type collectCase struct {
	name       string
	patterns   []string
	regex      bool
	pages      [][]types.Bucket
	err        error
	wantNames  []string
	wantPrefix string
	wantErr    string
}

func collectCases() []collectCase {
	return []collectCase{
		{
			name:      "no patterns keeps every bucket across pages",
			pages:     [][]types.Bucket{{bucket("a"), bucket("b")}, {bucket("c")}},
			wantNames: []string{"a", "b", "c"},
		},
		{
			name:       "glob filters client-side and narrows with the literal prefix",
			patterns:   []string{"prod-*"},
			pages:      [][]types.Bucket{{bucket("prod-logs"), bucket("prod-x"), bucket("production")}},
			wantNames:  []string{"prod-logs", "prod-x"},
			wantPrefix: "prod-",
		},
		{
			name:      "two globs send no prefix",
			patterns:  []string{"prod-*", "dev-*"},
			pages:     [][]types.Bucket{{bucket("prod-logs"), bucket("dev-logs"), bucket("test-logs")}},
			wantNames: []string{"prod-logs", "dev-logs"},
		},
		{
			name:       "regex matches substrings and uses the literal prefix",
			patterns:   []string{"^prod-.*logs$"},
			regex:      true,
			pages:      [][]types.Bucket{{bucket("prod-app-logs"), bucket("prod-logs-old")}},
			wantNames:  []string{"prod-app-logs"},
			wantPrefix: "prod-",
		},
		{
			name:    "api error is wrapped",
			err:     errors.New("access denied"),
			wantErr: "error listing buckets: access denied",
		},
	}
}

// TestResults_collectBuckets tests the paginated listing, client-side matching and prefix narrowing.
func TestResults_collectBuckets(t *testing.T) {
	for _, tt := range collectCases() {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", map[string][]string{FilterKeyName: tt.patterns}, "", tt.regex)
			matcher, err := r.matcher()
			if err != nil {
				t.Fatalf("matcher() error = %v", err)
			}
			client := &fakeListBuckets{pages: tt.pages, err: tt.err}

			err = r.collectBuckets(context.Background(), client, matcher)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("collectBuckets() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("collectBuckets() unexpected error: %v", err)
			}
			got := make([]string, 0, len(r.Data))
			for i := range r.Data {
				got = append(got, r.Data[i].Name)
			}
			if !reflect.DeepEqual(got, tt.wantNames) {
				t.Errorf("collectBuckets() names = %v, want %v", got, tt.wantNames)
			}
			if len(client.inputs) != len(tt.pages) {
				t.Errorf("ListBuckets called %d times, want %d (one per page)", len(client.inputs), len(tt.pages))
			}
			first := client.inputs[0]
			if common.StringValue(first.BucketRegion) != "us-east-1" {
				t.Errorf("BucketRegion = %q, want us-east-1", common.StringValue(first.BucketRegion))
			}
			if got := common.StringValue(first.Prefix); got != tt.wantPrefix {
				t.Errorf("Prefix = %q, want %q", got, tt.wantPrefix)
			}
		})
	}
}

// TestResults_Search_invalidPattern checks an invalid pattern is reported without calling AWS.
func TestResults_Search_invalidPattern(t *testing.T) {
	r := New("default", "us-east-1", map[string][]string{FilterKeyName: {"(unclosed"}}, "name", true)
	r.Search(context.Background())
	if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], "invalid regular expression") {
		t.Errorf("Search() Errors = %v, want one invalid regular expression error", r.Errors)
	}
}

// TestResults_sortResults tests sorting by name and created, and the invalid field error.
func TestResults_sortResults(t *testing.T) {
	rows := []dataRow{
		{Name: "b", CreationDate: "2024-01-01T00:00:00Z"},
		{Name: "a", CreationDate: "2025-01-01T00:00:00Z"},
		{Name: "c", CreationDate: "2023-12-31T23:59:59Z"},
	}
	tests := []struct {
		name    string
		field   string
		want    []string
		wantErr bool
	}{
		{name: "name", field: "name", want: []string{"a", "b", "c"}},
		{name: "created is chronological", field: "created", want: []string{"c", "b", "a"}},
		{name: "invalid", field: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", map[string][]string{}, "", false)
			r.Data = append([]dataRow(nil), rows...)
			err := r.sortResults(tt.field)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortResults(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			got := []string{r.Data[0].Name, r.Data[1].Name, r.Data[2].Name}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortResults(%q) order = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// TestGetSortFields tests GetSortFields and SortFieldNames.
func TestGetSortFields(t *testing.T) {
	want := map[string]string{"name": "Name", "region": "Region", "created": "CreationDate", "arn": "ARN"}
	got, err := GetSortFields("name")
	if err != nil {
		t.Fatalf("GetSortFields(name) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetSortFields() = %#v, want %#v", got, want)
	}
	if _, err := GetSortFields("invalid"); err == nil {
		t.Error("GetSortFields(invalid) error = nil, want error")
	}
	if names := SortFieldNames(); !reflect.DeepEqual(names, []string{"arn", "created", "name", "region"}) {
		t.Errorf("SortFieldNames() = %v", names)
	}
}
