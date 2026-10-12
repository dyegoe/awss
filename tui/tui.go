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

// Package tui is an experimental terminal UI, in the style of k9s, to browse the resources of a
// search: it loads them once across the profiles and regions, keeps them in memory, and filters
// them as you type.
package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dyegoe/awss/common"
)

// Loader runs the search of resource across the profiles and regions and sends every result set
// to out. It closes out when every search has finished, also when it returns an error.
type Loader func(ctx context.Context, resource string, out chan<- common.Results) error

// Config is what the TUI needs to run.
type Config struct {
	// Resource is the resource shown first, such as ec2.
	Resource string

	// Resources are the resources the : command switches to.
	Resources []string

	// Searches is how many profile and region searches one load makes, for the progress line.
	Searches int

	// Load runs the searches of a resource.
	Load Loader
}

// mode is what the keys act on.
type mode int

const (
	modeTable mode = iota
	modeFilter
	modeCommand
	modeDetails
	modeErrors
)

const (
	// bufferSize is how many finished result sets may wait for the UI.
	bufferSize = 32

	// maxColumnWidth caps a column, so one long value does not push the others off the screen.
	maxColumnWidth = 40

	// chromeLines are the lines around the table: the title and the footer.
	chromeLines = 2
)

// resultMsg carries one result set of the load numbered gen.
type resultMsg struct {
	gen int
	r   common.Results
}

// doneMsg tells that every search of the load numbered gen has finished.
type doneMsg struct{ gen int }

// loadErrMsg carries the error the loader of load gen returned.
type loadErrMsg struct {
	gen int
	err error
}

// Model is the state of the TUI. It implements tea.Model.
type Model struct {
	cfg Config

	// ctx is the parent of every load; cancel stops the current one.
	ctx    context.Context
	cancel context.CancelFunc

	// gen numbers the loads, so the messages of a replaced load are dropped; ch is its channel.
	gen int
	ch  chan common.Results

	resource string
	headers  []string
	items    []item
	shown    []int
	errs     []string
	done     int
	loading  bool
	started  time.Time
	elapsed  time.Duration

	mode    mode
	query   string
	message string

	table  table.Model
	input  textinput.Model
	pager  viewport.Model
	width  int
	height int
}

// New returns the model of the TUI. ctx is the parent of every load.
func New(ctx context.Context, cfg Config) *Model {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	styles.Selected = lipgloss.NewStyle().Reverse(true)

	return &Model{
		cfg:   cfg,
		ctx:   ctx,
		table: table.New(table.WithFocused(true), table.WithStyles(styles)),
		input: textinput.New(),
		pager: pager(),
	}
}

// pager returns the scrollable view of the details and the errors. It wraps long lines.
func pager() viewport.Model {
	p := viewport.New()
	p.SoftWrap = true
	return p
}

// Run starts the TUI on the terminal and returns when the user quits.
func Run(ctx context.Context, cfg Config) error {
	m := New(ctx, cfg)
	defer m.stop()
	if _, err := tea.NewProgram(m, tea.WithContext(ctx)).Run(); err != nil {
		return fmt.Errorf("running the terminal UI: %w", err)
	}
	return nil
}

// Init starts loading the first resource.
func (m *Model) Init() tea.Cmd {
	return m.load(m.cfg.Resource)
}

// Update handles a message and returns the next command.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
	case resultMsg:
		cmd := m.onResult(msg)
		return m, cmd
	case doneMsg:
		if msg.gen == m.gen {
			m.loading = false
			m.elapsed = time.Since(m.started)
		}
	case loadErrMsg:
		if msg.gen == m.gen && msg.err != nil {
			m.errs = append(m.errs, msg.err.Error())
		}
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// View renders the screen.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// load stops the current load and starts loading resource, with an empty table.
func (m *Model) load(resource string) tea.Cmd {
	m.stop()

	m.gen++
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.ch = make(chan common.Results, bufferSize)

	m.resource = resource
	m.headers, m.items, m.errs = nil, nil, nil
	m.done, m.loading, m.started, m.message = 0, true, time.Now(), ""
	m.refresh()

	gen, ch, load := m.gen, m.ch, m.cfg.Load
	run := func() tea.Msg { return loadErrMsg{gen: gen, err: load(ctx, resource, ch)} }
	return tea.Batch(run, next(gen, ch))
}

// stop cancels the current load and drains its channel, so its searches do not block on a full
// channel nobody reads any more.
func (m *Model) stop() {
	if m.cancel == nil {
		return
	}
	m.cancel()
	go drain(m.ch)
}

// drain reads ch until it is closed.
func drain(ch <-chan common.Results) {
	for range ch { //nolint:revive // reading is the point: it unblocks the searches still sending
	}
}

// next returns the command that waits for the next result set of load gen.
func next(gen int, ch <-chan common.Results) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return doneMsg{gen: gen}
		}
		return resultMsg{gen: gen, r: r}
	}
}

// onResult adds a result set of the current load to the table and waits for the next one.
func (m *Model) onResult(msg resultMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil
	}

	r := msg.r
	m.done++
	if m.headers == nil {
		m.headers = columns(r.GetHeaders())
	}
	for _, e := range r.GetErrors() {
		m.errs = append(m.errs, fmt.Sprintf("%s %s: %s", r.GetProfile(), r.GetRegion(), e))
	}
	for _, row := range r.GetRows() {
		m.items = append(m.items, newItem(r.GetProfile(), r.GetRegion(), row))
	}
	m.refresh()

	return next(m.gen, m.ch)
}

// resize fits the table and the pager to the terminal.
func (m *Model) resize(width, height int) {
	m.width, m.height = width, height
	m.table.SetWidth(width)
	m.refresh()
	m.pager.SetWidth(width)
	m.pager.SetHeight(max(height-chromeLines, 1))
	m.input.SetWidth(max(width-2, 1))
}
