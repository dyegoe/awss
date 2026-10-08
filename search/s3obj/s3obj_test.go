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

package s3obj

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeS3 serves buckets per region and object pages per bucket, recording the inputs.
type fakeS3 struct {
	bucketsInRegion map[string][]string         // region -> bucket names
	objects         map[string][][]types.Object // bucket -> pages
	bucketsErr      error
	objectsErr      map[string]error // bucket -> error

	listBucketsInputs []*s3.ListBucketsInput
	listObjectsInputs []*s3.ListObjectsV2Input
}

func (f *fakeS3) ListBuckets(
	_ context.Context, in *s3.ListBucketsInput, _ ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	f.listBucketsInputs = append(f.listBucketsInputs, in)
	if f.bucketsErr != nil {
		return nil, f.bucketsErr
	}
	out := &s3.ListBucketsOutput{}
	for _, name := range f.bucketsInRegion[aws.ToString(in.BucketRegion)] {
		out.Buckets = append(out.Buckets, types.Bucket{Name: aws.String(name)})
	}
	return out, nil
}

func (f *fakeS3) ListObjectsV2(
	_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	f.listObjectsInputs = append(f.listObjectsInputs, in)
	bucket := aws.ToString(in.Bucket)
	if err := f.objectsErr[bucket]; err != nil {
		return nil, err
	}
	page := 0
	if in.ContinuationToken != nil {
		page = int((*in.ContinuationToken)[0] - '0')
	}
	pages := f.objects[bucket]
	out := &s3.ListObjectsV2Output{}
	if page < len(pages) {
		out.Contents = pages[page]
	}
	if page < len(pages)-1 {
		truncated := true
		out.IsTruncated = &truncated
		out.NextContinuationToken = aws.String(string(rune('0' + page + 1)))
	}
	return out, nil
}

func obj(key string, size int64) types.Object {
	return types.Object{Key: aws.String(key), Size: &size, StorageClass: types.ObjectStorageClassStandard}
}

func keys(rows []dataRow) []string {
	out := make([]string, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].Bucket+"/"+rows[i].Key)
	}
	return out
}

// TestNew tests New, including the MaxKeys and MaxBuckets defaults.
func TestNew(t *testing.T) {
	filters := map[string][]string{FilterKeyBucket: {"b"}, FilterKeyKey: {"*.gz"}}
	got := New("default", "us-east-1", filters, "key", true, 0, 0)
	if got.MaxKeys != DefaultMaxKeys {
		t.Errorf("New(maxKeys=0).MaxKeys = %d, want %d", got.MaxKeys, DefaultMaxKeys)
	}
	if !got.Regex || got.GetSortField() != "key" || got.GetProfile() != "default" || got.GetRegion() != "us-east-1" {
		t.Errorf("New() did not keep its arguments: %#v", got)
	}
	if got := New("p", "r", filters, "", false, 5, 0).MaxKeys; got != 5 {
		t.Errorf("New(maxKeys=5).MaxKeys = %d, want 5", got)
	}
	if got.MaxBuckets != DefaultMaxBuckets {
		t.Errorf("New(maxBuckets=0).MaxBuckets = %d, want %d", got.MaxBuckets, DefaultMaxBuckets)
	}
	if got := New("p", "r", filters, "", false, 0, 7).MaxBuckets; got != 7 {
		t.Errorf("New(maxBuckets=7).MaxBuckets = %d, want 7", got)
	}
}

// TestResults_accessors tests Len, GetHeaders, GetRows.
func TestResults_accessors(t *testing.T) {
	r := New("default", "us-east-1", map[string][]string{}, "key", false, 0, 0)
	r.Data = []dataRow{{Key: "a"}, {Key: "b"}}
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	wantHeaders := []interface{}{"Bucket", "Key", "Size (bytes)", "Modified", "Class"}
	if got := r.GetHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Errorf("GetHeaders() = %#v, want %#v", got, wantHeaders)
	}
	if got := r.GetRows(); len(got) != 2 {
		t.Errorf("GetRows() = %#v, want 2 rows", got)
	}
}

// TestParseObject tests parseObject with populated and nil fields.
func TestParseObject(t *testing.T) {
	modified := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	size := int64(42)
	full := types.Object{
		Key: aws.String("logs/a.gz"), Size: &size, LastModified: &modified,
		StorageClass: types.ObjectStorageClassGlacier,
	}
	want := dataRow{Bucket: "b", Key: "logs/a.gz", Size: 42, LastModified: "2024-05-06T07:08:09Z", StorageClass: "GLACIER"}
	if got := parseObject("b", &full); !reflect.DeepEqual(got, want) {
		t.Errorf("parseObject() = %#v, want %#v", got, want)
	}
	if got := parseObject("b", &types.Object{}); !reflect.DeepEqual(got, dataRow{Bucket: "b"}) {
		t.Errorf("parseObject(nil fields) = %#v, want only Bucket set", got)
	}
}

// patternBuckets is a fake with four buckets in us-east-1 and one in eu-west-1.
func patternBuckets() *fakeS3 {
	return &fakeS3{bucketsInRegion: map[string][]string{
		"us-east-1": {"prod-logs-b", "prod-data", "dev-logs", "prod-logs-a"},
		"eu-west-1": {"prod-logs-eu"},
	}}
}

// TestResults_bucketsInRegion checks which buckets of the region the bucket patterns select:
// bucketsCase is one table entry of TestResults_bucketsInRegion.
type bucketsCase struct {
	name       string
	buckets    []string
	regex      bool
	maxBuckets int
	want       []string
	wantErr    string
}

// bucketsCases lists the bucket selections of TestResults_bucketsInRegion.
func bucketsCases() []bucketsCase {
	return []bucketsCase{
		{
			name:    "exact names: missing and other-region names are skipped",
			buckets: []string{"prod-data", "prod-logs-eu", "missing"},
			want:    []string{"prod-data"},
		},
		{name: "glob star", buckets: []string{"prod-logs-*"}, want: []string{"prod-logs-a", "prod-logs-b"}},
		{name: "glob question mark", buckets: []string{"prod-logs-?"}, want: []string{"prod-logs-a", "prod-logs-b"}},
		{
			name:    "several patterns, a bucket matched twice is listed once",
			buckets: []string{"prod-*", "*-logs-a"},
			want:    []string{"prod-data", "prod-logs-a", "prod-logs-b"},
		},
		{
			name:    "bucket patterns stay globs when --regex is set for the keys",
			buckets: []string{"prod-logs-.*"},
			regex:   true,
		},
		{name: "no match", buckets: []string{"staging-*"}},
		{
			name: "exactly at the cap", buckets: []string{"prod-logs-*"}, maxBuckets: 2,
			want: []string{"prod-logs-a", "prod-logs-b"},
		},
		{
			name: "above the cap", buckets: []string{"*"}, maxBuckets: 3,
			wantErr: "the bucket patterns match 4 buckets in us-east-1, more than --max-buckets 3: " +
				"narrow the patterns or raise --max-buckets",
		},
		{name: "invalid pattern", buckets: []string{"[unclosed"}, wantErr: "invalid bucket pattern"},
		{name: "no bucket filter", wantErr: `no bucket given: use the "bucket" filter`},
	}
}

// exact names, globs (never regexes), the --max-buckets cap, and the errors.
func TestResults_bucketsInRegion(t *testing.T) {
	for _, tt := range bucketsCases() {
		t.Run(tt.name, func(t *testing.T) {
			client := patternBuckets()
			filters := map[string][]string{}
			if tt.buckets != nil {
				filters[FilterKeyBucket] = tt.buckets
			}
			r := New("default", "us-east-1", filters, "", tt.regex, 0, tt.maxBuckets)

			got, err := r.bucketsInRegion(context.Background(), client)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("bucketsInRegion(%v) error = %v, want %q", tt.buckets, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("bucketsInRegion(%v) error = %v", tt.buckets, err)
			}
			if len(got) != 0 || len(tt.want) != 0 {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("bucketsInRegion(%v) = %v, want %v", tt.buckets, got, tt.want)
				}
			}
			if region := aws.ToString(client.listBucketsInputs[0].BucketRegion); region != "us-east-1" {
				t.Errorf("ListBuckets BucketRegion = %q, want us-east-1", region)
			}
		})
	}
}

// TestResults_bucketsInRegion_listError checks that a ListBuckets error is wrapped and returned.
func TestResults_bucketsInRegion_listError(t *testing.T) {
	client := patternBuckets()
	client.bucketsErr = errors.New("denied")
	r := New("default", "us-east-1", map[string][]string{FilterKeyBucket: {"prod-*"}}, "", false, 0, 0)

	if _, err := r.bucketsInRegion(context.Background(), client); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Errorf("bucketsInRegion() error = %v, want the wrapped ListBuckets error", err)
	}
}

// TestResults_collect_tooManyBuckets checks that a region whose bucket patterns match more than
// --max-buckets buckets reports it and lists no object at all, rather than a partial set.
func TestResults_collect_tooManyBuckets(t *testing.T) {
	client := patternBuckets()
	r := New("default", "us-east-1", map[string][]string{FilterKeyBucket: {"*"}}, "", false, 0, 3)

	matcher, err := common.NewMatcher(nil, false)
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}

	r.collect(context.Background(), client, matcher)

	if len(client.listObjectsInputs) != 0 {
		t.Errorf("ListObjectsV2 calls = %d, want 0", len(client.listObjectsInputs))
	}
	if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], "match 4 buckets in us-east-1") {
		t.Errorf("collect() errors = %q, want the --max-buckets error", r.Errors)
	}
}

// collectCase is one table entry of TestResults_collect.
type collectCase struct {
	name       string
	buckets    []string
	patterns   []string
	regex      bool
	maxKeys    int
	client     *fakeS3
	wantKeys   []string
	wantErrs   []string
	wantPrefix string
}

// twoBuckets is a fake with b1 (two pages) and b2 in us-east-1 and b3 in eu-west-1.
func twoBuckets() *fakeS3 {
	return &fakeS3{
		bucketsInRegion: map[string][]string{"us-east-1": {"b1", "b2"}, "eu-west-1": {"b3"}},
		objects: map[string][][]types.Object{
			"b1": {{obj("logs/a.gz", 1), obj("logs/b.txt", 2)}, {obj("logs/c.gz", 3)}},
			"b2": {{obj("data/x.gz", 4)}},
			"b3": {{obj("should-not-be-listed", 0)}},
		},
	}
}

// failingBuckets returns twoBuckets with the given errors injected.
func failingBuckets(bucketsErr error, objectsErr map[string]error) *fakeS3 {
	c := twoBuckets()
	c.bucketsErr = bucketsErr
	c.objectsErr = objectsErr
	return c
}

func collectCases() []collectCase {
	return []collectCase{
		{
			name:     "no pattern lists every key of the region's buckets across pages",
			buckets:  []string{"b1", "b2", "b3"},
			client:   twoBuckets(),
			wantKeys: []string{"b1/logs/a.gz", "b1/logs/b.txt", "b1/logs/c.gz", "b2/data/x.gz"},
		},
		{
			name:       "glob filters keys and narrows with the literal prefix",
			buckets:    []string{"b1", "b2"},
			patterns:   []string{"logs/*.gz"},
			client:     twoBuckets(),
			wantKeys:   []string{"b1/logs/a.gz", "b1/logs/c.gz"},
			wantPrefix: "logs/",
		},
		{
			name:       "regex anchored uses prefix and matches",
			buckets:    []string{"b1"},
			patterns:   []string{`^logs/.*\.gz$`},
			regex:      true,
			client:     twoBuckets(),
			wantKeys:   []string{"b1/logs/a.gz", "b1/logs/c.gz"},
			wantPrefix: "logs/",
		},
		{
			name:     "max-keys stops the bucket, keeps rows so far and reports it",
			buckets:  []string{"b1", "b2"},
			maxKeys:  2,
			client:   twoBuckets(),
			wantKeys: []string{"b1/logs/a.gz", "b1/logs/b.txt", "b2/data/x.gz"},
			wantErrs: []string{"bucket b1: stopped after scanning 2 keys; refine the key pattern or raise --max-keys"},
		},
		{
			name:     "one bucket failing does not stop the others",
			buckets:  []string{"b1", "b2"},
			client:   failingBuckets(nil, map[string]error{"b1": errors.New("access denied")}),
			wantKeys: []string{"b2/data/x.gz"},
			wantErrs: []string{"bucket b1: error listing objects: access denied"},
		},
		{
			name:     "bucket listing error stops everything",
			buckets:  []string{"b1"},
			client:   failingBuckets(errors.New("denied"), nil),
			wantKeys: []string{},
			wantErrs: []string{"error listing buckets: denied"},
		},
	}
}

// TestResults_collect tests bucket selection, pagination, matching, prefix narrowing, limits and errors.
func TestResults_collect(t *testing.T) {
	for _, tt := range collectCases() {
		t.Run(tt.name, func(t *testing.T) {
			filters := map[string][]string{FilterKeyBucket: tt.buckets, FilterKeyKey: tt.patterns}
			r := New("default", "us-east-1", filters, "", tt.regex, tt.maxKeys, 0)
			matcher, err := common.NewMatcher(tt.patterns, tt.regex)
			if err != nil {
				t.Fatalf("NewMatcher() error = %v", err)
			}

			r.collect(context.Background(), tt.client, matcher)

			if got := keys(r.Data); !reflect.DeepEqual(got, tt.wantKeys) {
				t.Errorf("collect() keys = %v, want %v", got, tt.wantKeys)
			}
			if tt.wantErrs == nil {
				tt.wantErrs = []string{}
			}
			if !reflect.DeepEqual(r.Errors, tt.wantErrs) {
				t.Errorf("collect() errors = %v, want %v", r.Errors, tt.wantErrs)
			}
			if len(tt.client.listObjectsInputs) > 0 {
				if got := aws.ToString(tt.client.listObjectsInputs[0].Prefix); got != tt.wantPrefix {
					t.Errorf("ListObjectsV2 Prefix = %q, want %q", got, tt.wantPrefix)
				}
			}
		})
	}
}

// TestResults_Search_invalidPattern checks an invalid pattern is reported without calling AWS.
func TestResults_Search_invalidPattern(t *testing.T) {
	r := New("default", "us-east-1", map[string][]string{FilterKeyBucket: {"b"}, FilterKeyKey: {"[x"}}, "key", false, 0, 0)
	r.Search(context.Background())
	if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], "invalid pattern") {
		t.Errorf("Search() Errors = %v, want one invalid pattern error", r.Errors)
	}
}

// TestResults_sortResults tests sorting by key, numeric size and chronological modified.
func TestResults_sortResults(t *testing.T) {
	rows := []dataRow{
		{Key: "b", Size: 10, LastModified: "2024-01-01T00:00:00Z"},
		{Key: "a", Size: 9, LastModified: "2025-01-01T00:00:00Z"},
		{Key: "c", Size: 100, LastModified: "2023-01-01T00:00:00Z"},
	}
	tests := []struct {
		name    string
		field   string
		want    []string
		wantErr bool
	}{
		{name: "key", field: "key", want: []string{"a", "b", "c"}},
		{name: "size is numeric", field: "size", want: []string{"a", "b", "c"}},
		{name: "modified is chronological", field: "modified", want: []string{"c", "b", "a"}},
		{name: "invalid", field: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("default", "us-east-1", map[string][]string{}, "", false, 0, 0)
			r.Data = append([]dataRow(nil), rows...)
			err := r.sortResults(tt.field)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortResults(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			got := []string{r.Data[0].Key, r.Data[1].Key, r.Data[2].Key}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortResults(%q) order = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// TestGetSortFields tests GetSortFields and SortFieldNames.
func TestGetSortFields(t *testing.T) {
	want := map[string]string{
		"bucket": "Bucket", "key": "Key", "size": "Size", "modified": "LastModified", "class": "StorageClass",
	}
	got, err := GetSortFields("key")
	if err != nil {
		t.Fatalf("GetSortFields(key) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetSortFields() = %#v, want %#v", got, want)
	}
	if _, err := GetSortFields("invalid"); err == nil {
		t.Error("GetSortFields(invalid) error = nil, want error")
	}
	if names := SortFieldNames(); !reflect.DeepEqual(names, []string{"bucket", "class", "key", "modified", "size"}) {
		t.Errorf("SortFieldNames() = %v", names)
	}
}
