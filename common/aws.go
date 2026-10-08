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
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"gopkg.in/ini.v1"
)

// DefaultRegion is used when no --regions flag, AWS_REGION, or
// AWS_DEFAULT_REGION is set.
const DefaultRegion = "us-east-1"

// ErrNoAZSelected is returned when no availability zone is selected.
// It is used as a sentinel to distinguish "no AZ filter" from invalid AZ input.
var ErrNoAZSelected = fmt.Errorf("no availability zone selected")

// AwsConfig returns a AWS config for the specific profile and region.
func AwsConfig(profile, region string) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return cfg, fmt.Errorf("loading the AWS config of profile %q in %s: %w", profile, region, err)
	}
	return cfg, nil
}

// awsConfigFile returns the AWS config file the AWS SDK reads: AWS_CONFIG_FILE when it is set,
// ~/.aws/config otherwise. The SDK reads only that one file, so awss must list profiles from it too.
//
// It is resolved at each call, not at start-up, so the environment at run time decides.
func awsConfigFile() string {
	if f := os.Getenv("AWS_CONFIG_FILE"); f != "" {
		return f
	}
	return config.DefaultSharedConfigFilename()
}

// GetAwsProfiles returns the profiles of the AWS config file (see awsConfigFile).
//
// Named profiles are stored as `[profile name]` sections, while the default
// profile is stored as a bare `[default]` section, so both forms are matched.
func GetAwsProfiles() ([]string, error) {
	cfg, err := ini.Load(awsConfigFile())
	if err != nil {
		return nil, fmt.Errorf("reading the AWS config file: %w", err)
	}
	profiles := []string{}
	for _, section := range cfg.Sections() {
		switch {
		case section.Name() == "default":
			profiles = append(profiles, "default")
		case strings.HasPrefix(section.Name(), "profile "):
			profiles = append(profiles, strings.TrimPrefix(section.Name(), "profile "))
		}
	}
	return profiles, nil
}

// TagName returns the value of the tag:Name from a slice of types.Tag.
//
// A tag with a nil key is skipped, and a nil value reads as "".
func TagName(tags []types.Tag) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "Name" {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}

// TagsToMap takes a slice of types.Tag and returns a map of tags and values.
//
// A tag with a nil key is skipped, as in S3TagsToMap, and a nil value reads as "".
func TagsToMap(tags []types.Tag) map[string]string {
	data := map[string]string{}
	for _, t := range tags {
		if t.Key == nil {
			continue
		}
		data[*t.Key] = aws.ToString(t.Value)
	}
	return data
}

// S3TagsToMap takes a slice of S3 tags and returns a map of tags and values.
//
// S3 uses its own Tag type, so it cannot share TagsToMap. Tags with a nil key are skipped.
func S3TagsToMap(tags []s3types.Tag) map[string]string {
	data := map[string]string{}
	for _, t := range tags {
		if t.Key == nil {
			continue
		}
		data[*t.Key] = aws.ToString(t.Value)
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
			Name:   aws.String("tag:Name"),
			Values: names,
		},
	}
}

// FilterTags returns a list of types.Filter by tag:Key=Value1,Value2,Value3...
// It returns an error if any tag string is malformed.
func FilterTags(tags []string) ([]types.Filter, error) {
	if len(tags) == 0 {
		return []types.Filter{}, nil
	}
	parsed, err := ParseTags(tags)
	if err != nil {
		return nil, fmt.Errorf("parsing tag filters: %w", err)
	}
	filters := []types.Filter{}
	for key, values := range parsed {
		filters = append(filters, types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: values,
		})
	}
	return filters, nil
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
			Name:   aws.String("availability-zone"),
			Values: azs,
		},
	}
}

// getAwsProfilesFn wraps GetAwsProfiles so it can be mocked in tests.
var getAwsProfilesFn = GetAwsProfiles

// CheckProfiles checks if the profiles are valid.
//
// If the user passes no profile, it returns a single empty-string profile so
// the AWS SDK resolves credentials itself, in order: AWS_PROFILE, static
// AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY env vars, or the `default` profile.
// This is intentionally not validated against the AWS config file, since that file
// may not exist when credentials come purely from the environment.
// If the user passes the `all` profile, it returns allProfiles (the `all-profiles`
// list of the awss config) when it is not empty, or else every profile in the AWS config file.
// If the user passes a list of profiles, it will check if they are valid and return them.
// Every returned profile, including the allProfiles list, must exist in the AWS config file
// (AWS_CONFIG_FILE, or ~/.aws/config when it is not set).
func CheckProfiles(profiles, allProfiles []string) ([]string, error) {
	if len(profiles) == 0 {
		return []string{""}, nil
	}

	awsProfiles, err := getAwsProfilesFn()
	if err != nil {
		return nil, err
	}

	if len(profiles) == 1 && profiles[0] == "all" {
		if len(allProfiles) == 0 {
			return awsProfiles, nil
		}
		if err := profilesExist(allProfiles, awsProfiles); err != nil {
			return nil, fmt.Errorf("checking all-profiles: %w", err)
		}
		return allProfiles, nil
	}

	if err := profilesExist(profiles, awsProfiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

// profilesExist returns an error naming the first profile that is not in awsProfiles.
func profilesExist(profiles, awsProfiles []string) error {
	for _, profile := range profiles {
		if !StringInSlice(profile, awsProfiles) {
			return fmt.Errorf("profile %s not found", profile)
		}
	}
	return nil
}

// getAwsRegionEnvFn returns the region from the AWS_REGION or AWS_DEFAULT_REGION
// environment variables, in that order of precedence, matching the AWS SDK's
// own resolution order. It returns an empty string when neither is set.
//
// We use a variable to mock it in the tests.
var getAwsRegionEnvFn = func() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	return os.Getenv("AWS_DEFAULT_REGION")
}

// CheckRegions checks if the regions are valid.
//
// If the user passes no region, it falls back to AWS_REGION/AWS_DEFAULT_REGION,
// or DefaultRegion, and that single region is not validated against allRegions.
// If the user passes the `all` region, it will return all the regions.
// If the user passes a list of regions, it will check if they are valid and return them.
// It compares the regions passwd by the user with the all-regions list in the config file.
func CheckRegions(regions, allRegions []string) ([]string, error) {
	if len(regions) == 0 {
		if region := getAwsRegionEnvFn(); region != "" {
			return []string{region}, nil
		}
		return []string{DefaultRegion}, nil
	}

	if len(regions) == 1 && regions[0] == "all" {
		return allRegions, nil
	}

	for _, region := range regions {
		if !StringInSlice(region, allRegions) {
			return nil, fmt.Errorf("region %s not found", region)
		}
	}
	return regions, nil
}

// CheckAvailabilityZones checks if the availability zones are valid.
//
// Availability zones must be only one letter that will be appended to the region name.
// Example: a,b,c,d,e,f for us-east-1
func CheckAvailabilityZones(az []string) error {
	if len(az) == 0 {
		return ErrNoAZSelected
	}

	for _, zone := range az {
		validAvailabilityZones := []string{"a", "b", "c", "d", "e", "f"}
		if len(zone) != 1 {
			return fmt.Errorf("availability zones must be just a letter. It will be append to the region: %s", zone)
		}
		if !StringInSlice(zone, validAvailabilityZones) {
			return fmt.Errorf("availability zone %s not found. Valid options are: %s",
				zone, StringSliceToString(validAvailabilityZones, ", "))
		}
	}
	return nil
}

// FilterDefault returns a list of types.Filter. The key is used as filter Name and the values as Values.
func FilterDefault(key string, values []string) []types.Filter {
	if key == "" || len(values) == 0 {
		return []types.Filter{}
	}
	return []types.Filter{
		{
			Name:   aws.String(key),
			Values: values,
		},
	}
}
