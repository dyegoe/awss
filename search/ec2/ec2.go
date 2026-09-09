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

	"github.com/dyegoe/awss/common"
	searchSubnet "github.com/dyegoe/awss/search/subnet"
	searchVPC "github.com/dyegoe/awss/search/vpc"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	// filterKeyInstanceID is the AWS EC2 "instance-id" filter key.
	filterKeyInstanceID = "instance-id"

	// FilterKeyCIDR is the pseudo-filter key carrying --cidrs values.
	//
	// It is not an AWS filter: Search resolves it into "subnet-id" or "vpc-id" per profile
	// and region before calling DescribeInstances.
	FilterKeyCIDR = "cidr"

	// filterKeySubnetID and filterKeyVpcID are the AWS EC2 filter keys the CIDR resolves into.
	// They match the instance's primary network interface.
	filterKeySubnetID = "subnet-id"
	filterKeyVpcID    = "vpc-id"
)

// subnetIDsByCIDR and vpcIDsByCIDR look up subnet and VPC IDs by CIDR.
//
// They are variables so tests can replace the AWS calls.
var (
	subnetIDsByCIDR = searchSubnet.IDsByCIDR
	vpcIDsByCIDR    = searchVPC.IDsByCIDR
)

// Results describes results of the EC2 instances search.
type Results struct {
	common.BaseResults

	// Data contains the instances found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`
}

// dataRow represents a row of the EC2 instances search results.
type dataRow struct {
	// InstanceID is the instance ID.
	InstanceID string `json:"id,omitempty" header:"ID" sort:"id"`

	// InstanceName is the tag:Name of the instance.
	InstanceName string `json:"name,omitempty" header:"Name" sort:"name"`

	// InstanceType is the Type of the instance.
	InstanceType string `json:"type,omitempty" header:"Type" sort:"type"`

	// AvailabilityZone is the AZ of the instance.
	AvailabilityZone string `json:"az,omitempty" header:"AZ" sort:"az"`

	// InstanceState is the current state of the instance.
	InstanceState string `json:"state,omitempty" header:"State" sort:"state"`

	// PrivateIPAddress is the private IP address assigned to the instance.
	PrivateIPAddress string `json:"private_ip,omitempty" header:"Private IP" sort:"private-ip"`

	// PublicIPAddress is the public IP address assigned to the instance.
	PublicIPAddress string `json:"public_ip,omitempty" header:"Public IP" sort:"public-ip"`

	// NetworkInterfaces are the ENIs attached to the instance.
	NetworkInterfaces []string `json:"enis,omitempty" header:"ENIs" sort:"enis"`

	// Volumes are the EBS volume IDs attached to the instance.
	Volumes []string `json:"volumes,omitempty" header:"Volumes" sort:"volumes"`

	// Tags are a map of the tags assigned to the instance.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of EC2 results.
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

// Search performs the EC2s search.
//
// results are stored in the Data field.
func (r *Results) Search(ctx context.Context) {
	// Resolve the CIDR pseudo-filter, if any, into subnet or VPC IDs for this region.
	filters, err := r.resolveCIDRFilter(ctx)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	// Get search filters.
	input, err := filtersToInput(filters, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	// Get AWS config.
	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	// Get AWS client and describe instances.
	client := ec2.NewFromConfig(cfg)
	response, err := client.DescribeInstances(ctx, input)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}

	// Parse response.
	for _, i := range response.Reservations {
		for _, inst := range i.Instances { //nolint:gocritic
			r.Data = append(r.Data, parseInstance(&inst))
		}
	}
	if err = r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// parseInstance converts a single EC2 Instance into a dataRow.
func parseInstance(inst *types.Instance) dataRow {
	enis := make([]string, 0, len(inst.NetworkInterfaces))
	for _, eni := range inst.NetworkInterfaces { //nolint:gocritic
		enis = append(enis, common.StringValue(eni.NetworkInterfaceId))
	}
	volumes := parseVolumeIDs(inst.BlockDeviceMappings)
	var az string
	if inst.Placement != nil {
		az = common.StringValue(inst.Placement.AvailabilityZone)
	}
	var state string
	if inst.State != nil {
		state = string(inst.State.Name)
	}
	return dataRow{
		InstanceID:        common.StringValue(inst.InstanceId),
		InstanceName:      common.TagName(inst.Tags),
		InstanceType:      string(inst.InstanceType),
		AvailabilityZone:  az,
		InstanceState:     state,
		PrivateIPAddress:  common.StringValue(inst.PrivateIpAddress),
		PublicIPAddress:   common.StringValue(inst.PublicIpAddress),
		NetworkInterfaces: enis,
		Volumes:           volumes,
		Tags:              common.TagsToMap(inst.Tags),
	}
}

// parseVolumeIDs returns the EBS volume IDs from the instance block device mappings.
//
// Mappings without an EBS block or without a volume ID are skipped.
// It returns nil when no volume is attached so the JSON output omits the field.
func parseVolumeIDs(mappings []types.InstanceBlockDeviceMapping) []string {
	var volumes []string
	for _, m := range mappings {
		if m.Ebs == nil || m.Ebs.VolumeId == nil {
			continue
		}
		volumes = append(volumes, *m.Ebs.VolumeId)
	}
	return volumes
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the dataRow fields.
func (r *Results) GetHeaders() []interface{} { return common.Headers(dataRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} { return common.Rows(r.Data) }

// resolveCIDRFilter returns the filters to search with, replacing the CIDR pseudo-filter.
//
// Without a CIDR filter it returns r.Filters untouched. Otherwise it returns a copy where
// "cidr" is replaced by "subnet-id" when at least one subnet has one of the CIDRs, or by
// "vpc-id" when no subnet matches but at least one VPC does. When neither matches it returns
// an error, since no instance can match. r.Filters is shared across goroutines and never mutated.
func (r *Results) resolveCIDRFilter(ctx context.Context) (map[string][]string, error) {
	cidrs, ok := r.Filters[FilterKeyCIDR]
	if !ok {
		return r.Filters, nil
	}

	resolved := make(map[string][]string, len(r.Filters))
	for key, values := range r.Filters {
		if key != FilterKeyCIDR {
			resolved[key] = values
		}
	}

	subnetIDs, err := subnetIDsByCIDR(ctx, r.Profile, r.Region, cidrs)
	if err != nil {
		return nil, fmt.Errorf("resolving CIDR filter: %w", err)
	}
	if len(subnetIDs) > 0 {
		resolved[filterKeySubnetID] = subnetIDs
		return resolved, nil
	}

	vpcIDs, err := vpcIDsByCIDR(ctx, r.Profile, r.Region, cidrs)
	if err != nil {
		return nil, fmt.Errorf("resolving CIDR filter: %w", err)
	}
	if len(vpcIDs) > 0 {
		resolved[filterKeyVpcID] = vpcIDs
		return resolved, nil
	}

	return nil, fmt.Errorf("no subnet or VPC found for CIDR %s in %s",
		common.StringSliceToString(cidrs, ", "), r.Region)
}

// getFilters returns the DescribeInstances input built from r.Filters.
//
// It is kept for callers that have no CIDR pseudo-filter; Search uses filtersToInput on the
// resolved filters instead.
func (r *Results) getFilters() (*ec2.DescribeInstancesInput, error) {
	return filtersToInput(r.Filters, r.Region)
}

// filtersToInput converts a filters map into a DescribeInstancesInput.
//
// This function expects the filters to be in the format used by the AWS SDK.
// Except for "instance-id", "tag:Name", "tag" and "availability-zone", all other filters are
// passed as they are. The "cidr" pseudo-filter must have been resolved first and is rejected.
func filtersToInput(filters map[string][]string, region string) (*ec2.DescribeInstancesInput, error) {
	input := ec2.DescribeInstancesInput{}

	for key, values := range filters {
		switch key {
		case FilterKeyCIDR:
			return nil, fmt.Errorf("filter %q must be resolved into subnet or VPC IDs before searching", key)
		case filterKeyInstanceID:
			input.InstanceIds = values
		case "tag:Name":
			input.Filters = append(input.Filters, common.FilterNames(values)...)
		case "tag":
			tagFilters, err := common.FilterTags(values)
			if err != nil {
				return nil, fmt.Errorf("building tag filters: %w", err)
			}
			input.Filters = append(input.Filters, tagFilters...)
		case "availability-zone":
			input.Filters = append(input.Filters, common.FilterAvailabilityZones(values, region)...)
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

// SearchInstanceNames returns a map of instanceID to instance name for all given IDs.
// It makes a single DescribeInstances API call instead of one per ID.
func SearchInstanceNames(profile, region string, instanceIDs []string) (map[string]string, error) {
	if len(instanceIDs) == 0 {
		return map[string]string{}, nil
	}
	r := New(profile, region, map[string][]string{filterKeyInstanceID: instanceIDs}, "id")
	r.Search(context.Background())
	if len(r.Errors) > 0 {
		return nil, fmt.Errorf("error searching instance names: %v", r.Errors)
	}
	names := make(map[string]string, len(r.Data))
	for i := range r.Data {
		names[r.Data[i].InstanceID] = r.Data[i].InstanceName
	}
	return names, nil
}
