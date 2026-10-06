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

// Package subnet contains the search for subnets.
//
// It implements the common.Results interface.
package subnet

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	// filterKeySubnetID is the AWS "subnet-id" filter key.
	filterKeySubnetID = "subnet-id"

	// FilterKeyCIDR is the AWS filter key that matches the IPv4 CIDR block of a subnet.
	// The value must exactly match the block (wildcards are allowed).
	FilterKeyCIDR = "cidr-block"
)

// Results describes results of the subnets search.
type Results struct {
	common.BaseResults

	// Data contains the subnets found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`
}

// dataRow represents a row of the subnets search results.
type dataRow struct {
	// SubnetID is the ID of the subnet.
	SubnetID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Name is the tag:Name of the subnet.
	Name string `json:"name,omitempty" header:"Name" sort:"name"`

	// VpcID is the ID of the VPC the subnet belongs to.
	VpcID string `json:"vpc_id,omitempty" header:"VPC ID" sort:"vpc-id"`

	// CidrBlock is the IPv4 CIDR block of the subnet.
	CidrBlock string `json:"cidr,omitempty" header:"CIDR" sort:"cidr"`

	// AvailabilityZone is the AZ of the subnet.
	AvailabilityZone string `json:"az,omitempty" header:"AZ" sort:"az"`

	// AvailableIPs is the number of unused IPv4 addresses in the subnet.
	AvailableIPs int32 `json:"available_ips" header:"Available IPs" sort:"available-ips"`

	// State is the state of the subnet (pending or available).
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// MapPublicIP indicates whether instances launched in the subnet get a public IPv4 address.
	MapPublicIP string `json:"public_ip_on_launch,omitempty" header:"Public IP on Launch" sort:"public-ip"`

	// DefaultForAz indicates whether the subnet is the default one for its AZ.
	DefaultForAz string `json:"default_for_az,omitempty" header:"Default for AZ" sort:"default"`

	// OwnerID is the ID of the AWS account that owns the subnet.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// Tags are the tags assigned to the subnet.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of subnet results.
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

// Search performs the subnets search.
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

	r.collect(ctx, ec2.NewFromConfig(cfg), input)
}

// pageSize is the number of subnets asked per DescribeSubnets call.
//
// AWS recommends paginated calls only, so every search that does not name the subnets asks for
// pages. 1000 is the largest page AWS allows.
const pageSize int32 = 1000

// collect describes the subnets, following every page, and sorts the rows.
//
// Search and InCIDRs build the real client; tests pass a fake.
func (r *Results) collect(ctx context.Context, client ec2.DescribeSubnetsAPIClient, input *ec2.DescribeSubnetsInput) {
	paged := *input
	// AWS rejects MaxResults together with SubnetIds.
	if len(paged.SubnetIds) == 0 && paged.MaxResults == nil {
		paged.MaxResults = aws.Int32(pageSize)
	}

	paginator := ec2.NewDescribeSubnetsPaginator(client, &paged)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("error describing subnets: %v", err))
			return
		}
		for i := range page.Subnets {
			r.Data = append(r.Data, parseSubnet(&page.Subnets[i]))
		}
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// parseSubnet converts a single Subnet into a dataRow.
func parseSubnet(subnet *types.Subnet) dataRow {
	row := dataRow{
		SubnetID:         common.StringValue(subnet.SubnetId),
		Name:             common.TagName(subnet.Tags),
		VpcID:            common.StringValue(subnet.VpcId),
		CidrBlock:        common.StringValue(subnet.CidrBlock),
		AvailabilityZone: common.StringValue(subnet.AvailabilityZone),
		State:            string(subnet.State),
		OwnerID:          common.StringValue(subnet.OwnerId),
		Tags:             common.TagsToMap(subnet.Tags),
	}
	if subnet.AvailableIpAddressCount != nil {
		row.AvailableIPs = *subnet.AvailableIpAddressCount
	}
	if subnet.MapPublicIpOnLaunch != nil {
		row.MapPublicIP = strconv.FormatBool(*subnet.MapPublicIpOnLaunch)
	}
	if subnet.DefaultForAz != nil {
		row.DefaultForAz = strconv.FormatBool(*subnet.DefaultForAz)
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
// "subnet-id" is passed as SubnetIds, "tag:Name" and "tag" are expanded into tag filters,
// "availability-zone" letters are expanded with the region, and every other key is passed
// as-is as an AWS filter.
func (r *Results) getFilters() (*ec2.DescribeSubnetsInput, error) {
	input := ec2.DescribeSubnetsInput{}

	for key, values := range r.Filters {
		switch key {
		case filterKeySubnetID:
			input.SubnetIds = values
		case "tag:Name":
			input.Filters = append(input.Filters, common.FilterNames(values)...)
		case "tag":
			tagFilters, err := common.FilterTags(values)
			if err != nil {
				return nil, fmt.Errorf("building tag filters: %w", err)
			}
			input.Filters = append(input.Filters, tagFilters...)
		case "availability-zone":
			input.Filters = append(input.Filters, common.FilterAvailabilityZones(values, r.Region)...)
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

// InCIDRs returns the IDs of the subnets whose IPv4 CIDR block overlaps any of the given
// networks, and the IDs of the VPCs those subnets belong to (sorted, without duplicates).
//
// It is used by the ec2 search to resolve --cidrs per profile and region: every address of a
// range lives in a subnet that overlaps it, whatever VPC CIDR block the subnet was carved from.
// AWS has no overlap filter, so all subnets of the region are listed and matched here.
// It returns empty slices when nothing overlaps and an error when the search failed.
func InCIDRs(ctx context.Context, profile, region string, nets []*net.IPNet) (subnetIDs, vpcIDs []string, err error) {
	if len(nets) == 0 {
		return []string{}, []string{}, nil
	}
	cfg, err := common.AwsConfig(profile, region)
	if err != nil {
		return nil, nil, fmt.Errorf("searching subnets by CIDR: error getting aws config: %w", err)
	}
	return inCIDRs(ctx, ec2.NewFromConfig(cfg), profile, region, nets)
}

// inCIDRs is InCIDRs once the EC2 client is built. Tests pass a fake client.
func inCIDRs(
	ctx context.Context, client ec2.DescribeSubnetsAPIClient, profile, region string, nets []*net.IPNet,
) (subnetIDs, vpcIDs []string, err error) {
	r := New(profile, region, map[string][]string{}, "id")
	r.collect(ctx, client, &ec2.DescribeSubnetsInput{})
	if len(r.Errors) > 0 {
		return nil, nil, fmt.Errorf("searching subnets by CIDR: %s", common.StringSliceToString(r.Errors, "; "))
	}

	subnetIDs, vpcIDs = []string{}, []string{}
	seenVPC := map[string]bool{}
	for i := range r.Data {
		if !overlapsAny(r.Data[i].CidrBlock, nets) {
			continue
		}
		subnetIDs = append(subnetIDs, r.Data[i].SubnetID)
		if vpc := r.Data[i].VpcID; vpc != "" && !seenVPC[vpc] {
			seenVPC[vpc] = true
			vpcIDs = append(vpcIDs, vpc)
		}
	}
	sort.Strings(vpcIDs)
	return subnetIDs, vpcIDs, nil
}

// overlapsAny reports whether the CIDR block overlaps any of nets. An unparsable block never does.
func overlapsAny(cidrBlock string, nets []*net.IPNet) bool {
	_, block, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return false
	}
	for _, n := range nets {
		if common.CIDRsOverlap(block, n) {
			return true
		}
	}
	return false
}
