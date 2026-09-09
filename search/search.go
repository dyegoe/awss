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
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/dyegoe/awss/common"
	searchEBS "github.com/dyegoe/awss/search/ebs"
	searchEC2 "github.com/dyegoe/awss/search/ec2"
	searchENI "github.com/dyegoe/awss/search/eni"
)

// Options carries the settings of a search that are not AWS filters.
type Options struct {
	// SortField is the field used to sort each result set.
	SortField string

	// Output is the output format: table, json or json-pretty.
	Output string

	// ShowEmpty prints result sets that have no rows.
	ShowEmpty bool

	// ShowTags shows the Tags column in table output.
	ShowTags bool

	// TagsKeys, when non-empty, restricts the Tags column of table output to those keys.
	TagsKeys []string

	// NoInstanceName skips the instance name lookup in searches that enrich rows with it.
	NoInstanceName bool
}

// constructor builds the results object of one search for a single profile and region.
type constructor func(profile, region string, filters map[string][]string, opts Options) common.Results

// engine describes a search command: how to build its results and which sort fields it accepts.
type engine struct {
	// new builds the common.Results for one profile and region.
	new constructor

	// sortFields validates a sort field and returns the sort tag to struct field mapping.
	sortFields func(string) (map[string]string, error)

	// sortFieldNames returns the valid sort fields, used to build the --sort help text.
	sortFieldNames func() []string
}

// engines is the registry of search commands, keyed by the cobra command name.
//
// Adding a resource type means adding one entry here.
// It is a variable so tests can replace it.
var engines = map[string]engine{
	"ec2": {
		new: func(profile, region string, filters map[string][]string, opts Options) common.Results {
			return searchEC2.New(profile, region, filters, opts.SortField)
		},
		sortFields:     searchEC2.GetSortFields,
		sortFieldNames: searchEC2.SortFieldNames,
	},
	"eni": {
		new: func(profile, region string, filters map[string][]string, opts Options) common.Results {
			return searchENI.New(profile, region, filters, opts.SortField, opts.NoInstanceName)
		},
		sortFields:     searchENI.GetSortFields,
		sortFieldNames: searchENI.SortFieldNames,
	},
	"ebs": {
		new: func(profile, region string, filters map[string][]string, opts Options) common.Results {
			return searchEBS.New(profile, region, filters, opts.SortField, opts.NoInstanceName)
		},
		sortFields:     searchEBS.GetSortFields,
		sortFieldNames: searchEBS.SortFieldNames,
	},
}

// Execute executes the search command.
//
// It searches for the given command in the given profiles and regions, in parallel.
// The filters are used to filter the results and opts holds every other setting.
func Execute(cmd string, profiles, regions []string, filters map[string][]string, opts Options) error {
	eng, ok := engines[cmd]
	if !ok {
		return fmt.Errorf("command %s not found", cmd)
	}

	ctx := context.Background()
	wg := sync.WaitGroup{}

	numInteractions := len(profiles) * len(regions)

	resultsChan := make(chan common.Results, numInteractions)

	done := make(chan bool)

	go common.PrintResults(os.Stdout, resultsChan, done, opts.Output, opts.ShowEmpty, opts.ShowTags, opts.TagsKeys)

	runOnce := true

	for _, profile := range profiles {
		for _, region := range regions {
			// Workaround to avoid to spam Okta with too many requests.
			// It will run once just to pre-authenticate.
			if runOnce {
				if _, err := common.WhoAmI(profile, region); err != nil {
					return err
				}
				runOnce = false
			}

			searchResults := eng.new(profile, region, filters, opts)

			wg.Add(1)

			go func() {
				defer wg.Done()

				searchResults.Search(ctx)

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
// It returns an error if the command is unknown or the sort field is not valid.
func CheckSortField(cmd, f string) error {
	eng, ok := engines[cmd]
	if !ok {
		return fmt.Errorf("command %s not found", cmd)
	}

	if _, err := eng.sortFields(f); err != nil {
		return err
	}

	return nil
}

// SortFieldNames returns the valid sort fields of the given command, sorted alphabetically.
//
// It returns nil for an unknown command. It is used by the cmd package to build --sort help texts.
func SortFieldNames(cmd string) []string {
	eng, ok := engines[cmd]
	if !ok {
		return nil
	}
	return eng.sortFieldNames()
}
