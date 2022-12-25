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
// The searches are done in parallel and the results are printed in the
// specified format.
package search

import (
	"fmt"
	"os"
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
func Execute(cmd string, profiles, regions []string, filters map[string][]string, sortField, output string, showEmpty, showTags bool) error { //nolint:lll
	wg := sync.WaitGroup{}

	numInteractions := len(profiles) * len(regions)

	resultsChan := make(chan common.Results, numInteractions)

	done := make(chan bool)

	go common.PrintResults(os.Stdout, resultsChan, done, output, showEmpty, showTags)

	runOnce := true

	for _, profile := range profiles {
		for _, region := range regions {
			var searchResults common.Results

			// Workaround to avoid to spam Okta with too many requests.
			// It will run once just to pre-authenticate.
			if runOnce {
				if _, err := common.WhoAmI(profile, region); err != nil {
					return err
				}
				runOnce = false
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

// getSortFieldsCMDlist is a map of functions that return the sort fields for the given command.
//
// The key is the command name.
// The value is the function that returns the sort fields.
// We use a map to avoid a switch case and mock the functions in the tests.
var getSortFieldsCMDList = map[string]func(string) (map[string]string, error){
	"ec2": searchEC2.GetSortFields,
}

// CheckSortField checks if the given sort field is valid for the given command.
//
// It returns an error if the sort field is not valid.
func CheckSortField(cmd, f string) error {
	execute, ok := getSortFieldsCMDList[cmd]
	if !ok {
		return fmt.Errorf("command %s not found", cmd)
	}

	if _, err := execute(f); err != nil {
		return err
	}

	return nil
}
