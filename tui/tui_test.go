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

package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/dyegoe/awss/common"
)

// info is a struct field of fakeRow, as eniInfo is in the eni rows.
type info struct {
	Kind  string `header:"Kind"`
	Empty string `header:"Empty"`
	Skip  string
}

// fakeRow is a result row with every kind of field the real rows have.
type fakeRow struct {
	ID    string            `json:"id" header:"ID"`
	Name  string            `json:"name" header:"Name"`
	IPs   []string          `json:"ips" header:"IPs"`
	Info  info              `json:"info" header:"Info"`
	Size  int32             `json:"size" header:"Size"`
	Tags  map[string]string `json:"tags" header:"Tags"`
	Inner string            `json:"inner"`
}

// fakeResults is a result set of fixed rows.
type fakeResults struct {
	common.BaseResults
	Data []fakeRow
}

// Search does nothing: the rows are fixed.
func (r *fakeResults) Search(context.Context) {}

// Len returns the number of rows.
func (r *fakeResults) Len() int { return len(r.Data) }

// GetHeaders returns the headers of fakeRow.
func (r *fakeResults) GetHeaders() []interface{} { return common.Headers(fakeRow{}) }

// GetRows returns the rows.
func (r *fakeResults) GetRows() []interface{} { return common.Rows(r.Data) }

// fixture returns the result sets of each fake resource: two profiles of ec2 and one failed
// region, and one row of vpc.
func fixture() map[string][]common.Results {
	return map[string][]common.Results{
		"ec2": {
			&fakeResults{
				BaseResults: common.BaseResults{Profile: "prod", Region: "eu-west-1"},
				Data: []fakeRow{
					{ID: "i-1", Name: "web-1", IPs: []string{"10.0.0.1", "10.0.0.2"}, Tags: map[string]string{"Env": "prod"}},
					{ID: "i-2", Name: "db-1", Info: info{Kind: "primary"}, Size: 8},
				},
			},
			&fakeResults{
				BaseResults: common.BaseResults{Profile: "dev", Region: "us-east-1"},
				Data:        []fakeRow{{ID: "i-3", Name: "web-dev"}},
			},
			&fakeResults{
				BaseResults: common.BaseResults{Profile: "dev", Region: "eu-west-1", Errors: []string{"access denied"}},
			},
		},
		"vpc": {
			&fakeResults{
				BaseResults: common.BaseResults{Profile: "prod", Region: "eu-west-1"},
				Data:        []fakeRow{{ID: "vpc-1", Name: "main"}},
			},
		},
	}
}

// fakeLoader sends the fixture result sets of a resource, or fails for an unknown one.
func fakeLoader(_ context.Context, resource string, out chan<- common.Results) error {
	defer close(out)
	sets, ok := fixture()[resource]
	if !ok {
		return errors.New("no such resource")
	}
	for _, r := range sets {
		out <- r
	}
	return nil
}

// newModel returns a model of the fake resources, sized like a small terminal.
func newModel(t *testing.T) *Model {
	t.Helper()
	m := New(context.Background(), Config{
		Resource: "ec2", Resources: []string{"ec2", "vpc"}, Searches: 3, Load: fakeLoader,
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	return m
}

// run executes cmd as the Bubble Tea runtime does, feeding every message to m until no command
// is left. It is only used for load commands: they end when the load has finished.
func run(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	msgs := make(chan tea.Msg)
	pending := 0
	exec := func(c tea.Cmd) {
		if c == nil {
			return
		}
		pending++
		go func() { msgs <- c() }()
	}

	exec(cmd)
	for pending > 0 {
		select {
		case msg := <-msgs:
			pending--
			for _, c := range handle(m, msg) {
				exec(c)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("run: the load did not finish")
		}
	}
}

// handle runs msg as the runtime does: a batch is its commands, any other message goes to
// Update. It returns the commands to execute next.
func handle(m *Model, msg tea.Msg) []tea.Cmd {
	if batch, ok := msg.(tea.BatchMsg); ok {
		return batch
	}
	_, next := m.Update(msg)
	return []tea.Cmd{next}
}

// loaded returns a model with the ec2 fixture loaded.
func loaded(t *testing.T) *Model {
	t.Helper()
	m := newModel(t)
	run(t, m, m.Init())
	return m
}

// press sends the keys of s to m one by one, and returns the command of the last key.
func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		_, cmd = m.Update(keyMsg(k))
	}
	return cmd
}

// keyMsg returns the key press of a key name such as "d", "enter" or "ctrl+c".
func keyMsg(k string) tea.KeyPressMsg {
	switch k {
	case keyEnter:
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case keyEsc:
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case keyCtrlC:
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		r, _ := utf8.DecodeRuneInString(k)
		return tea.KeyPressMsg{Code: r, Text: k}
	}
}

// typeText presses every character of s.
func typeText(m *Model, s string) {
	for _, r := range s {
		press(m, string(r))
	}
}

// isQuit reports whether cmd quits the program.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// TestModel_onKey_keyNames checks that the test key presses read as the key names onKey matches.
func TestModel_onKey_keyNames(t *testing.T) {
	for _, k := range []string{keyEnter, keyEsc, keyCtrlC, keyFilter, keyDetails} {
		if got := keyMsg(k).String(); got != k {
			t.Errorf("keyMsg(%q).String() = %q, want %q", k, got, k)
		}
	}
}

// TestColumns checks that the table columns start with Profile and Region and leave out Tags.
func TestColumns(t *testing.T) {
	got := columns(common.Headers(fakeRow{}))
	want := []string{"Profile", "Region", "ID", "Name", "IPs", "Info", "Size"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("columns(fakeRow headers) = %v, want %v", got, want)
	}
}

// TestNewItem checks the cells and the filter text of rows with every kind of field.
func TestNewItem(t *testing.T) {
	tests := []struct {
		name      string
		row       interface{}
		wantCells []string
		wantWords []string
	}{
		{
			name: "every kind of field",
			row: fakeRow{
				ID: "i-1", Name: "web", IPs: []string{"10.0.0.1", "10.0.0.2"}, Info: info{Kind: "primary", Skip: "x"},
				Size: 8, Tags: map[string]string{"b": "2", "a": "1"}, Inner: "hidden",
			},
			wantCells: []string{"i-1", "web", "10.0.0.1, 10.0.0.2", "Kind: primary", "8"},
			wantWords: []string{"p", "r", "i-1", "web", "10.0.0.1", "10.0.0.2", "kind: primary", "8", "a=1", "b=2"},
		},
		{
			name:      "pointer to a row",
			row:       &fakeRow{ID: "i-2"},
			wantCells: []string{"i-2", "", "", "", "0"},
			wantWords: []string{"p", "r", "i-2", "", "", "0"},
		},
		{
			name:      "not a struct",
			row:       "plain",
			wantCells: []string{"plain"},
			wantWords: []string{"p", "r", "plain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newItem("p", "r", tt.row)
			if !reflect.DeepEqual(got.cells, tt.wantCells) {
				t.Errorf("newItem(%v).cells = %q, want %q", tt.row, got.cells, tt.wantCells)
			}
			if !reflect.DeepEqual(got.words, tt.wantWords) {
				t.Errorf("newItem(%v).words = %q, want %q", tt.row, got.words, tt.wantWords)
			}
		})
	}
}

// TestMatch checks the filter: every item for an empty query, case-insensitive text terms that
// must all match, fuzzy terms with ~, and no match across two words or on letters spread through
// a long value.
func TestMatch(t *testing.T) {
	items := []item{
		{words: []string{"prod", "eu-west-1", "web-1"}},
		{words: []string{"prod", "eu-west-1", "db-1"}},
		{words: []string{"dev", "us-east-1", "web-dev"}},
		{words: []string{"dev", "eu-west-1", "shared-elk1", "aws:autoscaling:groupname=shared-elk"}},
	}
	tests := []struct {
		name  string
		query string
		want  []int
	}{
		{name: "empty query", query: "  ", want: []int{0, 1, 2, 3}},
		{name: "text", query: "web", want: []int{0, 2}},
		{name: "case insensitive", query: "DB", want: []int{1}},
		{name: "every term", query: "web prod", want: []int{0}},
		{name: "part of a value", query: "elk", want: []int{3}},
		{name: "text not spread through a value", query: "stag", want: []int{}},
		{name: "fuzzy", query: "~wbdv", want: []int{2}},
		{name: "fuzzy spread through a value", query: "~stag", want: []int{3}},
		{name: "fuzzy and text", query: "~wbdv dev", want: []int{2}},
		{name: "lone prefix is text", query: "~", want: []int{}},
		{name: "not across two words", query: "~westdb", want: []int{}},
		{name: "no match", query: "zzz", want: []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := match(items, tt.query); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("match(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// TestModel_load checks that a load fills the table with every row, collects the errors of the
// failed searches and shows the progress.
func TestModel_load(t *testing.T) {
	m := loaded(t)

	if got, want := len(m.items), 3; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if got, want := len(m.table.Rows()), 3; got != want {
		t.Errorf("len(table rows) = %d, want %d", got, want)
	}
	if got, want := m.errs, []string{"dev eu-west-1: access denied"}; !reflect.DeepEqual(got, want) {
		t.Errorf("errs = %v, want %v", got, want)
	}
	if m.loading || m.done != 3 {
		t.Errorf("loading, done = %v, %d, want false, 3", m.loading, m.done)
	}

	view := m.View()
	for _, want := range []string{"awss ec2", "3 rows", "loaded in", "1 errors (e)", "web-1", "Profile"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("View() does not contain %q:\n%s", want, view.Content)
		}
	}
	if !view.AltScreen {
		t.Error("View().AltScreen = false, want true")
	}
}

// TestModel_loadProgress checks the title while the searches are still running.
func TestModel_loadProgress(t *testing.T) {
	m := newModel(t)
	m.load("ec2")
	m.Update(resultMsg{gen: m.gen, r: fixture()["ec2"][1]})

	if got, want := m.title(), "loading 1/3 searches"; !strings.Contains(got, want) {
		t.Errorf("title() = %q, want it to contain %q", got, want)
	}
	m.stop()
}

// TestModel_staleMessages checks that the messages of a replaced load are dropped.
func TestModel_staleMessages(t *testing.T) {
	m := loaded(t)
	old := m.gen
	run(t, m, press(m, keyRefresh))

	m.Update(resultMsg{gen: old, r: fixture()["vpc"][0]})
	m.Update(loadErrMsg{gen: old, err: errors.New("old")})
	m.Update(doneMsg{gen: old})

	if got, want := len(m.items), 3; got != want {
		t.Errorf("len(items) = %d, want %d", got, want)
	}
	if got, want := len(m.errs), 1; got != want {
		t.Errorf("len(errs) = %d, want %d", got, want)
	}
}

// TestModel_filter checks that the filter applies as you type, enter keeps it and esc clears it.
func TestModel_filter(t *testing.T) {
	m := loaded(t)

	press(m, keyFilter)
	typeText(m, "web")
	if m.mode != modeFilter {
		t.Fatalf("mode = %v, want modeFilter", m.mode)
	}
	if got, want := len(m.table.Rows()), 2; got != want {
		t.Errorf("rows after typing web = %d, want %d", got, want)
	}

	press(m, keyEnter)
	if m.mode != modeTable || m.query != "web" {
		t.Errorf("after enter: mode, query = %v, %q, want modeTable, %q", m.mode, m.query, "web")
	}
	if got, want := m.title(), `2/3 rows match "web"`; !strings.Contains(got, want) {
		t.Errorf("title() = %q, want it to contain %q", got, want)
	}

	press(m, keyFilter, keyEsc)
	if m.mode != modeTable || m.query != "" || len(m.table.Rows()) != 3 {
		t.Errorf("after esc in the filter: mode, query, rows = %v, %q, %d, want modeTable, \"\", 3",
			m.mode, m.query, len(m.table.Rows()))
	}

	m.query = "db"
	m.refresh()
	press(m, keyEsc)
	if m.query != "" || len(m.table.Rows()) != 3 {
		t.Errorf("after esc on the table: query, rows = %q, %d, want \"\", 3", m.query, len(m.table.Rows()))
	}
}

// TestModel_command checks the : commands: switch resource, unknown resource, empty and quit.
func TestModel_command(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		wantResource string
		wantRows     int
		wantMessage  string
		wantQuit     bool
	}{
		{name: "switch resource", command: "vpc", wantResource: "vpc", wantRows: 1},
		{name: "unknown resource", command: "sg", wantResource: "ec2", wantRows: 3, wantMessage: `unknown resource "sg"`},
		{name: "empty", command: "", wantResource: "ec2", wantRows: 3},
		{name: "quit", command: "quit", wantResource: "ec2", wantRows: 3, wantQuit: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loaded(t)
			press(m, keyCommand)
			typeText(m, tt.command)
			cmd := press(m, keyEnter)

			if got := isQuit(cmd); got != tt.wantQuit {
				t.Fatalf("quit = %v, want %v", got, tt.wantQuit)
			}
			if !tt.wantQuit {
				run(t, m, cmd)
			}
			if m.resource != tt.wantResource || len(m.items) != tt.wantRows {
				t.Errorf("resource, rows = %q, %d, want %q, %d", m.resource, len(m.items), tt.wantResource, tt.wantRows)
			}
			if !strings.Contains(m.message, tt.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", m.message, tt.wantMessage)
			}
		})
	}
}

// TestModel_commandEsc checks that esc closes the command line without running it.
func TestModel_commandEsc(t *testing.T) {
	m := loaded(t)
	press(m, keyCommand)
	typeText(m, "vpc")
	if got := press(m, keyEsc); got != nil || m.mode != modeTable || m.resource != "ec2" {
		t.Errorf("after esc: cmd, mode, resource = %v, %v, %q, want nil, modeTable, ec2", got, m.mode, m.resource)
	}
}

// TestModel_details checks that d shows the selected row as JSON, and esc goes back.
func TestModel_details(t *testing.T) {
	m := loaded(t)
	press(m, "down", keyDetails)

	if m.mode != modeDetails {
		t.Fatalf("mode = %v, want modeDetails", m.mode)
	}
	view := m.View().Content
	for _, want := range []string{"profile: prod", `"id": "i-2"`, `"kind": "primary"`} {
		if !strings.Contains(strings.ToLower(view), strings.ToLower(want)) {
			t.Errorf("details view does not contain %q:\n%s", want, view)
		}
	}

	press(m, "down", keyEsc)
	if m.mode != modeTable {
		t.Errorf("mode after esc = %v, want modeTable", m.mode)
	}
}

// TestModel_detailsEmpty checks that d does nothing when no row matches the filter.
func TestModel_detailsEmpty(t *testing.T) {
	m := loaded(t)
	m.query = "zzz"
	m.refresh()
	press(m, keyDetails)
	if m.mode != modeTable {
		t.Errorf("mode = %v, want modeTable", m.mode)
	}
}

// TestModel_errors checks that e lists the errors, or says there are none.
func TestModel_errors(t *testing.T) {
	m := loaded(t)
	press(m, keyErrors)
	if m.mode != modeErrors || !strings.Contains(m.View().Content, "access denied") {
		t.Errorf("after e: mode = %v, view:\n%s", m.mode, m.View().Content)
	}
	press(m, keyErrors)

	m.errs = nil
	press(m, keyErrors)
	if m.mode != modeTable || !strings.Contains(m.View().Content, "no errors") {
		t.Errorf("after e with no errors: mode = %v, view:\n%s", m.mode, m.View().Content)
	}
}

// TestModel_loadError checks that the error a loader returns is listed with the others.
func TestModel_loadError(t *testing.T) {
	m := newModel(t)
	m.cfg.Resource = "sg"
	run(t, m, m.Init())
	if got, want := m.errs, []string{"no such resource"}; !reflect.DeepEqual(got, want) {
		t.Errorf("errs = %v, want %v", got, want)
	}
}

// TestModel_quit checks that q and ctrl+c quit, from the table and from the input line.
func TestModel_quit(t *testing.T) {
	tests := []struct {
		name string
		keys []string
	}{
		{name: "q", keys: []string{keyQuit}},
		{name: "ctrl+c", keys: []string{keyCtrlC}},
		{name: "ctrl+c in the filter", keys: []string{keyFilter, keyCtrlC}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loaded(t)
			if cmd := press(m, tt.keys...); !isQuit(cmd) {
				t.Errorf("keys %v do not quit", tt.keys)
			}
		})
	}
}

// TestModel_stopDrains checks that stopping a load drains its channel, so a loader blocked on a
// full channel finishes.
func TestModel_stopDrains(t *testing.T) {
	finished := make(chan struct{})
	load := func(ctx context.Context, _ string, out chan<- common.Results) error {
		defer close(finished)
		defer close(out)
		for range bufferSize * 2 {
			out <- fixture()["vpc"][0]
		}
		return ctx.Err()
	}
	m := New(context.Background(), Config{Resource: "ec2", Load: load})
	loader := m.Init()().(tea.BatchMsg)[0]
	go loader()

	// Wait until the loader is blocked on the full channel.
	deadline := time.Now().Add(5 * time.Second)
	for len(m.ch) < cap(m.ch) {
		if time.Now().After(deadline) {
			t.Fatal("the channel never filled")
		}
		time.Sleep(time.Millisecond)
	}
	m.stop()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the loader is still blocked after stop")
	}
}
