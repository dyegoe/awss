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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"
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
	err := Execute("nope", []string{"default"}, []string{"us-east-1"}, map[string][]string{}, opts)
	if err == nil {
		t.Fatal("Execute() error = nil, want command not found")
	}
	if called {
		t.Error("Execute() built results for an unknown command")
	}
}

// TestEngines_registeredCommands checks every built-in command has both a constructor and sort fields.
func TestEngines_registeredCommands(t *testing.T) {
	for _, cmd := range []string{"ec2", "eni", "ebs", "vpc", "subnet", "s3", "s3obj"} {
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
			if err := Execute("test", tt.profiles, tt.regions, map[string][]string{}, opts); err != nil {
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
			if err := Execute("test", []string{"p1", "p2"}, []string{"r1", "r2"}, map[string][]string{}, opts); err != nil {
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

// TestExecute_timeout checks that the finished result sets are printed as they are, and that a
// search still running at the deadline is printed as timed out, without rows, whether it honors
// the context or ignores it.
func TestExecute_timeout(t *testing.T) {
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })
	ctxErr := make(chan error, 1)

	mockTimedEngine(t, map[string]func(context.Context, *timedResults){
		"fast": func(_ context.Context, r *timedResults) { r.Data = append(r.Data, "row-1") },
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
	profiles := []string{"fast", "failed", "cut-short", "hung"}
	if err := Execute("test", profiles, []string{"us-east-1"}, map[string][]string{}, opts); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if err := <-ctxErr; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("search context error = %v, want %v", err, context.DeadlineExceeded)
	}
	timedOut := []string{"search timed out after 100ms; raise --timeout, or set it to 0 to disable it"}
	want := map[string]printedSet{
		"fast": {Profile: "fast", Region: "us-east-1", Data: []string{"row-1"}},
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
	if err := Execute("test", profiles, []string{"us-east-1"}, map[string][]string{}, opts); err != nil {
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
