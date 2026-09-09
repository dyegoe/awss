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

// Package vpc contains the search for VPCs.
//
// It implements the common.Results interface.
package vpc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	// filterKeyVpcID is the AWS "vpc-id" filter key.
	filterKeyVpcID = "vpc-id"

	// FilterKeyCIDR is the AWS filter key that matches any IPv4 CIDR block associated with a VPC,
	// primary or secondary. The value must exactly match the block (wildcards are allowed).
	FilterKeyCIDR = "cidr-block-association.cidr-block"
)

// Results describes results of the VPCs search.
type Results struct {
	common.BaseResults

	// Data contains the VPCs found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`
}

// dataRow represents a row of the VPCs search results.
type dataRow struct {
	// VpcID is the ID of the VPC.
	VpcID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Name is the tag:Name of the VPC.
	Name string `json:"name,omitempty" header:"Name" sort:"name"`

	// CidrBlock is the primary IPv4 CIDR block of the VPC.
	CidrBlock string `json:"cidr,omitempty" header:"CIDR" sort:"cidr"`

	// CidrBlocks are all the IPv4 CIDR blocks associated with the VPC, primary included.
	CidrBlocks []string `json:"cidrs,omitempty" header:"CIDR Blocks" sort:"cidrs"`

	// State is the state of the VPC (pending or available).
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// IsDefault indicates whether the VPC is the default VPC of the region.
	IsDefault string `json:"default,omitempty" header:"Default" sort:"default"`

	// OwnerID is the ID of the AWS account that owns the VPC.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// DhcpOptionsID is the ID of the DHCP options set associated with the VPC.
	DhcpOptionsID string `json:"dhcp_options_id,omitempty" header:"DHCP Options ID" sort:"dhcp"`

	// Tags are the tags assigned to the VPC.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of VPC results.
func New(profile, region string, filters map[string][]string, sortField string) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data:    []dataRow{},
		Filters: filters,
	}
}

// Search performs the VPCs search.
//
// Results are stored in the Data field.
func (r *Results) Search(ctx context.Context) {
	input, err := r.getFilters()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}

	paginator := ec2.NewDescribeVpcsPaginator(ec2.NewFromConfig(cfg), input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("error describing vpcs: %v", err))
			return
		}
		for i := range page.Vpcs {
			r.Data = append(r.Data, parseVpc(&page.Vpcs[i]))
		}
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// parseVpc converts a single Vpc into a dataRow.
func parseVpc(vpc *types.Vpc) dataRow {
	row := dataRow{
		VpcID:         common.StringValue(vpc.VpcId),
		Name:          common.TagName(vpc.Tags),
		CidrBlock:     common.StringValue(vpc.CidrBlock),
		State:         string(vpc.State),
		OwnerID:       common.StringValue(vpc.OwnerId),
		DhcpOptionsID: common.StringValue(vpc.DhcpOptionsId),
		Tags:          common.TagsToMap(vpc.Tags),
	}
	if vpc.IsDefault != nil {
		row.IsDefault = strconv.FormatBool(*vpc.IsDefault)
	}
	for i := range vpc.CidrBlockAssociationSet {
		if cidr := vpc.CidrBlockAssociationSet[i].CidrBlock; cidr != nil {
			row.CidrBlocks = append(row.CidrBlocks, *cidr)
		}
	}
	return row
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the dataRow fields.
func (r *Results) GetHeaders() []interface{} { return common.Headers(dataRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} { return common.Rows(r.Data) }

// getFilters returns the filters used to search.
//
// "vpc-id" is passed as VpcIds, "tag:Name" and "tag" are expanded into tag filters,
// and every other key is passed as-is as an AWS filter.
func (r *Results) getFilters() (*ec2.DescribeVpcsInput, error) {
	input := ec2.DescribeVpcsInput{}

	for key, values := range r.Filters {
		switch key {
		case filterKeyVpcID:
			input.VpcIds = values
		case "tag:Name":
			input.Filters = append(input.Filters, common.FilterNames(values)...)
		case "tag":
			tagFilters, err := common.FilterTags(values)
			if err != nil {
				return nil, fmt.Errorf("building tag filters: %w", err)
			}
			input.Filters = append(input.Filters, tagFilters...)
		default:
			input.Filters = append(input.Filters, common.FilterDefault(key, values)...)
		}
	}
	return &input, nil
}

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

// searchFn runs the search of r. It is a variable so tests can replace the AWS call.
var searchFn = func(ctx context.Context, r *Results) { r.Search(ctx) }

// IDsByCIDR returns the IDs of the VPCs that have any of the given IPv4 CIDR blocks associated.
//
// It is used by the ec2 search to resolve --cidrs into VPC IDs, per profile and region.
// It returns an empty slice when nothing matches and an error when the search failed.
func IDsByCIDR(ctx context.Context, profile, region string, cidrs []string) ([]string, error) {
	if len(cidrs) == 0 {
		return []string{}, nil
	}

	r := New(profile, region, map[string][]string{FilterKeyCIDR: cidrs}, "id")
	searchFn(ctx, r)
	if len(r.Errors) > 0 {
		return nil, fmt.Errorf("searching VPCs by CIDR: %s", common.StringSliceToString(r.Errors, "; "))
	}

	ids := make([]string, 0, len(r.Data))
	for i := range r.Data {
		ids = append(ids, r.Data[i].VpcID)
	}
	return ids, nil
}
