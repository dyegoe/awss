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

// Package eni contains the search for ENIs.
//
// It implements the common.Results interface
package eni

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/dyegoe/awss/common"
	searchEC2 "github.com/dyegoe/awss/search/ec2"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Results describes results of the ENIs search.
type Results struct {
	common.BaseResults

	// Data contains the instances found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`

	// NoInstanceName skips the instance name lookup when true.
	NoInstanceName bool `json:"-"`

	// AccountNames maps an account ID to its name, for the Owner column. It is built once per run
	// and shared by every search of the run, so it is only read.
	AccountNames map[string]string `json:"-"`
}

// dataRow represents a row of the ENIs search results.
type dataRow struct {
	// InterfaceInfo are the network interface infos (ID, type, AZ, status, subnet, instance).
	InterfaceInfo eniInfo `json:"interface_info,omitempty" header:"Interface Info"`

	// PrivateIPAddresses are the private IP addresses assigned to the network interface.
	PrivateIPAddresses []string `json:"private_ips,omitempty" header:"Private IPs"`

	// PublicIPAddresses are the public IP addresses or Elastic IP addresses bound to the network interface.
	PublicIPAddresses []string `json:"public_ips,omitempty" header:"Public IPs"`

	// Tags are the tags assigned to the network interface.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// eniInfo represents the network interface info.
type eniInfo struct {
	// NetworkInterfaceID is the ID of the network interface.
	NetworkInterfaceID string `json:"id,omitempty" header:"ID" sort:"id"`

	// InterfaceType is the interface type.
	InterfaceType string `json:"type,omitempty" header:"Type" sort:"type"`

	// AvailabilityZone is the AZ of the network interface.
	AvailabilityZone string `json:"az,omitempty" header:"AZ" sort:"az"`

	// Status is the status of the network interface.
	Status string `json:"status,omitempty" header:"Status" sort:"status"`

	// SubnetID is the ID of the subnet that the network interface is in.
	SubnetID string `json:"subnet_id,omitempty" header:"Subnet ID" sort:"subnet-id"`

	// InstanceID is the ID of the instance that this interface is associate.
	InstanceID string `json:"instance_id,omitempty" header:"Instance ID" sort:"instance-id"`

	// InstanceName is the name of the instance that this interface is associate.
	InstanceName string `json:"instance_name,omitempty" header:"Instance Name" sort:"instance-name"`

	// OwnerID is the ID of the AWS account that owns the network interface.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// OwnerName is the name of the owner account, from AccountNames; empty when it is unknown.
	OwnerName string `json:"owner_name,omitempty" header:"Owner" sort:"owner-name"`

	// RequesterID is the account or service that created the network interface (JSON output only).
	RequesterID string `json:"requester_id,omitempty"`

	// RequesterManaged reports whether an AWS service manages the network interface (JSON output only).
	RequesterManaged bool `json:"requester_managed,omitempty"`
}

// New initiates and returns a new instance of ENI results.
func New(profile, region string, filters map[string][]string, sortField string, noInstanceName bool) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data:           []dataRow{},
		Filters:        filters,
		NoInstanceName: noInstanceName,
	}
}

// ec2API is the part of the EC2 client used by the ENI search.
//
// Search builds the real client; tests pass a fake to collect.
type ec2API interface {
	ec2.DescribeNetworkInterfacesAPIClient
	ec2.DescribeInstancesAPIClient
}

// Search performs the ENIs search.
//
// results are stored in the Data field.
func (r *Results) Search(ctx context.Context) {
	// Get search filters.
	input, err := r.getFilters()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	// Get AWS config.
	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %s", err))
		return
	}

	r.collect(ctx, ec2.NewFromConfig(cfg), input)
}

// collect describes the network interfaces, looks up their instance names and sorts the rows.
func (r *Results) collect(ctx context.Context, client ec2API, input *ec2.DescribeNetworkInterfacesInput) {
	instanceIDs, err := r.collectENIs(ctx, client, input)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	if len(instanceIDs) > 0 && !r.NoInstanceName {
		r.enrichInstanceNames(ctx, client, instanceIDs)
	}

	if r.SortField != "" {
		if err := r.sortResults(r.SortField); err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
	}
}

// pageSize is the number of network interfaces asked per DescribeNetworkInterfaces call.
//
// AWS recommends paginated calls only (unpaginated ones are more exposed to throttling and
// timeouts), so every search that does not name the interfaces asks for pages. 1000 is the
// largest page AWS allows.
const pageSize int32 = 1000

// collectENIs appends one row per network interface to r.Data, following every page.
//
// It returns the IDs of the instances the interfaces are attached to.
func (r *Results) collectENIs(
	ctx context.Context, client ec2.DescribeNetworkInterfacesAPIClient, input *ec2.DescribeNetworkInterfacesInput,
) ([]string, error) {
	paged := *input
	// AWS rejects MaxResults together with NetworkInterfaceIds.
	if len(paged.NetworkInterfaceIds) == 0 && paged.MaxResults == nil {
		paged.MaxResults = aws.Int32(pageSize)
	}

	var instanceIDs []string
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(client, &paged)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error describing network interfaces: %w", err)
		}
		for i := range page.NetworkInterfaces {
			row := parseENIRow(&page.NetworkInterfaces[i])
			row.InterfaceInfo.OwnerName = r.AccountNames[row.InterfaceInfo.OwnerID]
			r.Data = append(r.Data, row)
			if row.InterfaceInfo.InstanceID != "" {
				instanceIDs = append(instanceIDs, row.InterfaceInfo.InstanceID)
			}
		}
	}
	return instanceIDs, nil
}

// enrichInstanceNames fills the instance name of every row attached to an instance.
//
// A failed lookup is reported in r.Errors and leaves the names empty; the rows are kept.
func (r *Results) enrichInstanceNames(ctx context.Context, client ec2.DescribeInstancesAPIClient, ids []string) {
	names, err := searchEC2.InstanceNames(ctx, client, ids)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}
	for i := range r.Data {
		if id := r.Data[i].InterfaceInfo.InstanceID; id != "" {
			r.Data[i].InterfaceInfo.InstanceName = names[id]
		}
	}
}

// parseENIRow converts a single NetworkInterface into a dataRow.
func parseENIRow(eni *types.NetworkInterface) dataRow {
	row := dataRow{
		InterfaceInfo: eniInfo{
			NetworkInterfaceID: aws.ToString(eni.NetworkInterfaceId),
			InterfaceType:      string(eni.InterfaceType),
			AvailabilityZone:   aws.ToString(eni.AvailabilityZone),
			SubnetID:           aws.ToString(eni.SubnetId),
			Status:             string(eni.Status),
			OwnerID:            aws.ToString(eni.OwnerId),
			RequesterID:        aws.ToString(eni.RequesterId),
		},
		Tags: common.TagsToMap(eni.TagSet),
	}
	if eni.Attachment != nil && eni.Attachment.InstanceId != nil {
		row.InterfaceInfo.InstanceID = *eni.Attachment.InstanceId
	}
	if eni.RequesterManaged != nil {
		row.InterfaceInfo.RequesterManaged = *eni.RequesterManaged
	}
	for _, ip := range eni.PrivateIpAddresses {
		row.PrivateIPAddresses = append(row.PrivateIPAddresses, aws.ToString(ip.PrivateIpAddress))
		if ip.Association != nil {
			row.PublicIPAddresses = append(row.PublicIPAddresses, aws.ToString(ip.Association.PublicIp))
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
// The filters are defined in the results.filters field.
// This function expects the filters to be in the format used by the AWS SDK.
// Except for "ids", "tags" and "availability-zones", all other filters are passed as it is.
// If no filters are given, it returns an empty list.
func (r *Results) getFilters() (*ec2.DescribeNetworkInterfacesInput, error) {
	input := ec2.DescribeNetworkInterfacesInput{}

	for key, values := range r.Filters {
		switch key {
		case "network-interface-id":
			input.NetworkInterfaceIds = values
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

// sortResults sorts the results by the given field of the nested InterfaceInfo struct.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}

	name := sortFields[field]
	sort.SliceStable(r.Data, func(p, q int) bool {
		return common.Less(
			reflect.ValueOf(r.Data[p].InterfaceInfo).FieldByName(name),
			reflect.ValueOf(r.Data[q].InterfaceInfo).FieldByName(name),
		)
	})
	return nil
}

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The sort fields are defined in the struct tag `sort` on eniInfo.
// The function returns an error if the given field is not a valid sort field.
func GetSortFields(f string) (map[string]string, error) {
	return common.SortFields(eniInfo{}, f)
}

// SortFieldNames returns the valid sort fields, sorted alphabetically.
func SortFieldNames() []string {
	return common.SortFieldNames(eniInfo{})
}
