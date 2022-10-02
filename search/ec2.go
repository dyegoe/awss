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
	"strings"

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

// printJSON returns the instances as JSON
func (instances *Instances) printJSON() {
	json, err := json.Marshal(instances)
	if err != nil {
		log.Default().Printf("[ERROR] marshalling instances: %v", err)
	}
	fmt.Println(string(json))
}

// printTable prints the instances as table
func (instances *Instances) printTable() {
	table := table{
		headers: []string{
			"InstanceID",
			"InstanceName",
			"InstanceType",
			"AvailabilityZone",
			"InstanceState",
			"PrivateIpAddress",
			"PublicIpAddress",
		},
	}
	for _, i := range instances.Instances {
		table.addRow([]string{
			i.InstanceID,
			i.InstanceName,
			i.InstanceType,
			i.AvailabilityZone,
			i.InstanceState,
			i.PrivateIpAddress,
			i.PublicIpAddress,
		})
	}

	table.print()
}

// Search returns the instances
func (instances *Instances) Search(by string, value []string) {
	switch by {
	case "ids":
		instances.searchByIds(value)
	case "names":
		instances.searchByNames(value)
	case "private-ips":
		instances.searchByPrivateIps(value)
	case "public-ips":
		instances.searchByPublicIps(value)
	case "tags":
		instances.searchByTags(value)
	}
}

// Print prints the instances
func (instances *Instances) Print(output string) {
	switch output {
	case "json":
		instances.printJSON()
	case "table":
		instances.printTable()
	case "yaml":
		fmt.Println("Not implemented yet")
	}
}

// searchByIds returns the instances by id
func (instances *Instances) searchByIds(ids []string) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: ids,
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}

// searchByNames returns the instances by name
func (instances *Instances) searchByNames(names []string) {
	instances.searchByTags([]string{fmt.Sprintf("Name=%s", strings.Join(names, ":"))})
}

// searchByPrivateIps returns the instances by private ip
func (instances *Instances) searchByPrivateIps(privateIps []string) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   awsString("private-ip-address"),
				Values: privateIps,
			},
		},
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}

// searchByPublicIps returns the instances by public ip
func (instances *Instances) searchByPublicIps(publicIps []string) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   awsString("ip-address"),
				Values: publicIps,
			},
		},
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}

// searchByTags returns the instances by tag
func (instances *Instances) searchByTags(tags []string) {
	filters := []types.Filter{}
	for _, tag := range tags {
		st := strings.Split(tag, "=")
		sv := strings.Split(st[1], ":")
		filters = append(filters, types.Filter{
			Name:   awsString("tag:" + st[0]),
			Values: sv,
		})
	}
	input := &ec2.DescribeInstancesInput{
		Filters: filters,
	}
	ctx := context.TODO()

	result := instances.getInstances(ctx, input)
	instances.parseInstances(result)
}
