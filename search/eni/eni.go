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

	"github.com/dyegoe/awss/common"
	searchEC2 "github.com/dyegoe/awss/search/ec2"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Results describes results of the ENIs search.
type Results struct {
	// Profile is the profile used to search.
	Profile string `json:"profile"`

	// Region is the region used to search.
	Region string `json:"region"`

	// Errors contains the erros found during the search.
	Errors []string `json:"errors,omitempty"`

	// Data contains the instances found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`

	// SortField is the field used to sort the results.
	SortField string `json:"-"`
}

// dataRow represents a row of the ENIs search results.
type dataRow struct {
	// InterfaceInfo are the network interface infos (ID, type, AZ, status, subnet, instance).
	InterfaceInfo eniInfo `json:"interface_info,omitempty" header:"Interface Info"`

	// PrivateIPAddresses are the private IP addresses assigned to the network interface.
	PrivateIPAddresses []string `json:"private_ip_addresses,omitempty" header:"Private IPs" sort:"private-ips"`

	// PublicIPAddresses are the public IP addresses or Elastic IP addresses bound to the network interface.
	PublicIPAddresses []string `json:"public_ip_addresses,omitempty" header:"Public IPs" sort:"public-ips"`

	// Tags are the tags assigned to the network interface.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// eniInfo represents the network interface info.
type eniInfo struct {
	// NetworkInterfaceID is the ID of the network interface.
	NetworkInterfaceID string `json:"network_interface_id,omitempty" header:"ID"`

	// InterfaceType is the interface type.
	InterfaceType string `json:"interface_type,omitempty" header:"Type"`

	// AvailabilityZone is the AZ of the network interface.
	AvailabilityZone string `json:"availability_zone,omitempty" header:"AZ"`

	// Status is the status of the network interface.
	Status string `json:"status,omitempty" header:"Status"`

	// SubnetID is the ID of the subnet that the network interface is in.
	SubnetID string `json:"subnet_id,omitempty" header:"Subnet"`

	// InstanceID is the ID of the instance that this interface is associate.
	InstanceID string `json:"instance_id,omitempty" header:"Instance"`

	// InstanceName is the name of the instance that this interface is associate.
	InstanceName string `json:"instance_name,omitempty" header:"Instance Name"`
}

// New initiates and returns a new instance of ENI results.
func New(profile, region string, filters map[string][]string, sortField string) *Results {
	return &Results{
		Profile:   profile,
		Region:    region,
		Filters:   filters,
		Errors:    []string{},
		Data:      []dataRow{},
		SortField: sortField,
	}
}

// Search performs the ENIs search.
//
// results are stored in the Data field.
func (r *Results) Search() {
	// Get search filters.
	input := r.getFilters()

	// Get AWS config.
	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %s", err))
		return
	}

	// Get AWS client and describe network interfaces.
	client := ec2.NewFromConfig(cfg)
	response, err := client.DescribeNetworkInterfaces(context.TODO(), input)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error describing network interfaces: %v", err))
		return
	}

	// Parse response.
	for _, eni := range response.NetworkInterfaces {
		row := dataRow{
			InterfaceInfo: eniInfo{
				NetworkInterfaceID: *eni.NetworkInterfaceId,
				InterfaceType:      string(eni.InterfaceType),
				AvailabilityZone:   *eni.AvailabilityZone,
				SubnetID:           *eni.SubnetId,
				Status:             string(eni.Status),
			},
			Tags: common.TagsToMap(eni.TagSet),
		}
		if eni.Attachment != nil {
			if eni.Attachment.InstanceId != nil {
				row.InterfaceInfo.InstanceID = *eni.Attachment.InstanceId
				row.InterfaceInfo.InstanceName, err = searchEC2.SearchInstanceName(r.Profile, r.Region, *eni.Attachment.InstanceId)
				if err != nil {
					r.Errors = append(r.Errors, err.Error())
				}
			}
		}
		if eni.PrivateIpAddresses != nil {
			for _, ip := range eni.PrivateIpAddresses {
				row.PrivateIPAddresses = append(row.PrivateIPAddresses, *ip.PrivateIpAddress)
				if ip.Association != nil {
					row.PublicIPAddresses = append(row.PublicIPAddresses, *ip.Association.PublicIp)
				}
			}
		}
		r.Data = append(r.Data, row)
	}
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetProfile returns the profile used to search.
func (r *Results) GetProfile() string { return r.Profile }

// GetRegion returns the region used to search.
func (r *Results) GetRegion() string { return r.Region }

// GetErrors returns the errors found during the search.
func (r *Results) GetErrors() []string { return r.Errors }

// GetSortField returns the field used to sort the results.
func (r *Results) GetSortField() string { return r.SortField }

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

	for _, row := range r.Data {
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
func (r *Results) getFilters() *ec2.DescribeNetworkInterfacesInput {
	input := ec2.DescribeNetworkInterfacesInput{}

	for key, values := range r.Filters {
		switch key {
		case "network-interface-id":
			input.NetworkInterfaceIds = values
		case "tag":
			input.Filters = append(input.Filters, common.FilterTags(values)...)
		case "availability-zone":
			input.Filters = append(input.Filters, common.FilterAvailabilityZones(values, r.Region)...)
		default:
			input.Filters = append(input.Filters, common.FilterDefault(key, values)...)
		}
	}
	return &input
}
