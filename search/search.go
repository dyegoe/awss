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
	"io"
	"os"
	"sync"
	"time"

	"github.com/dyegoe/awss/common"
	searchEBS "github.com/dyegoe/awss/search/ebs"
	searchEC2 "github.com/dyegoe/awss/search/ec2"
	searchENI "github.com/dyegoe/awss/search/eni"
	searchS3 "github.com/dyegoe/awss/search/s3"
	searchS3obj "github.com/dyegoe/awss/search/s3obj"
	searchSubnet "github.com/dyegoe/awss/search/subnet"
	searchVPC "github.com/dyegoe/awss/search/vpc"
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

	// Regex makes name patterns regular expressions instead of globs, in searches that match names client-side.
	Regex bool

	// MaxKeys caps the keys scanned per bucket in the s3obj search. Zero means the search's default.
	MaxKeys int

	// MaxBuckets caps the buckets one profile and region may scan in the s3obj search. Zero means
	// the search's default.
	MaxBuckets int

	// AccountNames maps an account ID to its name, for the Owner column of the searches that have
	// one (vpc, subnet, eni). It is shared by every search of the run and only read. Nil means no
	// names.
	AccountNames map[string]string

	// Concurrency is how many profile and region searches may run at once. Zero means
	// DefaultConcurrency.
	Concurrency int

	// Timeout is how long the run may take. A profile and region still searching at the deadline
	// is reported as timed out. Zero disables it.
	Timeout time.Duration
}

// DefaultConcurrency is how many profile and region searches run at once when Options.Concurrency
// is not set. --profiles all over every region is thousands of searches; starting them all at once
// opens thousands of connections and credential lookups, and every finished result set waits in
// memory for the printer.
const DefaultConcurrency = 32

// constructor builds the results object of one search for a single profile and region.
type constructor func(profile, region string, filters map[string][]string, opts *Options) common.Results

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
		new: func(profile, region string, filters map[string][]string, opts *Options) common.Results {
			return searchEC2.New(profile, region, filters, opts.SortField)
		},
		sortFields:     searchEC2.GetSortFields,
		sortFieldNames: searchEC2.SortFieldNames,
	},
	"eni": {
		new: func(profile, region string, filters map[string][]string, opts *Options) common.Results {
			r := searchENI.New(profile, region, filters, opts.SortField, opts.NoInstanceName)
			r.AccountNames = opts.AccountNames
			return r
		},
		sortFields:     searchENI.GetSortFields,
		sortFieldNames: searchENI.SortFieldNames,
	},
	"ebs": {
		new: func(profile, region string, filters map[string][]string, opts *Options) common.Results {
			return searchEBS.New(profile, region, filters, opts.SortField, opts.NoInstanceName)
		},
		sortFields:     searchEBS.GetSortFields,
		sortFieldNames: searchEBS.SortFieldNames,
	},
	"vpc": {
		new: func(profile, region string, filters map[string][]string, opts *Options) common.Results {
			r := searchVPC.New(profile, region, filters, opts.SortField)
			r.AccountNames = opts.AccountNames
			return r
		},
		sortFields:     searchVPC.GetSortFields,
		sortFieldNames: searchVPC.SortFieldNames,
	},
	"subnet": {
		new: func(profile, region string, filters map[string][]string, opts *Options) common.Results {
			r := searchSubnet.New(profile, region, filters, opts.SortField)
			r.AccountNames = opts.AccountNames
			return r
		},
		sortFields:     searchSubnet.GetSortFields,
		sortFieldNames: searchSubnet.SortFieldNames,
	},
	"s3": {
		new: func(profile, region string, filters map[string][]string, opts *Options) common.Results {
			return searchS3.New(profile, region, filters, opts.SortField, opts.Regex, opts.ShowTags)
		},
		sortFields:     searchS3.GetSortFields,
		sortFieldNames: searchS3.SortFieldNames,
	},
	"s3obj": {
		new: func(profile, region string, filters map[string][]string, opts *Options) common.Results {
			return searchS3obj.New(profile, region, filters, opts.SortField, opts.Regex, opts.MaxKeys, opts.MaxBuckets)
		},
		sortFields:     searchS3obj.GetSortFields,
		sortFieldNames: searchS3obj.SortFieldNames,
	},
}

// Execute executes the search command.
//
// It searches for the given command in the given profiles and regions, in parallel: at most
// opts.Concurrency searches run at once, the others wait for a free slot without calling AWS.
// The filters are used to filter the results and opts holds every other setting.
//
// When opts.Timeout is set, a profile and region still searching or still waiting at the deadline
// is reported as timed out in its own result set; the result sets that finished are printed as
// usual.
func Execute(cmd string, profiles, regions []string, filters map[string][]string, opts *Options) error {
	eng, ok := engines[cmd]
	if !ok {
		return fmt.Errorf("command %s not found", cmd)
	}

	workers := workerCount(opts.Concurrency, len(profiles)*len(regions))

	// The buffer matches the workers, so the finished result sets held in memory scale with the
	// concurrency, not with profiles x regions: a worker waits while the printer is behind.
	resultsChan := make(chan common.Results, workers)

	done := make(chan bool)

	go common.PrintResults(stdout, resultsChan, done, opts.Output, opts.ShowEmpty, opts.ShowTags, opts.TagsKeys)

	ctx, cancel := withTimeout(context.Background(), opts.Timeout)
	defer cancel()

	targets := make(chan target)

	wg := sync.WaitGroup{}
	for range workers {
		wg.Go(func() {
			for t := range targets {
				resultsChan <- searchOne(ctx, eng, t.profile, t.region, filters, opts)
			}
		})
	}

	for _, profile := range profiles {
		for _, region := range regions {
			targets <- target{profile: profile, region: region}
		}
	}
	close(targets)

	wg.Wait()
	close(resultsChan)
	<-done
	close(done)

	return nil
}

// target is one profile and region to search.
type target struct {
	profile, region string
}

// workerCount returns how many searches run at once: concurrency, or DefaultConcurrency when it is
// not set, and never more than the searches to run.
func workerCount(concurrency, searches int) int {
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	return min(concurrency, searches)
}

// stdout is where the results are printed. It is a variable so tests can capture it.
var stdout io.Writer = os.Stdout

// withTimeout returns a context that expires after d, or one that never expires when d is 0.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// searchOne runs the search of one profile and region and returns its result set.
//
// When the search has not returned by the deadline of ctx, it returns a new, empty result set
// holding only a timed-out error instead: the rows of a search cut short may be incomplete.
// When the deadline passed before the search started, it is not started at all.
// A search that ignores ctx is left running; its result set is never read again.
func searchOne(
	ctx context.Context, eng engine, profile, region string, filters map[string][]string, opts *Options,
) common.Results {
	// The deadline passed while this search waited for a free slot: do not start it.
	if ctx.Err() != nil {
		return timedOut(eng, profile, region, filters, opts, fmt.Sprintf(
			"search did not start before the %s timeout; raise --timeout or --concurrency", opts.Timeout))
	}

	r := eng.new(profile, region, filters, opts)

	inTime := make(chan bool, 1)
	go func() {
		r.Search(ctx)
		inTime <- ctx.Err() == nil
	}()

	select {
	case ok := <-inTime:
		if ok {
			return r
		}
	case <-ctx.Done():
		// The search may have returned in time just as the deadline hit.
		select {
		case ok := <-inTime:
			if ok {
				return r
			}
		default:
		}
	}

	return timedOut(eng, profile, region, filters, opts, fmt.Sprintf(
		"search timed out after %s; raise --timeout, or set it to 0 to disable it", opts.Timeout))
}

// timedOut returns a new, empty result set of profile and region holding only msg as its error.
func timedOut(
	eng engine, profile, region string, filters map[string][]string, opts *Options, msg string,
) common.Results {
	r := eng.new(profile, region, filters, opts)
	r.AddError(msg)
	return r
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
