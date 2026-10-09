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

// Package rdscluster contains the search for RDS DB clusters: Aurora and Multi-AZ DB clusters.
//
// It implements the common.Results interface. The instances of a cluster are listed by the rds
// search with --clusters.
package rdscluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/dyegoe/awss/common"
	searchRDS "github.com/dyegoe/awss/search/rds"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// FilterKeyID is the DescribeDBClusters filter of the cluster identifiers. The engine, name,
// engine version and tag filters use the keys of the rds search.
const FilterKeyID = "db-cluster-id"

// pageSize is the number of records asked per DescribeDBClusters and DescribeDBSubnetGroups
// call, the RDS maximum.
const pageSize int32 = 100

// Results describes results of the RDS DB clusters search.
type Results struct {
	common.BaseResults

	// Data contains the DB clusters found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search, keyed by FilterKeyID and the rds filter keys.
	Filters map[string][]string `json:"-"`

	// Regex makes the name patterns Go regular expressions instead of globs.
	Regex bool `json:"-"`
}

// member is one DB instance of a cluster.
type member struct {
	// ID is the DB instance identifier.
	ID string `json:"id"`

	// Writer is true for the cluster's writer instance.
	Writer bool `json:"writer"`
}

// dataRow represents a row of the RDS DB clusters search results.
//
// The table shows only the fields with a `header` tag, to stay about 120 characters wide; the
// others, such as the endpoints, are in the JSON output only.
type dataRow struct {
	// ID is the DB cluster identifier.
	ID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Engine is the database engine, such as aurora-postgresql.
	Engine string `json:"engine,omitempty" header:"Engine" sort:"engine"`

	// EngineVersion is the version of the engine.
	EngineVersion common.Version `json:"engine_version,omitempty" header:"Version" sort:"version"`

	// Status is the cluster status, such as available.
	Status string `json:"status,omitempty" header:"Status" sort:"status"`

	// MembersSummary counts the writer and readers for the table: "1 writer, 2 readers".
	MembersSummary string `json:"-" header:"Members"`

	// VpcID is the VPC of the cluster's DB subnet group.
	VpcID string `json:"vpc_id,omitempty" header:"VPC" sort:"vpc"`

	// Members are the DB instances of the cluster.
	Members []member `json:"members,omitempty"`

	// WriterEndpoint is the cluster (writer) endpoint.
	WriterEndpoint string `json:"writer_endpoint,omitempty"`

	// ReaderEndpoint is the reader endpoint; Multi-AZ DB clusters and clusters without readers
	// may have none.
	ReaderEndpoint string `json:"reader_endpoint,omitempty"`

	// Port is the port of the endpoints.
	Port int32 `json:"port,omitempty"`

	// MultiAZ is "true" when the cluster has instances in several AZs.
	MultiAZ string `json:"multi_az,omitempty"`

	// SubnetGroup is the name of the cluster's DB subnet group, which gives its VPC.
	SubnetGroup string `json:"subnet_group,omitempty"`

	// Encrypted is "true" when the storage is encrypted.
	Encrypted string `json:"encrypted,omitempty"`

	// DeletionProtection is "true" when the cluster cannot be deleted.
	DeletionProtection string `json:"deletion_protection,omitempty"`

	// Tags are the tags of the cluster.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of RDS cluster results.
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

// rdsAPI is the part of the RDS client used by the cluster search.
type rdsAPI interface {
	rds.DescribeDBClustersAPIClient
	rds.DescribeDBSubnetGroupsAPIClient
}

// Search performs the RDS DB clusters search.
//
// Results are stored in the Data field.
func (r *Results) Search(ctx context.Context) {
	local, err := searchRDS.NewLocalFilters(r.Filters, r.Regex)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}

	r.collect(ctx, rds.NewFromConfig(cfg), local)
}

// collect describes the clusters, keeps those that pass the local filters, fills their VPC from
// the subnet groups and sorts the rows.
func (r *Results) collect(ctx context.Context, client rdsAPI, local *searchRDS.LocalFilters) {
	input := &rds.DescribeDBClustersInput{
		Filters:    searchRDS.AWSFilters(r.Filters, FilterKeyID, searchRDS.FilterKeyEngine),
		MaxRecords: aws.Int32(pageSize),
	}
	paginator := rds.NewDescribeDBClustersPaginator(client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("error describing DB clusters: %v", err))
			return
		}
		for i := range page.DBClusters {
			if row := parseCluster(&page.DBClusters[i]); local.Match(row.ID, string(row.EngineVersion), row.Tags) {
				r.Data = append(r.Data, row)
			}
		}
	}

	r.fillVpcIDs(ctx, client)

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// fillVpcIDs sets the VPC of every row from its DB subnet group. DescribeDBClusters returns only
// the group's name, so one paginated DescribeDBSubnetGroups call lists every group of the region,
// never one call per cluster. A failed call is an error of the result set and leaves the VPC
// column empty; the rows are kept. A group that is not found leaves that row's VPC empty.
func (r *Results) fillVpcIDs(ctx context.Context, client rds.DescribeDBSubnetGroupsAPIClient) {
	if len(r.Data) == 0 {
		return
	}
	vpcs := map[string]string{}
	paginator := rds.NewDescribeDBSubnetGroupsPaginator(client,
		&rds.DescribeDBSubnetGroupsInput{MaxRecords: aws.Int32(pageSize)})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("error describing DB subnet groups, VPC left empty: %v", err))
			return
		}
		for _, g := range page.DBSubnetGroups {
			vpcs[aws.ToString(g.DBSubnetGroupName)] = aws.ToString(g.VpcId)
		}
	}
	for i := range r.Data {
		r.Data[i].VpcID = vpcs[r.Data[i].SubnetGroup]
	}
}

// parseCluster converts a DBCluster into a dataRow.
func parseCluster(c *types.DBCluster) dataRow {
	row := dataRow{
		ID:                 aws.ToString(c.DBClusterIdentifier),
		Engine:             aws.ToString(c.Engine),
		EngineVersion:      common.Version(aws.ToString(c.EngineVersion)),
		Status:             aws.ToString(c.Status),
		WriterEndpoint:     aws.ToString(c.Endpoint),
		ReaderEndpoint:     aws.ToString(c.ReaderEndpoint),
		Port:               aws.ToInt32(c.Port),
		MultiAZ:            searchRDS.FormatBool(c.MultiAZ),
		SubnetGroup:        aws.ToString(c.DBSubnetGroup),
		Encrypted:          searchRDS.FormatBool(c.StorageEncrypted),
		DeletionProtection: searchRDS.FormatBool(c.DeletionProtection),
		Tags:               searchRDS.TagsToMap(c.TagList),
	}
	writers, readers := 0, 0
	for _, m := range c.DBClusterMembers {
		writer := aws.ToBool(m.IsClusterWriter)
		row.Members = append(row.Members, member{ID: aws.ToString(m.DBInstanceIdentifier), Writer: writer})
		if writer {
			writers++
		} else {
			readers++
		}
	}
	row.MembersSummary = membersSummary(writers, readers)
	return row
}

// membersSummary returns "1 writer, 2 readers", or "" for a cluster without instances.
func membersSummary(writers, readers int) string {
	var parts []string
	if writers > 0 {
		parts = append(parts, count(writers, "writer"))
	}
	if readers > 0 {
		parts = append(parts, count(readers, "reader"))
	}
	return strings.Join(parts, ", ")
}

// count returns n with noun, plural when n is not 1: "1 writer", "2 readers".
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
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
