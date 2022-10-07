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
	"fmt"
	"strings"

	"github.com/dyegoe/awss/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"gopkg.in/ini.v1"
)

var l = logger.NewLog()

// search is an interface to search for AWS resources.
type search interface {
	Search(searchBy string, values []string) search
	GetProfile() string
	GetRegion() string
	GetHeaders() []string
	GetRows() [][]string
}

// Run is the main function to run the search
func Run(profile, region []string, output, cmd, searchBy string, values []string) error {
	profiles, err := getProfiles(profile)
	if err != nil {
		return err
	}

	for _, p := range profiles {
		regions, err := getRegions(region, p)
		if err != nil {
			return err
		}

		for _, r := range regions {

			s := getFunction(cmd, p, r)
			if s == nil {
				return fmt.Errorf("no function found for %s", cmd)
			}

			response := s.Search(searchBy, values)

			switch output {
			case "table":
				printTable(s)
			case "json":
				printJson(response)
			case "json-pretty":
				printJsonPretty(response)
			}
		}
	}

	return nil
}

// getProfiles returns the profiles
func getProfiles(p []string) ([]string, error) {
	profiles, err := getProfilesFromConfig()
	if err != nil {
		return nil, err
	}
	if p[0] == "all" {
		return profiles, nil
	}
	for _, profile := range p {
		if !stringInSlice(profile, profiles) {
			return nil, fmt.Errorf("profile %s not found", profile)
		}
	}
	return p, nil
}

// getProfilesFromConfig returns the profiles from the config file
func getProfilesFromConfig() ([]string, error) {
	f, err := ini.Load(config.DefaultSharedConfigFilename())
	if err != nil {
		return nil, fmt.Errorf("fail to read file: %v", err)
	}
	arr := []string{}
	for _, v := range f.Sections() {
		if strings.HasPrefix(v.Name(), "profile ") {
			arr = append(arr, strings.TrimPrefix(v.Name(), "profile "))
		}
	}
	return arr, nil
}

// getRegions returns the regions
func getRegions(r []string, p string) ([]string, error) {
	regions, err := getOptedInRegions(p)
	if err != nil {
		return nil, err
	}
	if r[0] == "all" {
		return regions, nil
	}
	for _, region := range r {
		if !stringInSlice(region, regions) {
			return nil, fmt.Errorf("region %s not found", region)
		}
	}
	return r, nil
}

// getOptedInRegions returns the opted-in regions
func getOptedInRegions(p string) ([]string, error) {
	cfg, err := getAwsConfig(p, "us-east-1")
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)
	response, err := client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("opt-in-status"),
				Values: []string{"opt-in-not-required", "opted-in"},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error getting regions: %v", err)
	}
	regions := []string{}
	for _, r := range response.Regions {
		regions = append(regions, *r.RegionName)
	}
	return regions, nil
}

// getAwsConfig returns a AWS config for the specific profile and region
func getAwsConfig(profile, region string) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return cfg, fmt.Errorf("unable to load SDK config: %v", err)
	}
	return cfg, nil
}

// getFunction returns the function to search for the specific resource
func getFunction(cmd, profile, region string) search {
	switch cmd {
	case "ec2":
		return &Instances{
			Profile: profile,
			Region:  region,
		}
	default:
		return nil
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

// getValue returns the string value if not nil
func getValue(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// stringInSlice returns true if the string is in the slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// ParseTags parses the tags to a map
func ParseTags(tags []string) (map[string][]string, error) {
	m := make(map[string][]string)
	for _, tag := range tags {
		splited := strings.Split(tag, "=")
		if len(splited) != 2 {
			return nil, fmt.Errorf("invalid tag: %s", tag)
		}
		key := splited[0]
		values := strings.Split(splited[1], ":")
		m[key] = values
	}
	return m, nil
}
