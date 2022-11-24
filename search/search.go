package search

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// search is an interface to search for AWS resources.
type search interface {
	search(searchBy map[string][]string, sortBy string)
	getProfile() string
	getRegion() string
	getHeaders() []string
	getRows() [][]string
	getError() string
}

// Run is the main function to run the search
func Run(profiles, regions []string, output string, verbose bool, cmd string, searchBy map[string][]string, sortBy string) error {
	var wg sync.WaitGroup

	// Create a channel to receive the results
	sChan := make(chan search, len(profiles)*len(regions))
	// Create a channel to signal when the printing is done
	done := make(chan bool)
	// Launch the printer
	go printResult(sChan, output, verbose, done)

	// Iterate over profiles
	for _, p := range profiles {
		// Iterate over regions for each profile
		for _, r := range regions {
			s := getStruct(cmd, p, r)
			if s == nil {
				return fmt.Errorf("no function found for %s", cmd)
			}

			wg.Add(1)
			go func() {
				s.search(searchBy, sortBy)
				sChan <- s
				defer wg.Done()
			}()
		}
	}
	// Wait for all searches to finish
	wg.Wait()
	// Close the channel to signal that no more results will be sent
	close(sChan)
	// Wait for the printResult goroutine to finish
	<-done
	close(done)

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

// getStructFieldByTag lookups for a field by json tag in a struct
func getStructFieldByTag(tag, value string, s interface{}) (string, error) {
	r := reflect.ValueOf(s)
	for i := 0; i < r.NumField(); i++ {
		field := r.Type().Field(i)
		if field.Tag.Get(tag) == value {
			return field.Name, nil
		}
	}
	return "", fmt.Errorf("field not found")
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

// getTags returns the tags
func getTags(tags []types.Tag) map[string]string {
	data := map[string]string{}
	for _, t := range tags {
		data[*t.Key] = *t.Value
	}
	return data
}

// getValue returns the string value if not nil
func getValue(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// mapToString converts a map[string]string to a string
func mapToString(m map[string]string, kvSep, listSep string) string {
	var tags []string
	for k, v := range m {
		tags = append(tags, fmt.Sprintf("%s%s%s", k, kvSep, v))
	}
	return strings.Join(tags, listSep)
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
