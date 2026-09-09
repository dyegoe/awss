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

// Package s3 contains the search for S3 buckets.
//
// It implements the common.Results interface. Buckets are listed per region, so the
// profile x region fan-out of the search package lists each bucket exactly once.
package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// FilterKeyName is the filters key carrying the bucket name patterns.
//
// S3 has no server-side name filter, so names are matched client-side with common.Matcher.
const FilterKeyName = "name"

// Results describes results of the S3 buckets search.
type Results struct {
	common.BaseResults

	// Data contains the buckets found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`

	// Regex makes the name patterns regular expressions instead of globs.
	Regex bool `json:"-"`
}

// dataRow represents a row of the S3 buckets search results.
type dataRow struct {
	// Name is the bucket name.
	Name string `json:"name,omitempty" header:"Name" sort:"name"`

	// Region is the region the bucket lives in.
	Region string `json:"region,omitempty" header:"Region" sort:"region"`

	// CreationDate is when the bucket was created, in RFC 3339 UTC so it sorts chronologically.
	CreationDate string `json:"created,omitempty" header:"Created" sort:"created"`

	// ARN is the Amazon Resource Name of the bucket.
	ARN string `json:"arn,omitempty" header:"ARN" sort:"arn"`
}

// New initiates and returns a new instance of S3 bucket results.
func New(profile, region string, filters map[string][]string, sortField string, regex bool) *Results {
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
	}
}

// Search performs the S3 buckets search.
//
// Results are stored in the Data field. Only buckets in r.Region are listed.
func (r *Results) Search(ctx context.Context) {
	matcher, err := r.matcher()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}

	if err := r.collectBuckets(ctx, s3.NewFromConfig(cfg), matcher); err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// matcher builds the client-side name matcher from the "name" filter.
func (r *Results) matcher() (*common.Matcher, error) {
	return common.NewMatcher(r.Filters[FilterKeyName], r.Regex)
}

// collectBuckets lists the buckets of r.Region and appends those whose name matches.
//
// The listing is narrowed server-side with the literal prefix of the pattern when there is one.
func (r *Results) collectBuckets(ctx context.Context, client s3.ListBucketsAPIClient, matcher *common.Matcher) error {
	input := &s3.ListBucketsInput{BucketRegion: common.String(r.Region)}
	if prefix := matcher.Prefix(); prefix != "" {
		input.Prefix = common.String(prefix)
	}

	paginator := s3.NewListBucketsPaginator(client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("error listing buckets: %w", err)
		}
		for i := range page.Buckets {
			if row := parseBucket(&page.Buckets[i]); matcher.Match(row.Name) {
				r.Data = append(r.Data, row)
			}
		}
	}
	return nil
}

// parseBucket converts a single Bucket into a dataRow.
func parseBucket(bucket *types.Bucket) dataRow {
	row := dataRow{
		Name:   common.StringValue(bucket.Name),
		Region: common.StringValue(bucket.BucketRegion),
		ARN:    common.StringValue(bucket.BucketArn),
	}
	if bucket.CreationDate != nil {
		row.CreationDate = bucket.CreationDate.UTC().Format(time.RFC3339)
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
