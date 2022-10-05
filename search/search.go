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
	"awss/logger"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/markkurossi/tabulate"
	"gopkg.in/ini.v1"
)

var l = logger.NewLog()

// search is an interface to search for AWS resources.
type search interface {
	Search(searchBy string, values []string) search
	GetHeaders() []string
	GetRows() [][]string
}

// Run is the main function to run the search
func Run(cmd, searchBy, output string, profile, region, values []string) bool {
	profiles := getProfiles(profile)

	for _, p := range profiles {
		regions := getRegions(region, p)

		for _, r := range regions {

			s := getFunction(cmd, p, r)
			if s == nil {
				l.Errorf("no function found for %s", cmd)
				return false
			}

			response := s.Search(searchBy, values)

			switch output {
			case "json":
				printJson(response)
			case "table":
				printTable(s, p, r)
			}
		}
	}

	return true
}

// getProfiles returns the profiles
func getProfiles(p []string) []string {
	if len(p) == 0 {
		l.Fatalf("no profile provided")
	}
	if p[0] == "all" {
		return getProfilesFromConfig()
	}
	return p
}

// getProfilesFromConfig returns the profiles from the config file
func getProfilesFromConfig() []string {
	fname := config.DefaultSharedConfigFilename()
	f, err := ini.Load(fname)
	arr := []string{}
	if err != nil {
		l.Fatalf("Fail to read file: %v", err)
	} else {
		for _, v := range f.Sections() {
			if strings.HasPrefix(v.Name(), "profile ") {
				arr = append(arr, strings.TrimPrefix(v.Name(), "profile "))
			}
		}
	}
	return arr
}

// getRegions returns the regions
func getRegions(r []string, p string) []string {
	if len(r) == 0 {
		l.Fatalf("no region provided")
	}
	if r[0] == "all" {
		return getOptedInRegions(p)
	}
	return r
}

// getConfig returns a AWS config for the specific profile and region
func getConfig(profile, region string) aws.Config {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		l.Fatalf("unable to load SDK config, %v", err)
	}
	return cfg
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

// printJson returns the instances as JSON
func printJson(s search) {
	json, err := json.Marshal(s)
	if err != nil {
		l.Fatalf("marshalling instances", err)
	}
	fmt.Println(string(json))
}

// printTable prints the instances as a table
func printTable(s search, profile, region string) {
	table := tabulate.New(tabulate.Unicode)
	headers := s.GetHeaders()
	rows := s.GetRows()

	fmt.Println("[+] [profile]:", profile, "[region]:", region)
	if len(rows) == 0 {
		fmt.Println("No results found")
		return
	}

	for _, header := range headers {
		table.Header(header).SetAlign(tabulate.TL)
	}
	for _, r := range rows {
		row := table.Row()
		for _, column := range r {
			row.Column(column)
		}
	}
	table.Print(os.Stdout)
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

// getOptedInRegions returns the opted-in regions
func getOptedInRegions(p string) []string {
	cfg := getConfig(p, "us-east-1")
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
		l.Errorf("error getting regions: %v", err)
		return nil
	}
	regions := []string{}
	for _, r := range response.Regions {
		regions = append(regions, *r.RegionName)
	}
	return regions
}
