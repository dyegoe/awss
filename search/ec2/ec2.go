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

// Package ec2 contains the EC2 search functions.
//
// It implements the common.Results interface
package ec2

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Results describes results of the EC2 instances search.
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

// dataRow represents a row of the EC2 instances search results.
type dataRow struct {
	// InstanceID is the instance ID.
	InstanceID string `json:"instance_id,omitempty" header:"ID" sort:"id"`

	// InstanceName is the tag:Name of the instance.
	InstanceName string `json:"instance_name,omitempty" header:"Name" sort:"name"`

	// InstanceType is the Type of the instance.
	InstanceType string `json:"instance_type,omitempty" header:"Type" sort:"type"`

	// AvailabilityZone is the AZ of the instance.
	AvailabilityZone string `json:"availability_zone,omitempty" header:"AZ" sort:"az"`

	// InstanceState is the current state of the instance.
	InstanceState string `json:"instance_state,omitempty" header:"State" sort:"state"`

	// PrivateIPAddress is the private IP address assigned to the instance.
	PrivateIPAddress string `json:"private_ip_address,omitempty" header:"Private IP" sort:"private_ip"`

	// PublicIPAddress is the public IP address assigned to the instance.
	PublicIPAddress string `json:"public_ip_address,omitempty" header:"Public IP" sort:"public_ip"`

	// NetworkInterfaces are the ENIs attached to the instance.
	NetworkInterfaces []string `json:"network_interfaces,omitempty" header:"ENIs" sort:"enis"`

	// Tags are a map of the tags assigned to the instance.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of EC2 results.
func New(profile, region string, filters map[string][]string, sortField string) *Results {
	return &Results{
		Profile:   profile,
		Region:    region,
		Filters:   filters,
		SortField: sortField,
	}
}

// Search performs the EC2s search.
//
// results are stored in the Data field.
func (r *Results) Search() {
	// Get search filters.
	input := r.getFilters()

	// Get AWS config.
	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	// Get AWS client and describe instances.
	client := ec2.NewFromConfig(cfg)
	response, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	// Parse response.
	for _, i := range response.Reservations {
		for _, inst := range i.Instances {
			enis := []string{}
			for _, eni := range inst.NetworkInterfaces {
				enis = append(enis, *eni.NetworkInterfaceId)
			}
			r.Data = append(r.Data, dataRow{
				InstanceID:        *inst.InstanceId,
				InstanceName:      common.TagName(inst.Tags),
				InstanceType:      string(inst.InstanceType),
				AvailabilityZone:  *inst.Placement.AvailabilityZone,
				InstanceState:     string(inst.State.Name),
				PrivateIPAddress:  common.StringValue(inst.PrivateIpAddress),
				PublicIPAddress:   common.StringValue(inst.PublicIpAddress),
				NetworkInterfaces: enis,
				Tags:              common.TagsToMap(inst.Tags),
			})
		}
	}
	err = r.sortResults(r.SortField)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// Len returns the length of the results.
func (r *Results) Len() int {
	return len(r.Data)
}

// GetProfile returns the profile used to search.
func (r *Results) GetProfile() string {
	return r.Profile
}

// GetRegion returns the region used to search.
func (r *Results) GetRegion() string {
	return r.Region
}

// GetErrors returns the errors found during the search.
func (r *Results) GetErrors() []string {
	return r.Errors
}

// GetSortField returns the field used to sort the results.
func (r *Results) GetSortField() string {
	return r.SortField
}

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
// Except for "ids", "names", "tags" and "availability-zones", all other filters are passed as it is.
// If no filters are given, it returns an empty list.
func (r *Results) getFilters() *ec2.DescribeInstancesInput {
	input := ec2.DescribeInstancesInput{}

	for key, values := range r.Filters {
		switch key {
		case "instance-id":
			input.InstanceIds = values
		case "tag:Name":
			input.Filters = append(input.Filters, common.FilterNames(values)...)
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

// sortResults sorts the results by the given field.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}

	sort.Slice(r.Data, func(p, q int) bool {
		return reflect.ValueOf(r.Data[p]).FieldByName(sortFields[field]).String() < reflect.ValueOf(r.Data[q]).FieldByName(sortFields[field]).String()
	})
	return nil
}

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The sort fields are defined in the struct tag `sort`.
// The function returns an error if the given field is not a valid sort field.
func GetSortFields(f string) (map[string]string, error) {
	sortFields := map[string]string{}

	v := reflect.ValueOf(dataRow{})
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)

		if sort, ok := field.Tag.Lookup("sort"); ok {
			sortFields[sort] = field.Name
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

// SearchInstanceName returns the name of an instance.
//
// It returns the value of the tag:Name or empty string in case that the instance has no name.
func SearchInstanceName(profile, region, instanceID string) (string, error) {
	r := New(profile, region, map[string][]string{"instance-id": {instanceID}}, "id")
	r.Search()
	if len(r.Errors) > 0 {
		return "", fmt.Errorf("error searching instance name: %v", r.Errors)
	}

	switch r.Len() {
	case 0:
		return "", fmt.Errorf("instance %s not found", instanceID)
	case 1:
		return r.Data[0].InstanceName, nil
	default:
		return "", fmt.Errorf("more than one instance found")
	}
}
