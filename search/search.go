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
	"gopkg.in/ini.v1"
)

var l = logger.NewLog()

// search is an interface to search for AWS resources.
type search interface {
	Search(searchBy string, values []string) search
	GetHeaders() []string
}

// Run is the main function to run the search
func Run(cmd, searchBy string, profile, region, values []string) bool {
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
			printJson(response)
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
