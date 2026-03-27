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
}

// New initiates and returns a new instance of ENI results.
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

	// Get AWS client and describe network interfaces.
	client := ec2.NewFromConfig(cfg)
	response, err := client.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error describing network interfaces: %v", err))
		return
	}

	// Parse response and collect instance IDs for batch lookup.
	var instanceIDs []string
	for _, eni := range response.NetworkInterfaces { //nolint:gocritic
		r.Data = append(r.Data, parseENIRow(&eni))
		if eni.Attachment != nil && eni.Attachment.InstanceId != nil {
			instanceIDs = append(instanceIDs, *eni.Attachment.InstanceId)
		}
	}

	// Batch lookup instance names in a single API call.
	if len(instanceIDs) > 0 {
		names, err := searchEC2.SearchInstanceNames(r.Profile, r.Region, instanceIDs)
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
		} else {
			for i := range r.Data {
				if id := r.Data[i].InterfaceInfo.InstanceID; id != "" {
					r.Data[i].InterfaceInfo.InstanceName = names[id]
				}
			}
		}
	}

	if r.SortField != "" {
		if err := r.sortResults(r.SortField); err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
	}
}

// parseENIRow converts a single NetworkInterface into a dataRow.
func parseENIRow(eni *types.NetworkInterface) dataRow {
	row := dataRow{
		InterfaceInfo: eniInfo{
			NetworkInterfaceID: common.StringValue(eni.NetworkInterfaceId),
			InterfaceType:      string(eni.InterfaceType),
			AvailabilityZone:   common.StringValue(eni.AvailabilityZone),
			SubnetID:           common.StringValue(eni.SubnetId),
			Status:             string(eni.Status),
		},
		Tags: common.TagsToMap(eni.TagSet),
	}
	if eni.Attachment != nil && eni.Attachment.InstanceId != nil {
		row.InterfaceInfo.InstanceID = *eni.Attachment.InstanceId
	}
	for _, ip := range eni.PrivateIpAddresses {
		row.PrivateIPAddresses = append(row.PrivateIPAddresses, common.StringValue(ip.PrivateIpAddress))
		if ip.Association != nil {
			row.PublicIPAddresses = append(row.PublicIPAddresses, common.StringValue(ip.Association.PublicIp))
		}
	}
	return row
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the the tag `header` of the struct fields.
func (r *Results) GetHeaders() []interface{} {
	headers := []interface{}{}

	v := reflect.ValueOf(dataRow{})
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)

		if header, ok := field.Tag.Lookup("header"); ok {
			headers = append(headers, header)
		}
	}

	return headers
}

// GetRows iterates results.Data and returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} {
	rows := []interface{}{}

	for _, row := range r.Data { //nolint:gocritic
		rows = append(rows, row)
	}
	return rows
}

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

// sortResults sorts the results by the given field.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}

	sort.Slice(r.Data, func(p, q int) bool {
		pInfo := reflect.ValueOf(r.Data[p].InterfaceInfo)
		qInfo := reflect.ValueOf(r.Data[q].InterfaceInfo)
		return pInfo.FieldByName(sortFields[field]).String() < qInfo.FieldByName(sortFields[field]).String()
	})
	return nil
}

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The sort fields are defined in the struct tag `sort` on eniInfo.
// The function returns an error if the given field is not a valid sort field.
func GetSortFields(f string) (map[string]string, error) {
	sortFields := map[string]string{}

	v := reflect.ValueOf(eniInfo{})
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)

		if s, ok := field.Tag.Lookup("sort"); ok {
			sortFields[s] = field.Name
		}
	}

	options := []string{}
	for k := range sortFields {
		options = append(options, k)
	}

	if _, ok := sortFields[f]; !ok {
		return nil, fmt.Errorf("invalid sort field: %s. The options are: %s", f, common.StringSliceToString(options, ", "))
	}
	return sortFields, nil
}
