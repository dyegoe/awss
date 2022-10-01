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
	"encoding/json"
	"fmt"
	"log"

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
	InstanceState    string `json:"instance_state"`
	InstanceName     string `json:"instance_name"`
	InstanceID       string `json:"instance_id"`
	InstanceType     string `json:"instance_type"`
	AvailabilityZone string `json:"availability_zone"`
	PrivateIpAddress string `json:"private_ip_address"`
	PublicIpAddress  string `json:"public_ip_address"`
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

// getEC2Client returns a new ec2 client
func getEC2Client(profile, region string) (*ec2.Client, error) {
	cfg, err := getConfig(profile, region)
	if err != nil {
		return nil, err
	}

	return ec2.NewFromConfig(cfg), nil
}

// getInstances returns the instances
func (instances *Instances) getInstances(c context.Context, input *ec2.DescribeInstancesInput) *ec2.DescribeInstancesOutput {
	client, err := getEC2Client(instances.Profile, instances.Region)
	if err != nil {
		log.Default().Printf("[ERROR] getting ec2 client: %v", err)
	}
	response, err := client.DescribeInstances(c, input)
	if err != nil {
		log.Default().Printf("[ERROR] getting instances: %v", err)
		return nil
	}
	return response
}

// parseInstances parses the instances
func (instances *Instances) parseInstances(result *ec2.DescribeInstancesOutput) {
	for _, r := range result.Reservations {
		for _, i := range r.Instances {
			instances.Instances = append(instances.Instances, instance{
				InstanceState:    string(i.State.Name),
				InstanceName:     getTagName(i.Tags),
				InstanceID:       *i.InstanceId,
				InstanceType:     string(i.InstanceType),
				AvailabilityZone: *i.Placement.AvailabilityZone,
				PrivateIpAddress: *i.PrivateIpAddress,
				PublicIpAddress:  *i.PublicIpAddress,
			})
		}
	}
}

// JSON returns the instances as JSON
func (instances *Instances) JSON() {
	json, err := json.Marshal(instances)
	if err != nil {
		log.Default().Printf("[ERROR] marshalling instances: %v", err)
	}
	fmt.Println(string(json))
}

// Search returns the instances
func (instances *Instances) Search(by string, value []string) {
	switch by {
	case "ids":
		instances.instancesByIds(value)
	case "names":
		instances.instancesByNames(value)
	case "private-ips":
		instances.instancesByPrivateIps(value)
	case "public-ips":
		instances.instancesByPublicIps(value)
	}
}

// instancesByIds returns the instances by id
func (instances *Instances) instancesByIds(ids []string) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: ids,
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}

// instancesByNames returns the instances by name
func (instances *Instances) instancesByNames(names []string) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: names,
			},
		},
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}

// instancesByPrivateIps returns the instances by private ip
func (instances *Instances) instancesByPrivateIps(privateIps []string) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("private-ip-address"),
				Values: privateIps,
			},
		},
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}

// instancesByPublicIps returns the instances by public ip
func (instances *Instances) instancesByPublicIps(publicIps []string) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("ip-address"),
				Values: publicIps,
			},
		},
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}
