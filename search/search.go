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
package search

import (
	"fmt"
	"sync"

	"github.com/dyegoe/awss/common"
	searchEC2 "github.com/dyegoe/awss/search/ec2"
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

	for _, profile := range profiles {
		for _, region := range regions {
			var searchResults common.Results

			switch cmd {
			case "ec2":
				searchResults = searchEC2.New(profile, region, filters, sortField)
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

func CheckSortField(cmd, f string) error {
	switch cmd {
	case "ec2":
		if _, err := searchEC2.GetSortFields(f); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("command %s not found", cmd)
}
