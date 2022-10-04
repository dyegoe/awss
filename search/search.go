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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

var l = logger.NewLog()

// search is an interface to search for AWS resources.
type search interface {
	Search(searchBy string, values []string, responseChan chan<- search)
}

// Run is the main function to run the search
func Run(cmd, searchBy, profile, region string, values []string) bool {
	profiles := getProfiles(profile)
	regions := getRegions(region)

	responseChan := make(chan search)

	for _, p := range profiles {
		for _, r := range regions {
			s := getFunction(cmd, p, r)
			if s == nil {
				l.Errorf("no function found for %s", cmd)
				return false
			}
			go s.Search(searchBy, values, responseChan)
		}
	}
	for i := 0; i < len(profiles)*len(regions); i++ {
		response := <-responseChan
		printJson(response)
	}
	return true
}

// getProfiles returns the profiles
func getProfiles(p string) []string {
	if p == "" {
		l.Errorf("no profile provided")
	}
	parsed := strings.Split(p, ",")
	if parsed[0] == "all" {
		return []string{"default", "dyego"}
	}
	return parsed
}

// getRegions returns the regions
func getRegions(p string) []string {
	if p == "" {
		l.Errorf("no region provided")
	}
	parsed := strings.Split(p, ",")
	if parsed[0] == "all" {
		return []string{"eu-central-1", "sa-east-1"}
	}
	return parsed
}

// getConfig returns a AWS config for the specific profile and region
func getConfig(profile, region string) aws.Config {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		l.Errorf("unable to load SDK config, %v", err)
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
		l.Errorf("marshalling instances", err)
	}
	fmt.Println(string(json))
}
