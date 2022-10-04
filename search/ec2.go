/*
Copyright © 2022 Dyego Alexandre Eugenio dyegoe@gmail.com

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
package search

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Instances is a struct to hold the instances
type Instances struct {
	Profile   string     `json:"profile"`
	Region    string     `json:"region"`
	Instances []instance `json:"instances"`
}

// instances is a struct to hold the instance
type instance struct {
	InstanceID       string `json:"instance_id"`
	InstanceName     string `json:"instance_name"`
	InstanceType     string `json:"instance_type"`
	AvailabilityZone string `json:"availability_zone"`
	InstanceState    string `json:"instance_state"`
	PrivateIpAddress string `json:"private_ip_address"`
	PublicIpAddress  string `json:"public_ip_address"`
}

// Search is a method to search for instances. It gets instances from API and update the struct with the data.
func (i *Instances) Search(searchBy string, values []string, responseChan chan<- search) {
	var input *ec2.DescribeInstancesInput
	switch searchBy {
	case "ids":
		input = i.filterByIds(values)
	case "names":
		input = i.filterByNames(values)
	case "private-ips":
		input = i.filterByPrivateIps(values)
	case "public-ips":
		input = i.filterByPublicIps(values)
	case "tags":
		input = i.filterByTags(values)
	}
	result := i.getInstances(input)
	i.parseInstances(result)
	responseChan <- i
}

// filterByIds returns filters by id
func (i *Instances) filterByIds(ids []string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		InstanceIds: ids,
	}
}

// filterByNames returns filters by name
func (i *Instances) filterByNames(names []string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: names,
			},
		},
	}
}

// filterByPrivateIps returns filters by private ip
func (i *Instances) filterByPrivateIps(privateIps []string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("private-ip-address"),
				Values: privateIps,
			},
		},
	}
}

// filterByPublicIps returns filters by public ip
func (i *Instances) filterByPublicIps(publicIps []string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("ip-address"),
				Values: publicIps,
			},
		},
	}
}

// filterByTags returns filters by tag
func (i *Instances) filterByTags(tags []string) *ec2.DescribeInstancesInput {
	filters := []types.Filter{}
	for _, tag := range tags {
		st := strings.Split(tag, "=")
		sv := strings.Split(st[1], ":")
		filters = append(filters, types.Filter{
			Name:   aws.String("tag:" + st[0]),
			Values: sv,
		})
	}
	return &ec2.DescribeInstancesInput{
		Filters: filters,
	}
}

// getInstances returns the instances
func (i *Instances) getInstances(input *ec2.DescribeInstancesInput) *ec2.DescribeInstancesOutput {
	client := ec2.NewFromConfig(getConfig(i.Profile, i.Region))
	response, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		l.Errorf("error getting instances: %v", err)
		return nil
	}
	return response
}

// parseInstances parses the instances
func (i *Instances) parseInstances(result *ec2.DescribeInstancesOutput) {
	for _, r := range result.Reservations {
		for _, inst := range r.Instances {
			i.Instances = append(i.Instances, instance{
				InstanceID:       *inst.InstanceId,
				InstanceName:     getTagName(inst.Tags),
				InstanceType:     string(inst.InstanceType),
				AvailabilityZone: *inst.Placement.AvailabilityZone,
				InstanceState:    string(inst.State.Name),
				PrivateIpAddress: *inst.PrivateIpAddress,
				PublicIpAddress:  *inst.PublicIpAddress,
			})
		}
	}
}

// getTagName returns the value of the tag Name
func getTagName(tags []types.Tag) string {
	for _, tag := range tags {
		if *tag.Key == "Name" {
			return *tag.Value
		}
	}
	return ""
}
