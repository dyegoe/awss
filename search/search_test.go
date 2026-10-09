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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"
	searchENI "github.com/dyegoe/awss/search/eni"
	searchSubnet "github.com/dyegoe/awss/search/subnet"
	searchVPC "github.com/dyegoe/awss/search/vpc"
)

// mockEngines swaps the registry for a single "test" engine and restores it after the test.
func mockEngines(t *testing.T, newFn constructor) {
	t.Helper()
	old := engines
	t.Cleanup(func() { engines = old })
	engines = map[string]engine{
		"test": {
			new: newFn,
			sortFields: func(f string) (map[string]string, error) {
				fields := map[string]string{"field1": "value1"}
				if _, ok := fields[f]; !ok {
					return nil, fmt.Errorf("field %s not found", f)
				}
				return fields, nil
			},
			sortFieldNames: func() []string { return []string{"field1"} },
		},
	}
}

// TestCheckSortField tests the CheckSortField function.
func TestCheckSortField(t *testing.T) {
	mockEngines(t, nil)

	tests := []struct {
		name    string
		cmd     string
		f       string
		wantErr bool
	}{
		{
			name:    "Command found and field found",
			cmd:     "test",
			f:       "field1",
			wantErr: false,
		},
		{
			name:    "Command not found",
			cmd:     "test2",
			f:       "field1",
			wantErr: true,
		},
		{
			name:    "Command found but field not found",
			cmd:     "test",
			f:       "field2",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckSortField(tt.cmd, tt.f); (err != nil) != tt.wantErr {
				t.Errorf("CheckSortField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestExecute_unknownCommand checks that an unknown command fails before any profile or region is touched.
func TestExecute_unknownCommand(t *testing.T) {
	called := false
	mockEngines(t, func(_, _ string, _ map[string][]string, _ *Options) common.Results {
		called = true
		return nil
	})

	opts := &Options{Output: common.JSON}
	_, err := Execute("nope", []string{"default"}, []string{"us-east-1"}, map[string][]string{}, opts)
	if err == nil {
		t.Fatal("Execute() error = nil, want command not found")
	}
	if called {
		t.Error("Execute() built results for an unknown command")
	}
}

// TestEngines_searchKeepsFilters checks that no search changes the filters map it is given: one
// map is shared by every profile x region goroutine of a run (AGENTS.md). The profile does not
// exist, so each search builds its filters and stops at the AWS config, with no AWS call.
func TestEngines_searchKeepsFilters(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	// One key of every kind the commands send: IDs, names, tags, zones, CIDRs and S3 patterns.
	sent := func() map[string][]string {
		return map[string][]string{
			"instance-id": {"i-1"}, "tag:Name": {"web-*"}, "tag": {"Env=prod:dev"},
			"availability-zone": {"a", "b"}, "cidr": {"10.0.0.0/16"}, "vpc-id": {"vpc-1"},
			"name": {"logs-*"}, "bucket": {"b1"}, "key": {"app/*"},
			"engine": {"postgres"}, "engine-version": {"13*"}, "db-cluster-id": {"c1"}, "db-instance-id": {"db-1"},
		}
	}
	for cmd, eng := range engines {
		t.Run(cmd, func(t *testing.T) {
			filters := sent()
			r := eng.new("awss-test-missing-profile", "us-east-1", filters, &Options{NoInstanceName: true})

			r.Search(context.Background())

			if !reflect.DeepEqual(filters, sent()) {
				t.Errorf("%s Search() changed its filters to %v, want %v", cmd, filters, sent())
			}
		})
	}
}

// TestEngines_registeredCommands checks every built-in command has both a constructor and sort fields.
func TestEngines_registeredCommands(t *testing.T) {
	for _, cmd := range []string{"ec2", "eni", "ebs", "vpc", "subnet", "s3", "s3obj", "org", "rds"} {
		eng, ok := engines[cmd]
		if !ok {
			t.Errorf("engines[%q] missing", cmd)
			continue
		}
		if eng.new == nil || eng.sortFields == nil || eng.sortFieldNames == nil {
			t.Errorf("engines[%q] must define new, sortFields and sortFieldNames", cmd)
		}
		for _, name := range eng.sortFieldNames() {
			if _, err := eng.sortFields(name); err != nil {
				t.Errorf("engines[%q]: sortFieldNames lists %q but sortFields rejects it: %v", cmd, name, err)
			}
		}
		r := eng.new("default", "us-east-1", map[string][]string{}, &Options{SortField: "id", NoInstanceName: true})
		if r == nil {
			t.Errorf("engines[%q].new returned nil", cmd)
			continue
		}
		if got := r.GetSortField(); got != "id" {
			t.Errorf("engines[%q].new did not pass SortField through, got %q", cmd, got)
		}
	}
}

// TestSortFieldNames tests SortFieldNames for known and unknown commands.
func TestSortFieldNames(t *testing.T) {
	mockEngines(t, nil)
	if got := SortFieldNames("test"); len(got) != 1 || got[0] != "field1" {
		t.Errorf("SortFieldNames(test) = %v, want [field1]", got)
	}
	if got := SortFieldNames("nope"); got != nil {
		t.Errorf("SortFieldNames(nope) = %v, want nil", got)
	}
}

// fakeResults is an empty, error-free result set, so Execute prints nothing for it.
type fakeResults struct{ common.BaseResults }

func (f *fakeResults) Search(_ context.Context)  {}
func (f *fakeResults) Len() int                  { return 0 }
func (f *fakeResults) GetHeaders() []interface{} { return nil }
func (f *fakeResults) GetRows() []interface{}    { return nil }

// mockCountingEngine registers a "test" engine returning fakeResults and counts how many it built.
func mockCountingEngine(t *testing.T) *int {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	mockEngines(t, func(profile, region string, _ map[string][]string, _ *Options) common.Results {
		mu.Lock()
		calls++
		mu.Unlock()
		return &fakeResults{common.BaseResults{Profile: profile, Region: region}}
	})
	return &calls
}

// TestExecute_fanOut checks that every profile x region is searched once, and none without profiles.
func TestExecute_fanOut(t *testing.T) {
	tests := []struct {
		name      string
		profiles  []string
		regions   []string
		wantCalls int
	}{
		{name: "every profile x region", profiles: []string{"p1", "p2", "p3"}, regions: []string{"r1", "r2"}, wantCalls: 6},
		{name: "one profile and region", profiles: []string{"p1"}, regions: []string{"r1"}, wantCalls: 1},
		{name: "no profiles, no searches", profiles: []string{}, regions: []string{"r1"}, wantCalls: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := mockCountingEngine(t)

			opts := &Options{Output: common.JSON}
			if _, err := Execute("test", tt.profiles, tt.regions, map[string][]string{}, opts); err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}
			if *calls != tt.wantCalls {
				t.Errorf("searches built = %d, want %d", *calls, tt.wantCalls)
			}
		})
	}
}

// timedResults is a result set whose search is given per profile, so one run can mix searches
// that finish, honor the deadline or hang.
type timedResults struct {
	common.BaseResults
	Data   []string                                   `json:"data"`
	search func(ctx context.Context, r *timedResults) `json:"-"`
}

func (f *timedResults) Search(ctx context.Context) { f.search(ctx, f) }
func (f *timedResults) Len() int                   { return len(f.Data) }
func (f *timedResults) GetHeaders() []interface{}  { return nil }
func (f *timedResults) GetRows() []interface{}     { return nil }

// mockTimedEngine registers a "test" engine whose searches are looked up by profile.
func mockTimedEngine(t *testing.T, searches map[string]func(ctx context.Context, r *timedResults)) {
	t.Helper()
	mockEngines(t, func(profile, region string, _ map[string][]string, _ *Options) common.Results {
		return &timedResults{
			BaseResults: common.BaseResults{Profile: profile, Region: region},
			Data:        []string{},
			search:      searches[profile],
		}
	})
}

// captureStdout sends the printed results to a buffer for the duration of the test.
func captureStdout(t *testing.T) *bytes.Buffer {
	t.Helper()
	old := stdout
	t.Cleanup(func() { stdout = old })
	buf := &bytes.Buffer{}
	stdout = buf
	return buf
}

// printedSet is one result set as printed in JSON.
type printedSet struct {
	Profile string   `json:"profile"`
	Region  string   `json:"region"`
	Errors  []string `json:"errors"`
	Data    []string `json:"data"`
}

// decodePrinted parses the JSON result sets printed by Execute, keyed by profile.
func decodePrinted(t *testing.T, out *bytes.Buffer) map[string]printedSet {
	t.Helper()
	got := map[string]printedSet{}
	dec := json.NewDecoder(out)
	for dec.More() {
		var set printedSet
		if err := dec.Decode(&set); err != nil {
			t.Fatalf("decoding output %q: %v", out.String(), err)
		}
		got[set.Profile] = set
	}
	return got
}

// TestExecute_timeoutReachesSearch checks that every search gets a context that expires at the
// timeout, and that a timeout of 0 sets no deadline.
func TestExecute_timeoutReachesSearch(t *testing.T) {
	tests := []struct {
		name         string
		timeout      time.Duration
		wantDeadline bool
	}{
		{name: "timeout sets a deadline", timeout: time.Hour, wantDeadline: true},
		{name: "zero disables the deadline", timeout: 0, wantDeadline: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			deadlines := map[string]bool{}
			record := func(ctx context.Context, r *timedResults) {
				_, ok := ctx.Deadline()
				mu.Lock()
				deadlines[r.Profile+"/"+r.Region] = ok
				mu.Unlock()
			}
			mockTimedEngine(t, map[string]func(context.Context, *timedResults){"p1": record, "p2": record})
			captureStdout(t)

			opts := &Options{Output: common.JSON, Timeout: tt.timeout}
			if _, err := Execute("test", []string{"p1", "p2"}, []string{"r1", "r2"}, map[string][]string{}, opts); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(deadlines) != 4 {
				t.Fatalf("searches run = %d, want 4", len(deadlines))
			}
			for key, ok := range deadlines {
				if ok != tt.wantDeadline {
					t.Errorf("%s: context has deadline = %v, want %v", key, ok, tt.wantDeadline)
				}
			}
		})
	}
}

// TestExecute_timeout checks that the finished result sets are printed as they are, that a
// search still running at the deadline is printed as timed out, without rows, whether it honors
// the context or ignores it, and that the summary counts each result set once.
func TestExecute_timeout(t *testing.T) {
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })
	ctxErr := make(chan error, 1)

	mockTimedEngine(t, map[string]func(context.Context, *timedResults){
		"fast":  func(_ context.Context, r *timedResults) { r.Data = append(r.Data, "row-1", "row-2") },
		"empty": func(_ context.Context, _ *timedResults) {},
		"failed": func(_ context.Context, r *timedResults) {
			r.Errors = append(r.Errors, "error describing instances: AccessDenied")
		},
		// Honors the context: returns at the deadline with a partial page and the SDK error.
		"cut-short": func(ctx context.Context, r *timedResults) {
			r.Data = append(r.Data, "partial-row")
			<-ctx.Done()
			ctxErr <- ctx.Err()
			r.Errors = append(r.Errors, "error describing instances: "+ctx.Err().Error())
		},
		// Ignores the context and never returns during the run.
		"hung": func(_ context.Context, r *timedResults) {
			<-hang
			r.Data = append(r.Data, "too-late")
		},
	})
	out := captureStdout(t)

	opts := &Options{Output: common.JSON, Timeout: 100 * time.Millisecond}
	profiles := []string{"fast", "empty", "failed", "cut-short", "hung"}
	summary, err := Execute("test", profiles, []string{"us-east-1"}, map[string][]string{}, opts)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	wantSummary := Summary{
		Profiles: 5, Regions: 1, Searches: 5, Concurrency: 5,
		WithResults: 1, Empty: 1, Errors: 1, TimedOut: 2, Resources: 2,
	}
	if summary != wantSummary {
		t.Errorf("Execute() summary = %+v, want %+v", summary, wantSummary)
	}
	if got := summary.Failed(); got != 3 {
		t.Errorf("Summary.Failed() = %d, want 3", got)
	}

	if err := <-ctxErr; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("search context error = %v, want %v", err, context.DeadlineExceeded)
	}
	timedOut := []string{"search timed out after 100ms; raise --timeout, or set it to 0 to disable it"}
	want := map[string]printedSet{
		// "empty" is not printed: the run does not set ShowEmpty.
		"fast": {Profile: "fast", Region: "us-east-1", Data: []string{"row-1", "row-2"}},
		"failed": {
			Profile: "failed", Region: "us-east-1", Data: []string{},
			Errors: []string{"error describing instances: AccessDenied"},
		},
		"cut-short": {Profile: "cut-short", Region: "us-east-1", Data: []string{}, Errors: timedOut},
		"hung":      {Profile: "hung", Region: "us-east-1", Data: []string{}, Errors: timedOut},
	}
	if got := decodePrinted(t, out); !reflect.DeepEqual(got, want) {
		t.Errorf("printed result sets\n%+v\nwant\n%+v", got, want)
	}
}

// TestExecute_loggedOutProfile checks that a profile without a valid session reports its own error
// in its result set, and that the other profiles are unaffected.
func TestExecute_loggedOutProfile(t *testing.T) {
	const expired = "error describing instances: failed to refresh cached credentials, the SSO session has expired"
	mockTimedEngine(t, map[string]func(context.Context, *timedResults){
		"logged-in-1": func(_ context.Context, r *timedResults) { r.Data = append(r.Data, "row-1") },
		"expired":     func(_ context.Context, r *timedResults) { r.Errors = append(r.Errors, expired) },
		"logged-in-2": func(_ context.Context, r *timedResults) { r.Data = append(r.Data, "row-2") },
	})
	out := captureStdout(t)

	opts := &Options{Output: common.JSON}
	profiles := []string{"logged-in-1", "expired", "logged-in-2"}
	if _, err := Execute("test", profiles, []string{"us-east-1"}, map[string][]string{}, opts); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	want := map[string]printedSet{
		"logged-in-1": {Profile: "logged-in-1", Region: "us-east-1", Data: []string{"row-1"}},
		"expired":     {Profile: "expired", Region: "us-east-1", Data: []string{}, Errors: []string{expired}},
		"logged-in-2": {Profile: "logged-in-2", Region: "us-east-1", Data: []string{"row-2"}},
	}
	if got := decodePrinted(t, out); !reflect.DeepEqual(got, want) {
		t.Errorf("printed result sets\n%+v\nwant\n%+v", got, want)
	}
}

// concurrencyProbe counts the searches running at once. Each search waits until the expected
// number is running and then 50ms more, time for a pool without a limit to start the others, so
// a pool that never fills, or one that runs more, shows in max.
type concurrencyProbe struct {
	mu       sync.Mutex
	want     int
	active   int
	max      int
	finished int
	full     chan struct{}
	filled   bool
}

// search is the Search body of every result set: it blocks until want searches have run at once,
// plus the grace time.
func (p *concurrencyProbe) search(_ context.Context, _ *timedResults) {
	p.mu.Lock()
	p.active++
	p.max = max(p.max, p.active)
	// Later searches reach want again as the first ones finish; close only once.
	if p.active == p.want && !p.filled {
		p.filled = true
		time.AfterFunc(50*time.Millisecond, func() { close(p.full) })
	}
	p.mu.Unlock()

	select {
	case <-p.full:
	case <-time.After(2 * time.Second): // the pool never filled; max reports it
	}

	p.mu.Lock()
	p.active--
	p.finished++
	p.mu.Unlock()
}

// TestExecute_concurrency checks that at most opts.Concurrency searches run at once, that the
// default applies when it is not set, that it never exceeds the searches to run, and that every
// profile x region still completes.
func TestExecute_concurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		profiles    int
		regions     int
		wantMax     int
	}{
		{name: "one at a time", concurrency: 1, profiles: 3, regions: 2, wantMax: 1},
		{name: "a limit below the searches", concurrency: 3, profiles: 4, regions: 3, wantMax: 3},
		{name: "zero uses the default", concurrency: 0, profiles: 20, regions: 3, wantMax: DefaultConcurrency},
		{name: "a limit above the searches", concurrency: 50, profiles: 5, regions: 1, wantMax: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := &concurrencyProbe{want: tt.wantMax, full: make(chan struct{})}
			mockEngines(t, func(profile, region string, _ map[string][]string, _ *Options) common.Results {
				return &timedResults{
					BaseResults: common.BaseResults{Profile: profile, Region: region},
					Data:        []string{},
					search:      probe.search,
				}
			})
			captureStdout(t)

			profiles := make([]string, tt.profiles)
			for i := range profiles {
				profiles[i] = fmt.Sprintf("p%d", i)
			}
			regions := make([]string, tt.regions)
			for i := range regions {
				regions[i] = fmt.Sprintf("r%d", i)
			}

			opts := &Options{Output: common.JSON, Concurrency: tt.concurrency}
			if _, err := Execute("test", profiles, regions, map[string][]string{}, opts); err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}
			if probe.max != tt.wantMax {
				t.Errorf("searches running at once = %d, want %d", probe.max, tt.wantMax)
			}
			if want := tt.profiles * tt.regions; probe.finished != want {
				t.Errorf("searches finished = %d, want %d", probe.finished, want)
			}
		})
	}
}

// TestExecute_timeoutWhileWaiting checks that a search still waiting for a free slot at the
// deadline is reported as not started, never calls its Search, and counts as timed out.
func TestExecute_timeoutWhileWaiting(t *testing.T) {
	started := make(chan string, 2)
	mockTimedEngine(t, map[string]func(context.Context, *timedResults){
		// Holds the only slot until the deadline.
		"first": func(ctx context.Context, r *timedResults) {
			started <- r.Profile
			<-ctx.Done()
		},
		"second": func(_ context.Context, r *timedResults) { started <- r.Profile },
	})
	out := captureStdout(t)

	opts := &Options{Output: common.JSON, Concurrency: 1, Timeout: 50 * time.Millisecond}
	summary, err := Execute("test", []string{"first", "second"}, []string{"us-east-1"}, map[string][]string{}, opts)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	// A search that did not start counts as timed out, like one cut short.
	if summary.TimedOut != 2 || summary.Errors != 0 {
		t.Errorf("Execute() summary = %+v, want TimedOut 2 and Errors 0", summary)
	}
	// Drained, not closed: a search returning at the deadline may still be sending.
	var got []string
	for len(started) > 0 {
		got = append(got, <-started)
	}
	if !reflect.DeepEqual(got, []string{"first"}) {
		t.Errorf("searches started = %v, want [first]", got)
	}
	want := map[string]printedSet{
		"first": {
			Profile: "first", Region: "us-east-1", Data: []string{},
			Errors: []string{"search timed out after 50ms; raise --timeout, or set it to 0 to disable it"},
		},
		"second": {
			Profile: "second", Region: "us-east-1", Data: []string{},
			Errors: []string{"search did not start before the 50ms timeout; raise --timeout or --concurrency"},
		},
	}
	if got := decodePrinted(t, out); !reflect.DeepEqual(got, want) {
		t.Errorf("printed result sets\n%+v\nwant\n%+v", got, want)
	}
}

// TestWorkerCount checks the number of workers for a concurrency and a number of searches.
func TestWorkerCount(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		searches    int
		want        int
	}{
		{name: "the limit", concurrency: 8, searches: 100, want: 8},
		{name: "fewer searches than the limit", concurrency: 8, searches: 3, want: 3},
		{name: "zero uses the default", concurrency: 0, searches: 100, want: DefaultConcurrency},
		{name: "negative uses the default", concurrency: -1, searches: 100, want: DefaultConcurrency},
		{name: "no searches", concurrency: 8, searches: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workerCount(tt.concurrency, tt.searches); got != tt.want {
				t.Errorf("workerCount(%d, %d) = %d, want %d", tt.concurrency, tt.searches, got, tt.want)
			}
		})
	}
}

// TestEngines_accountNames checks that the searches with an Owner column receive the account
// names of the run.
func TestEngines_accountNames(t *testing.T) {
	names := map[string]string{"111111111111": "network"}
	opts := &Options{AccountNames: names}
	tests := []struct {
		name string
		get  func(common.Results) map[string]string
	}{
		{name: "vpc", get: func(r common.Results) map[string]string { return r.(*searchVPC.Results).AccountNames }},
		{name: "subnet", get: func(r common.Results) map[string]string { return r.(*searchSubnet.Results).AccountNames }},
		{name: "eni", get: func(r common.Results) map[string]string { return r.(*searchENI.Results).AccountNames }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := engines[tt.name].new("default", "us-east-1", map[string][]string{}, opts)
			if got := tt.get(r); !reflect.DeepEqual(got, names) {
				t.Errorf("engines[%q].new AccountNames = %v, want %v", tt.name, got, names)
			}
		})
	}
}
