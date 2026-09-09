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

// Package s3obj contains the search for S3 objects (keys) inside given buckets.
//
// It implements the common.Results interface. Each profile x region goroutine only
// scans the requested buckets that live in its region, so every key is listed once.
package s3obj

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	// FilterKeyBucket is the filters key carrying the exact bucket names to scan.
	FilterKeyBucket = "bucket"

	// FilterKeyKey is the filters key carrying the key patterns, matched client-side.
	FilterKeyKey = "key"

	// DefaultMaxKeys is the default cap of keys scanned per bucket.
	DefaultMaxKeys = 10000
)

// api is the subset of the S3 client used by the search. *s3.Client satisfies it.
type api interface {
	s3.ListBucketsAPIClient
	s3.ListObjectsV2APIClient
}

// Results describes results of the S3 objects search.
type Results struct {
	common.BaseResults

	// Data contains the objects found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`

	// Regex makes the key patterns regular expressions instead of globs.
	Regex bool `json:"-"`

	// MaxKeys caps the number of keys scanned per bucket. Zero or less means DefaultMaxKeys.
	MaxKeys int `json:"-"`
}

// dataRow represents a row of the S3 objects search results.
type dataRow struct {
	// Bucket is the bucket the object is in.
	Bucket string `json:"bucket,omitempty" header:"Bucket" sort:"bucket"`

	// Key is the object key.
	Key string `json:"key,omitempty" header:"Key" sort:"key"`

	// Size is the object size in bytes.
	Size int64 `json:"size" header:"Size (bytes)" sort:"size"`

	// LastModified is when the object was last modified, in RFC 3339 UTC so it sorts chronologically.
	LastModified string `json:"modified,omitempty" header:"Modified" sort:"modified"`

	// StorageClass is the storage class of the object.
	StorageClass string `json:"class,omitempty" header:"Class" sort:"class"`
}

// New initiates and returns a new instance of S3 object results.
func New(profile, region string, filters map[string][]string, sortField string, regex bool, maxKeys int) *Results {
	if maxKeys <= 0 {
		maxKeys = DefaultMaxKeys
	}
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data:    []dataRow{},
		Filters: filters,
		Regex:   regex,
		MaxKeys: maxKeys,
	}
}

// Search performs the S3 objects search.
//
// Results are stored in the Data field. Only the requested buckets that live in r.Region are scanned.
func (r *Results) Search(ctx context.Context) {
	matcher, err := common.NewMatcher(r.Filters[FilterKeyKey], r.Regex)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}

	r.collect(ctx, s3.NewFromConfig(cfg), matcher)

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// collect scans every requested bucket that lives in r.Region.
//
// A bucket that fails to list records an error and does not stop the other buckets.
func (r *Results) collect(ctx context.Context, client api, matcher *common.Matcher) {
	buckets, err := r.bucketsInRegion(ctx, client)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	for _, bucket := range buckets {
		if err := r.collectBucket(ctx, client, bucket, matcher); err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
	}
}

// bucketsInRegion returns the requested buckets that live in r.Region, sorted by name.
func (r *Results) bucketsInRegion(ctx context.Context, client s3.ListBucketsAPIClient) ([]string, error) {
	requested := r.Filters[FilterKeyBucket]
	if len(requested) == 0 {
		return nil, fmt.Errorf("no bucket given: use the %q filter", FilterKeyBucket)
	}

	inRegion := map[string]struct{}{}
	paginator := s3.NewListBucketsPaginator(client, &s3.ListBucketsInput{BucketRegion: common.String(r.Region)})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing buckets: %w", err)
		}
		for i := range page.Buckets {
			inRegion[common.StringValue(page.Buckets[i].Name)] = struct{}{}
		}
	}

	buckets := make([]string, 0, len(requested))
	for _, name := range requested {
		if _, ok := inRegion[name]; ok {
			buckets = append(buckets, name)
		}
	}
	sort.Strings(buckets)
	return buckets, nil
}

// collectBucket lists the keys of one bucket and appends those that match.
//
// The listing is narrowed server-side with the literal prefix of the pattern when there is one.
// It stops after r.MaxKeys keys and returns an error saying so, keeping the rows found so far.
func (r *Results) collectBucket(
	ctx context.Context, client s3.ListObjectsV2APIClient, bucket string, m *common.Matcher,
) error {
	input := &s3.ListObjectsV2Input{Bucket: common.String(bucket)}
	if prefix := m.Prefix(); prefix != "" {
		input.Prefix = common.String(prefix)
	}

	scanned := 0
	paginator := s3.NewListObjectsV2Paginator(client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("bucket %s: error listing objects: %w", bucket, err)
		}
		for i := range page.Contents {
			if scanned >= r.MaxKeys {
				return fmt.Errorf("bucket %s: stopped after scanning %d keys; refine the key pattern or raise --max-keys",
					bucket, scanned)
			}
			scanned++
			if row := parseObject(bucket, &page.Contents[i]); m.Match(row.Key) {
				r.Data = append(r.Data, row)
			}
		}
	}
	return nil
}

// parseObject converts a single Object into a dataRow.
func parseObject(bucket string, obj *types.Object) dataRow {
	row := dataRow{
		Bucket:       bucket,
		Key:          common.StringValue(obj.Key),
		StorageClass: string(obj.StorageClass),
	}
	if obj.Size != nil {
		row.Size = *obj.Size
	}
	if obj.LastModified != nil {
		row.LastModified = obj.LastModified.UTC().Format(time.RFC3339)
	}
	return row
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the dataRow fields.
func (r *Results) GetHeaders() []interface{} { return common.Headers(dataRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} { return common.Rows(r.Data) }

// sortResults sorts the results by the given field.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}
	common.SortByField(r.Data, sortFields[field])
	return nil
}

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The sort fields are defined in the struct tag `sort` on dataRow.
// The function returns an error if the given field is not a valid sort field.
func GetSortFields(f string) (map[string]string, error) {
	return common.SortFields(dataRow{}, f)
}

// SortFieldNames returns the valid sort fields, sorted alphabetically.
func SortFieldNames() []string {
	return common.SortFieldNames(dataRow{})
}
