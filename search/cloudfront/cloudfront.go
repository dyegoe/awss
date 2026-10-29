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

// Package cloudfront contains the search for CloudFront distributions.
//
// It implements the common.Results interface. CloudFront is a global service: one call lists
// every distribution of an account, so the search runs once per profile and for no region.
package cloudfront

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// Region is the region of the cloudfront result sets: CloudFront is global, so --regions does
// not apply.
const Region = "global"

// pageSize is the number of distributions asked per ListDistributions call: the API's default,
// since AWS documents no maximum.
//
// The paginator sets no page size by itself; asking for one follows the paginated-calls rule of
// the EC2 Describe* calls.
const pageSize int32 = 100

// tagWorkers caps the ListTagsForResource calls running at the same time.
const tagWorkers = 5

// tagInterval paces the ListTagsForResource calls: one starts at most every tagInterval, 5 per
// second. AWS does not publish the quota of this call, so awss keeps to the pace of org.
const tagInterval = 200 * time.Millisecond

// maxAttempts is how many times the CloudFront client tries a call: a throttled tag call waits
// and retries, with the SDK's backoff, longer than the SDK's default of 3 attempts.
const maxAttempts = 10

// Filter keys of the cloudfront search. ListDistributions cannot filter, so awss matches every
// one on the results.
const (
	FilterKeyID     = "id"
	FilterKeyDomain = "domain"
	FilterKeyOrigin = "origin"
	FilterKeyTag    = "tag"
)

// Results describes the distributions of one profile.
type Results struct {
	common.BaseResults

	// Data contains the distributions found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// Regex makes the domain and origin patterns regular expressions instead of globs.
	Regex bool `json:"-"`

	// ShowTags fetches the tags of each distribution found (one API call per distribution).
	ShowTags bool `json:"-"`

	// tagInterval is the least time between two ListTagsForResource calls; tests shorten it.
	tagInterval time.Duration
}

// dataRow represents one distribution.
type dataRow struct {
	// ID is the distribution ID.
	ID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Domain is the CloudFront domain name, such as d111111abcdef8.cloudfront.net.
	Domain string `json:"domain,omitempty" header:"Domain" sort:"domain"`

	// Aliases are the alternate domain names (CNAMEs) of the distribution.
	Aliases []string `json:"aliases,omitempty" header:"Aliases"`

	// Origins are the domain names of the origins.
	Origins []string `json:"origins,omitempty" header:"Origins"`

	// Status is Deployed, or InProgress while a change propagates.
	Status string `json:"status,omitempty" header:"Status" sort:"status"`

	// Enabled is "true" when the distribution serves requests.
	Enabled string `json:"enabled,omitempty" header:"Enabled"`

	// ARN is the Amazon Resource Name of the distribution.
	ARN string `json:"arn,omitempty"`

	// Comment is the free-text comment of the distribution.
	Comment string `json:"comment,omitempty"`

	// PriceClass is the price class, such as PriceClass_100.
	PriceClass string `json:"price_class,omitempty"`

	// HTTPVersion is the highest HTTP version served, such as http2.
	HTTPVersion string `json:"http_version,omitempty"`

	// IPv6 is "true" when IPv6 is enabled.
	IPv6 string `json:"ipv6,omitempty"`

	// WebACLID is the AWS WAF web ACL of the distribution.
	WebACLID string `json:"web_acl_id,omitempty"`

	// Certificate is the ACM certificate ARN, or the IAM certificate ID, of the aliases. Empty
	// for the default *.cloudfront.net certificate.
	Certificate string `json:"certificate,omitempty"`

	// Staging is "true" for the staging distribution of a continuous deployment.
	Staging string `json:"staging,omitempty"`

	// Modified is when the distribution was last changed, in RFC 3339 UTC so it sorts
	// chronologically. CloudFront returns no creation time.
	Modified string `json:"modified,omitempty" sort:"modified"`

	// Tags are the tags of the distribution, filled only when ShowTags is set or a tag filter is.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of cloudfront results. region is kept for the output
// only: the search always calls the global endpoint.
func New(profile, region string, filters map[string][]string, sortField string, regex, showTags bool) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data:        []dataRow{},
		Filters:     filters,
		Regex:       regex,
		ShowTags:    showTags,
		tagInterval: tagInterval,
	}
}

// listTagsAPIClient is the part of the CloudFront client used to read distribution tags. The SDK
// has no interface for it, since the call is not paginated.
type listTagsAPIClient interface {
	ListTagsForResource(
		ctx context.Context, params *cloudfront.ListTagsForResourceInput, optFns ...func(*cloudfront.Options),
	) (*cloudfront.ListTagsForResourceOutput, error)
}

// cloudfrontAPI is the part of the CloudFront client used by the search.
type cloudfrontAPI interface {
	cloudfront.ListDistributionsAPIClient
	listTagsAPIClient
}

// newClient builds the CloudFront client of a profile. It is a variable so tests can replace the
// AWS config with a fake client.
var newClient = func(profile string) (cloudfrontAPI, error) {
	// The global endpoint of the aws partition is served from us-east-1.
	cfg, err := common.AwsConfig(profile, common.DefaultRegion)
	if err != nil {
		return nil, err
	}
	return cloudfront.NewFromConfig(cfg, func(o *cloudfront.Options) {
		o.Retryer = retry.AddWithMaxAttempts(o.Retryer, maxAttempts)
	}), nil
}

// localFilters are the filters awss matches on the distributions.
type localFilters struct {
	ids              []string
	domains, origins *common.Matcher
	tags             *common.TagMatcher

	// byTags is true when a tag filter is set, so the tags must be fetched to match.
	byTags bool
}

// newLocalFilters builds the local filters from a filters map, which it only reads.
func newLocalFilters(filters map[string][]string, regex bool) (*localFilters, error) {
	f := &localFilters{ids: filters[FilterKeyID], byTags: len(filters[FilterKeyTag]) > 0}
	var err error
	if f.domains, err = common.NewMatcher(filters[FilterKeyDomain], regex); err != nil {
		return nil, fmt.Errorf("domains: %w", err)
	}
	if f.origins, err = common.NewMatcher(filters[FilterKeyOrigin], regex); err != nil {
		return nil, fmt.Errorf("origins: %w", err)
	}
	if f.tags, err = common.NewTagMatcher(filters[FilterKeyTag]); err != nil {
		return nil, fmt.Errorf("tags: %w", err)
	}
	return f, nil
}

// matchRow reports whether a row passes the ID, domain and origin filters. The domain filter
// matches the aliases and the CloudFront domain name.
func (f *localFilters) matchRow(row *dataRow) bool {
	if len(f.ids) > 0 && !slices.Contains(f.ids, row.ID) {
		return false
	}
	return f.domains.MatchAny(append([]string{row.Domain}, row.Aliases...)) && f.origins.MatchAny(row.Origins)
}

// Search lists the distributions of the profile. A missing permission is an error in the result
// set, never an empty list.
func (r *Results) Search(ctx context.Context) {
	local, err := newLocalFilters(r.Filters, r.Regex)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}
	client, err := newClient(r.Profile)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	r.collect(ctx, client, local)
}

// collect lists the distributions with client and keeps those that pass the local filters. It
// fetches the tags with ShowTags or a tag filter, then sorts the rows.
func (r *Results) collect(ctx context.Context, client cloudfrontAPI, local *localFilters) {
	if err := r.collectDistributions(ctx, client, local); err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error with profile %q: %v", r.Profile, err))
		return
	}

	if r.ShowTags || local.byTags {
		r.collectTags(ctx, client)
	}
	if local.byTags {
		// A row whose tags could not be read has none, so it does not match.
		r.Data = slices.DeleteFunc(r.Data, func(row dataRow) bool { return !local.tags.Match(row.Tags) })
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// collectDistributions lists every distribution, following every page, and appends those that
// pass the ID, domain and origin filters.
func (r *Results) collectDistributions(
	ctx context.Context, client cloudfront.ListDistributionsAPIClient, local *localFilters,
) error {
	paginator := cloudfront.NewListDistributionsPaginator(client, &cloudfront.ListDistributionsInput{
		MaxItems: aws.Int32(pageSize),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing the distributions: %w", err)
		}
		if page.DistributionList == nil {
			continue
		}
		for i := range page.DistributionList.Items {
			if row := parseDistribution(&page.DistributionList.Items[i]); local.matchRow(&row) {
				r.Data = append(r.Data, row)
			}
		}
	}
	return nil
}

// collectTags fills the Tags of every row: one ListTagsForResource call starts at most every
// r.tagInterval, and at most tagWorkers run at once. A failure is reported per distribution and
// leaves that row without tags. When ctx ends, no new call starts.
func (r *Results) collectTags(ctx context.Context, client listTagsAPIClient) {
	errs := make([]error, len(r.Data))
	sem := make(chan struct{}, tagWorkers)
	var wg sync.WaitGroup
	pace := time.NewTicker(r.tagInterval)
	defer pace.Stop()

	for i := range r.Data {
		if i > 0 && !wait(ctx, pace.C) {
			break
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			r.Data[i].Tags, errs[i] = distributionTags(ctx, client, &r.Data[i])
		})
	}
	wg.Wait()

	// Append in row order, so the errors are deterministic.
	for _, err := range errs {
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
	}
}

// wait waits for the next tick, and reports false when ctx ends first.
func wait(ctx context.Context, tick <-chan time.Time) bool {
	select {
	case <-tick:
		return true
	case <-ctx.Done():
		return false
	}
}

// distributionTags returns the tags of one distribution. A distribution without tags returns an
// empty map.
func distributionTags(ctx context.Context, client listTagsAPIClient, row *dataRow) (map[string]string, error) {
	out, err := client.ListTagsForResource(ctx, &cloudfront.ListTagsForResourceInput{Resource: aws.String(row.ARN)})
	if err != nil {
		return nil, fmt.Errorf("error getting tags of distribution %s: %w", row.ID, err)
	}
	tags := map[string]string{}
	if out == nil || out.Tags == nil {
		return tags, nil
	}
	for _, t := range out.Tags.Items {
		if t.Key != nil {
			tags[*t.Key] = aws.ToString(t.Value)
		}
	}
	return tags, nil
}

// parseDistribution converts a distribution summary into a dataRow.
func parseDistribution(d *types.DistributionSummary) dataRow {
	row := dataRow{
		ID:          aws.ToString(d.Id),
		Domain:      aws.ToString(d.DomainName),
		Status:      aws.ToString(d.Status),
		Enabled:     formatBool(d.Enabled),
		ARN:         aws.ToString(d.ARN),
		Comment:     aws.ToString(d.Comment),
		PriceClass:  string(d.PriceClass),
		HTTPVersion: string(d.HttpVersion),
		IPv6:        formatBool(d.IsIPV6Enabled),
		WebACLID:    aws.ToString(d.WebACLId),
		Staging:     formatBool(d.Staging),
	}
	if d.Aliases != nil && len(d.Aliases.Items) > 0 {
		row.Aliases = d.Aliases.Items
	}
	if d.Origins != nil {
		for _, o := range d.Origins.Items {
			row.Origins = append(row.Origins, aws.ToString(o.DomainName))
		}
	}
	if c := d.ViewerCertificate; c != nil {
		row.Certificate = aws.ToString(c.ACMCertificateArn)
		if row.Certificate == "" {
			row.Certificate = aws.ToString(c.IAMCertificateId)
		}
	}
	if d.LastModifiedTime != nil {
		row.Modified = d.LastModifiedTime.UTC().Format(time.RFC3339)
	}
	return row
}

// formatBool returns "true" or "false", or "" for nil, so the JSON output omits it.
func formatBool(b *bool) string {
	if b == nil {
		return ""
	}
	return strconv.FormatBool(*b)
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
