package search

import (
	"context"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// instances is a struct to hold the instances
type instances struct {
	Profile string     `json:"profile"`
	Region  string     `json:"region"`
	Error   string     `json:"error,omitempty"`
	Data    []instance `json:"data"`
}

// instances is a struct to hold the instance data
type instance struct {
	InstanceID       string `json:"instance_id"`
	InstanceName     string `json:"instance_name"`
	InstanceType     string `json:"instance_type"`
	AvailabilityZone string `json:"availability_zone"`
	InstanceState    string `json:"instance_state"`
	PrivateIPAddress string `json:"private_ip_address"`
	PublicIPAddress  string `json:"public_ip_address"`
}

// search is a method to search for instances. It gets instances from API and update the struct with the data.
func (i *instances) search(searchBy map[string][]string) {
	input := i.getFilters(searchBy)
	result, err := i.getInstances(input)
	if err != nil {
		i.Error = err.Error()
	}
	i.Data = i.parseInstances(result)
}

// getFilters returns the filters
func (i *instances) getFilters(searchBy map[string][]string) *ec2.DescribeInstancesInput {
	var input = ec2.DescribeInstancesInput{}

	for key, values := range searchBy {
		switch key {
		case "ids":
			input.InstanceIds = values
		case "names":
			input.Filters = append(input.Filters, i.filterByNames(values)...)
		case "tags":
			input.Filters = append(input.Filters, i.filterByTags(values)...)
		case "instance-types":
			input.Filters = append(input.Filters, i.filterByInstanceTypes(values)...)
		case "availability-zones":
			input.Filters = append(input.Filters, i.filterByAvailabilityZones(values)...)
		case "instance-states":
			input.Filters = append(input.Filters, i.filterByInstanceStates(values)...)
		case "private-ips":
			input.Filters = append(input.Filters, i.filterByPrivateIps(values)...)
		case "public-ips":
			input.Filters = append(input.Filters, i.filterByPublicIps(values)...)
		}
	}
	return &input
}

// filterByNames returns filters by name
func (i *instances) filterByNames(names []string) []types.Filter {
	return []types.Filter{
		{
			Name:   aws.String("tag:Name"),
			Values: names,
		},
	}
}

// filterByTags returns filters by tag
func (i *instances) filterByTags(tags []string) []types.Filter {
	filters := []types.Filter{}
	parsed, _ := ParseTags(tags)
	for key, values := range parsed {
		filters = append(filters, types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: values,
		})
	}
	return filters
}

// filterByInstanceTypes returns filters by instance type
func (i *instances) filterByInstanceTypes(instanceTypes []string) []types.Filter {
	return []types.Filter{
		{
			Name:   aws.String("instance-type"),
			Values: instanceTypes,
		},
	}
}

// filterByAvailabilityZones returns filters by availability zone
func (i *instances) filterByAvailabilityZones(availabilityZones []string) []types.Filter {
	var azs []string
	for _, value := range availabilityZones {
		azs = append(azs, fmt.Sprintf("%s%s", i.Region, value))
	}
	return []types.Filter{
		{
			Name:   aws.String("availability-zone"),
			Values: azs,
		},
	}
}

// filterByInstanceStates returns filters by instance state
func (i *instances) filterByInstanceStates(instanceStates []string) []types.Filter {
	return []types.Filter{
		{
			Name:   aws.String("instance-state-name"),
			Values: instanceStates,
		},
	}
}

// filterByPrivateIps returns filters by private ip
func (i *instances) filterByPrivateIps(privateIps []string) []types.Filter {
	return []types.Filter{
		{
			Name:   aws.String("private-ip-address"),
			Values: privateIps,
		},
	}
}

// filterByPublicIps returns filters by public ip
func (i *instances) filterByPublicIps(publicIps []string) []types.Filter {
	return []types.Filter{
		{
			Name:   aws.String("ip-address"),
			Values: publicIps,
		},
	}
}

// getInstances returns the instances
func (i *instances) getInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	cfg, err := getAwsConfig(i.Profile, i.Region)
	if err != nil {
		return &ec2.DescribeInstancesOutput{}, fmt.Errorf("error getting config: %v", err)
	}
	client := ec2.NewFromConfig(cfg)
	response, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		return &ec2.DescribeInstancesOutput{}, fmt.Errorf("error getting instances: %v", err)
	}
	return response, nil
}

// parseInstances parses the instances
func (i *instances) parseInstances(result *ec2.DescribeInstancesOutput) []instance {
	data := []instance{}
	for _, r := range result.Reservations {
		for _, inst := range r.Instances {
			data = append(data, instance{
				InstanceID:       *inst.InstanceId,
				InstanceName:     getTagName(inst.Tags),
				InstanceType:     string(inst.InstanceType),
				AvailabilityZone: *inst.Placement.AvailabilityZone,
				InstanceState:    string(inst.State.Name),
				PrivateIPAddress: getValue(inst.PrivateIpAddress),
				PublicIPAddress:  getValue(inst.PublicIpAddress),
			})
		}
	}
	return data
}

// getHeaders returns the headers
func (i *instances) getHeaders() []string {
	headers := []string{}
	val := reflect.ValueOf(instance{})
	for i := 0; i < val.Type().NumField(); i++ {
		headers = append(headers, val.Type().Field(i).Name)
	}
	return headers
}

// getRows returns the rows
func (i *instances) getRows() [][]string {
	rows := [][]string{}
	for _, v := range i.Data {
		row := []string{}
		val := reflect.ValueOf(v)
		for i := 0; i < val.Type().NumField(); i++ {
			row = append(row, val.Field(i).String())
		}
		rows = append(rows, row)
	}
	return rows
}

// getProfile returns the profile
func (i *instances) getProfile() string {
	return i.Profile
}

// getRegion returns the region
func (i *instances) getRegion() string {
	return i.Region
}

// getError returns the error
func (i *instances) getError() string {
	return i.Error
}
