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

// Package common contains common functions and types.
//
// It has AWS related functions and types.
// It also has functions to print the results in different formats.
package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"gopkg.in/ini.v1"
)

// AwsConfig returns a AWS config for the specific profile and region.
func AwsConfig(profile, region string) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

// WhoAmI returns the AWS account ID and error.
//
// The profile and region are used to create the AWS config.
// The AWS account ID is returned by the STS GetCallerIdentity API.
// This function is used as workaround to pre-authenticate the AWS config.
func WhoAmI(profile, region string) (string, error) {
	cfg, err := AwsConfig(profile, region)
	if err != nil {
		return "", err
	}
	client := sts.NewFromConfig(cfg)
	resp, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return *resp.Account, nil
}

// GetAwsProfiles returns a list of profiles from the AWS config file.
func GetAwsProfiles() ([]string, error) {
	config, err := ini.Load(config.DefaultSharedConfigFilename())
	if err != nil {
		return nil, err
	}
	profiles := []string{}
	for _, section := range config.Sections() {
		if strings.HasPrefix(section.Name(), "profile ") {
			profiles = append(profiles, strings.TrimPrefix(section.Name(), "profile "))
		}
	}
	return profiles, nil
}

// TagName returns the value of the tag:Name from a slice of types.Tag.
func TagName(tags []types.Tag) string {
	for _, tag := range tags {
		if *tag.Key == "Name" {
			return *tag.Value
		}
	}
	return ""
}

// TagsToMap takes a slice of types.Tag and returns a map of tags and values.
func TagsToMap(tags []types.Tag) map[string]string {
	data := map[string]string{}
	for _, t := range tags {
		data[*t.Key] = *t.Value
	}
	return data
}

// FilterNames returns a list of types.Filter where the filter Name is tag:Name and the names are the Values.
func FilterNames(names []string) []types.Filter {
	if len(names) == 0 {
		return []types.Filter{}
	}
	return []types.Filter{
		{
			Name:   String("tag:Name"),
			Values: names,
		},
	}
}

// FilterTags returns a list of types.Filter by tag:Key=Value1,Value2,Value3...
func FilterTags(tags []string) []types.Filter {
	filters := []types.Filter{}
	if len(tags) == 0 {
		return filters
	}
	parsed, err := ParseTags(tags)
	if err != nil {
		return filters
	}
	for key, values := range parsed {
		filters = append(filters, types.Filter{
			Name:   String(fmt.Sprintf("tag:%s", key)),
			Values: values,
		})
	}
	return filters
}

// FilterAvailabilityZones returns a list of types.Filter by availability-zone.
//
// The availabilityZones must be a list of letters that represent the availability zone.
// For example: "a", "b", "c". The region is used to get the full availability zone name.
func FilterAvailabilityZones(availabilityZones []string, region string) []types.Filter {
	if len(availabilityZones) == 0 {
		return []types.Filter{}
	}
	options := []string{"a", "b", "c", "d", "e", "f"}
	azs := []string{}
	for _, value := range availabilityZones {
		if !StringInSlice(value, options) {
			continue
		}
		azs = append(azs, fmt.Sprintf("%s%s", region, value))
	}
	if len(azs) == 0 {
		return []types.Filter{}
	}
	return []types.Filter{
		{
			Name:   String("availability-zone"),
			Values: azs,
		},
	}
}

// FilterDefault returns a list of types.Filter. The key is used as filter Name and the values as Values.
func FilterDefault(key string, values []string) []types.Filter {
	if key == "" || len(values) == 0 {
		return []types.Filter{}
	}
	return []types.Filter{
		{
			Name:   String(key),
			Values: values,
		},
	}
}
