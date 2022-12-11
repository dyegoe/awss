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

// Package search provides the entry point for the search command.
//
// It implements a search command that searches for resources in AWS.
// The searchs are done in parallel and the results are printed in the
// specified format.
package search

import (
	"fmt"
	"sync"

	"github.com/dyegoe/awss/common"
	searchEC2 "github.com/dyegoe/awss/search/ec2"
	searchENI "github.com/dyegoe/awss/search/eni"
)

// Execute executes the search command.
//
// It searches for the given command in the given profiles and regions.
// The filters are used to filter the results.
// The output is the format of the output.
// The showEmpty flag indicates if empty results should be shown.
func Execute(cmd string, profiles, regions []string, filters map[string][]string, sortField string, output string, showEmpty, showTags bool) error {
	wg := sync.WaitGroup{}

	numInteractions := len(profiles) * len(regions)

	resultsChan := make(chan common.Results, numInteractions)

	done := make(chan bool)

	go common.PrintResults(resultsChan, done, output, showEmpty, showTags)

	runOnce := true

	for _, profile := range profiles {
		for _, region := range regions {
			var searchResults common.Results

			// Workaround to avoid to spam Okta with too many requests.
			// It will run once just to pre-authenticate.
			if runOnce {
				if _, err := common.WhoAmI(profile, region); err != nil {
					return err
				} else {
					runOnce = false
				}
			}

			switch cmd {
			case "ec2":
				searchResults = searchEC2.New(profile, region, filters, sortField)
			case "eni":
				searchResults = searchENI.New(profile, region, filters, sortField)
			default:
				return fmt.Errorf("command %s not found", cmd)
			}

			wg.Add(1)

			go func() {
				defer wg.Done()

				searchResults.Search()

				resultsChan <- searchResults
			}()
		}
	}

	wg.Wait()
	close(resultsChan)
	<-done
	close(done)

	return nil
}

// CheckSortField checks if the given sort field is valid for the given command.
//
// It returns an error if the sort field is not valid.
func CheckSortField(cmd, f string) error {
	switch cmd {
	case "ec2":
		if _, err := searchEC2.GetSortFields(f); err != nil {
			return err
		}
	default:
		return fmt.Errorf("command %s not found for sort field %s", cmd, f)
	}
	return nil
}
