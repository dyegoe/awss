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

// Package rds contains the search for RDS DB instances.
//
// It implements the common.Results interface.
package rds

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// Filter keys of the rds search. The first three are sent to AWS; DescribeDBInstances cannot
// filter on the others, so awss matches them on the results.
const (
	FilterKeyID            = "db-instance-id"
	FilterKeyEngine        = "engine"
	FilterKeyCluster       = "db-cluster-id"
	FilterKeyName          = "name"
	FilterKeyEngineVersion = "engine-version"
	FilterKeyTag           = "tag"
)

// pageSize is the number of DB instances asked per DescribeDBInstances call, the RDS maximum.
const pageSize int32 = 100

// Results describes results of the RDS DB instances search.
type Results struct {
	common.BaseResults

	// Data contains the DB instances found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// Regex makes the name patterns Go regular expressions instead of globs.
	Regex bool `json:"-"`
}

// dataRow represents a row of the RDS DB instances search results.
//
// The table shows only the fields with a `header` tag, to stay about 120 characters wide; the
// others, such as the endpoint, are in the JSON output only.
type dataRow struct {
	// ID is the DB instance identifier.
	ID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Engine is the database engine, such as postgres or aurora-mysql.
	Engine string `json:"engine,omitempty" header:"Engine" sort:"engine"`

	// EngineVersion is the version of the engine.
	EngineVersion common.Version `json:"engine_version,omitempty" header:"Version" sort:"version"`

	// Class is the DB instance class, such as db.r6g.large.
	Class string `json:"class,omitempty" header:"Class" sort:"class"`

	// Status is the DB instance status, such as available.
	Status string `json:"status,omitempty" header:"Status" sort:"status"`

	// MultiAZ is "true" when the instance is a Multi-AZ deployment.
	MultiAZ string `json:"multi_az,omitempty" header:"Multi-AZ"`

	// AvailabilityZone is the AZ of the instance.
	AvailabilityZone string `json:"az,omitempty" header:"AZ" sort:"az"`

	// Endpoint is host:port, empty while the instance is being created. JSON only: it is often
	// 60 characters, too wide for the table.
	Endpoint string `json:"endpoint,omitempty"`

	// VpcID is the VPC of the instance's DB subnet group.
	VpcID string `json:"vpc_id,omitempty" sort:"vpc"`

	// Public is "true" when the instance is publicly accessible.
	Public string `json:"publicly_accessible,omitempty"`

	// Encrypted is "true" when the storage is encrypted.
	Encrypted string `json:"encrypted,omitempty"`

	// ClusterID is the Aurora or Multi-AZ DB cluster the instance belongs to.
	ClusterID string `json:"cluster_id,omitempty" header:"Cluster" sort:"cluster"`

	// StorageType is the storage type, such as gp3 or aurora.
	StorageType string `json:"storage_type,omitempty"`

	// StorageGiB is the allocated storage in GiB.
	StorageGiB int32 `json:"storage_gib,omitempty"`

	// Tags are the tags of the instance.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of RDS results.
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

// LocalFilters are the filters awss matches on the results, since the RDS Describe calls cannot
// filter on them: the identifier (FilterKeyName), the engine version (FilterKeyEngineVersion) and
// the tags (FilterKeyTag). The rds-cluster search uses them too.
type LocalFilters struct {
	names, versions *common.Matcher
	tags            *common.TagMatcher
}

// NewLocalFilters builds the local filters from a filters map, which it only reads. regex makes
// the name patterns Go regular expressions instead of globs.
func NewLocalFilters(filters map[string][]string, regex bool) (*LocalFilters, error) {
	f := &LocalFilters{}
	var err error
	if f.names, err = common.NewMatcher(filters[FilterKeyName], regex); err != nil {
		return nil, fmt.Errorf("names: %w", err)
	}
	if f.versions, err = common.NewMatcher(filters[FilterKeyEngineVersion], false); err != nil {
		return nil, fmt.Errorf("engine versions: %w", err)
	}
	if f.tags, err = common.NewTagMatcher(filters[FilterKeyTag]); err != nil {
		return nil, fmt.Errorf("tags: %w", err)
	}
	return f, nil
}

// Match reports whether a resource with this identifier, engine version and tags passes every
// local filter.
func (f *LocalFilters) Match(id, version string, tags map[string]string) bool {
	return f.names.Match(id) && f.versions.Match(version) && f.tags.Match(tags)
}

// AWSFilters returns the DescribeDB* filters of the given keys that have values, in key order.
func AWSFilters(filters map[string][]string, keys ...string) []types.Filter {
	var out []types.Filter
	for _, key := range keys {
		if values := filters[key]; len(values) > 0 {
			out = append(out, types.Filter{Name: aws.String(key), Values: values})
		}
	}
	return out
}

// Search performs the RDS DB instances search.
//
// Results are stored in the Data field.
func (r *Results) Search(ctx context.Context) {
	input, local, err := r.getFilters()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}

	r.collect(ctx, rds.NewFromConfig(cfg), input, local)
}

// collect describes the DB instances, keeps those that pass the local filters and sorts the rows.
func (r *Results) collect(
	ctx context.Context, client rds.DescribeDBInstancesAPIClient, input *rds.DescribeDBInstancesInput, local *LocalFilters,
) {
	paged := *input
	paged.MaxRecords = aws.Int32(pageSize)

	paginator := rds.NewDescribeDBInstancesPaginator(client, &paged)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("error describing DB instances: %v", err))
			return
		}
		for i := range page.DBInstances {
			if row := parseInstance(&page.DBInstances[i]); local.Match(row.ID, string(row.EngineVersion), row.Tags) {
				r.Data = append(r.Data, row)
			}
		}
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// getFilters splits r.Filters into the DescribeDBInstances filters and the local ones. It reads
// r.Filters and never changes it: one map is shared by every search of a run.
func (r *Results) getFilters() (*rds.DescribeDBInstancesInput, *LocalFilters, error) {
	local, err := NewLocalFilters(r.Filters, r.Regex)
	if err != nil {
		return nil, nil, err
	}
	input := &rds.DescribeDBInstancesInput{Filters: AWSFilters(r.Filters, FilterKeyID, FilterKeyEngine, FilterKeyCluster)}
	return input, local, nil
}

// parseInstance converts a DBInstance into a dataRow.
func parseInstance(db *types.DBInstance) dataRow {
	row := dataRow{
		ID:               aws.ToString(db.DBInstanceIdentifier),
		Engine:           aws.ToString(db.Engine),
		EngineVersion:    common.Version(aws.ToString(db.EngineVersion)),
		Class:            aws.ToString(db.DBInstanceClass),
		Status:           aws.ToString(db.DBInstanceStatus),
		MultiAZ:          FormatBool(db.MultiAZ),
		AvailabilityZone: aws.ToString(db.AvailabilityZone),
		Public:           FormatBool(db.PubliclyAccessible),
		Encrypted:        FormatBool(db.StorageEncrypted),
		ClusterID:        aws.ToString(db.DBClusterIdentifier),
		StorageType:      aws.ToString(db.StorageType),
		StorageGiB:       aws.ToInt32(db.AllocatedStorage),
		Tags:             TagsToMap(db.TagList),
	}
	if db.Endpoint != nil && db.Endpoint.Address != nil {
		row.Endpoint = *db.Endpoint.Address
		if db.Endpoint.Port != nil {
			row.Endpoint += ":" + strconv.Itoa(int(*db.Endpoint.Port))
		}
	}
	if db.DBSubnetGroup != nil {
		row.VpcID = aws.ToString(db.DBSubnetGroup.VpcId)
	}
	return row
}

// FormatBool returns "true" or "false", or "" when b is nil.
func FormatBool(b *bool) string {
	if b == nil {
		return ""
	}
	return strconv.FormatBool(*b)
}

// TagsToMap converts RDS tags to a map. RDS uses its own Tag type, so it cannot share
// common.TagsToMap. A tag with a nil key is skipped and a nil value reads as "".
func TagsToMap(tags []types.Tag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		if t.Key != nil {
			m[*t.Key] = aws.ToString(t.Value)
		}
	}
	return m
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
