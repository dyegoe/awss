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

// Search is a method to search for instances
func (i *Instances) Search(searchBy string, values []string, responseChan chan<- search) {
	switch searchBy {
	// case "ids":
	// 	i.searchByIds(values)
	// case "names":
	// 	i.searchByNames(values)
	// case "private-ips":
	// 	i.searchByPrivateIps(values)
	// case "public-ips":
	// 	i.searchByPublicIps(values)
	case "tags":
		i.searchByTags(values)
	}
	responseChan <- i
}

// searchByTags returns the instances by tag
func (i *Instances) searchByTags(tags []string) {
	filters := []types.Filter{}
	for _, tag := range tags {
		st := strings.Split(tag, "=")
		sv := strings.Split(st[1], ":")
		filters = append(filters, types.Filter{
			Name:   aws.String("tag:" + st[0]),
			Values: sv,
		})
	}
	input := &ec2.DescribeInstancesInput{
		Filters: filters,
	}
	result := i.getInstances(input)
	_ = result
	i.parseInstances(result)
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
func (instances *Instances) parseInstances(result *ec2.DescribeInstancesOutput) {
	for _, r := range result.Reservations {
		for _, i := range r.Instances {
			instances.Instances = append(instances.Instances, instance{
				InstanceID:       *i.InstanceId,
				InstanceName:     getTagName(i.Tags),
				InstanceType:     string(i.InstanceType),
				AvailabilityZone: *i.Placement.AvailabilityZone,
				InstanceState:    string(i.State.Name),
				PrivateIpAddress: *i.PrivateIpAddress,
				PublicIpAddress:  *i.PublicIpAddress,
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
