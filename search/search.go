package search

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// search is an interface to search for AWS resources.
type search interface {
	search(searchBy map[string][]string)
	getProfile() string
	getRegion() string
	getHeaders() []string
	getRows() [][]string
	getError() error
}

// Run is the main function to run the search
func Run(profiles, regions []string, output string, verbose bool, cmd string, searchBy map[string][]string) error {
	var wg sync.WaitGroup

	// Create a channel to receive the results
	sChan := make(chan search, len(profiles)*len(regions))
	go printResult(sChan, output, verbose)

	// iterate over profiles
	for _, p := range profiles {
		// iterate over regions for each profile
		for _, r := range regions {
			s := getStruct(cmd, p, r)
			if s == nil {
				return fmt.Errorf("no function found for %s", cmd)
			}

			wg.Add(1)
			go func() {
				s.search(searchBy)
				sChan <- s
				defer wg.Done()
			}()
		}
	}
	wg.Wait()
	close(sChan)
	return nil
}

// getStruct returns the struct for the specific command
func getStruct(cmd, profile, region string) search {
	switch cmd {
	case "ec2":
		return &instances{
			Profile: profile,
			Region:  region,
		}
	default:
		return nil
	}
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

// DefaultSharedConfigFilename returns the default shared config filename
func DefaultSharedConfigFilename() string {
	return config.DefaultSharedConfigFilename()
}
