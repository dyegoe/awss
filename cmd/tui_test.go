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

package cmd

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"
	"github.com/dyegoe/awss/tui"
)

// streamCall is what one load of the TUI sent to the search.
type streamCall struct {
	cmd               string
	profiles, regions []string
	filters           map[string][]string
	opts              search.Options
}

// fakeTUI replaces the terminal and the search: the TUI runs one load of resource and records it.
// It returns the config the TUI got and the search calls of that load.
func fakeTUI(t *testing.T, resource string) (*tui.Config, *[]streamCall) {
	t.Helper()
	got := &tui.Config{}
	calls := &[]streamCall{}

	oldRun, oldStream := runTUI, streamSearch
	streamSearch = func(
		_ context.Context, cmd string, profiles, regions []string, filters map[string][]string, opts *search.Options,
		out chan<- common.Results,
	) (search.Summary, error) {
		defer close(out)
		*calls = append(*calls, streamCall{cmd: cmd, profiles: profiles, regions: regions, filters: filters, opts: *opts})
		return search.Summary{}, nil
	}
	runTUI = func(ctx context.Context, cfg tui.Config) error {
		*got = cfg
		out := make(chan common.Results)
		return cfg.Load(ctx, resource, out)
	}
	t.Cleanup(func() { runTUI, streamSearch = oldRun, oldStream })
	return got, calls
}

// TestTuiRunE checks that the tui command starts the TUI on the resource, and that a load
// searches every profile and region with no filter and the default sort of the resource.
func TestTuiRunE(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		load         string
		wantResource string
		wantSort     string
	}{
		{name: "default resource", args: []string{"tui"}, load: "ec2", wantResource: "ec2", wantSort: "name"},
		{name: "given resource", args: []string{"tui", "eni"}, load: "eni", wantResource: "eni", wantSort: "id"},
		{name: "switched resource", args: []string{"tui"}, load: "rds-cluster", wantResource: "ec2", wantSort: "id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLI(t, nil)
			useAwsConfig(t, "[profile p1]\n[profile p2]\n")
			cfg, calls := fakeTUI(t, tt.load)

			args := append(slices.Clone(tt.args), "--profiles", "p1,p2", "--regions", "eu-west-1", "--timeout", "1m")
			if _, err := runCLI(t, args...); err != nil {
				t.Fatalf("Execute(%v) error = %v", args, err)
			}

			if cfg.Resource != tt.wantResource || cfg.Searches != 2 {
				t.Errorf("Config resource, searches = %q, %d, want %q, 2", cfg.Resource, cfg.Searches, tt.wantResource)
			}
			if !reflect.DeepEqual(cfg.Resources, tuiResources()) {
				t.Errorf("Config resources = %v, want %v", cfg.Resources, tuiResources())
			}
			want := []streamCall{{
				cmd: tt.load, profiles: []string{"p1", "p2"}, regions: []string{"eu-west-1"}, filters: map[string][]string{},
				opts: search.Options{SortField: tt.wantSort, Concurrency: search.DefaultConcurrency, Timeout: time.Minute},
			}}
			if !reflect.DeepEqual(*calls, want) {
				t.Errorf("search calls = %+v, want %+v", *calls, want)
			}
		})
	}
}

// TestTuiRunE_unknownResource checks that an unknown resource, such as a stray pattern, fails
// before the TUI starts.
func TestTuiRunE_unknownResource(t *testing.T) {
	resetCLI(t, nil)
	cfg, _ := fakeTUI(t, "ec2")

	_, err := runCLI(t, "tui", "sg")
	if err == nil || !strings.Contains(err.Error(), `invalid argument "sg"`) {
		t.Errorf("Execute(tui sg) error = %v, want invalid argument \"sg\"", err)
	}
	if cfg.Load != nil {
		t.Error("the TUI started for an unknown resource")
	}
}

// TestTuiResources checks that every resource of the TUI is a search command whose default sort
// field is valid.
func TestTuiResources(t *testing.T) {
	for _, r := range tuiResources() {
		if err := search.CheckSortField(r, tuiSorts[r]); err != nil {
			t.Errorf("CheckSortField(%s, %s) = %v, want nil", r, tuiSorts[r], err)
		}
	}
}
