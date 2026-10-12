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
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Keys the TUI acts on.
const (
	keyQuit    = "q"
	keyCtrlC   = "ctrl+c"
	keyEsc     = "esc"
	keyEnter   = "enter"
	keyBack    = "backspace"
	keyFilter  = "/"
	keyCommand = ":"
	keyDetails = "d"
	keyErrors  = "e"
	keyRefresh = "r"
)

// onKey handles a key press in the current mode.
func (m *Model) onKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case k.String() == keyCtrlC:
		cmd = m.quit()
	case m.mode == modeFilter || m.mode == modeCommand:
		cmd = m.onInputKey(k)
	case m.mode == modeDetails || m.mode == modeErrors:
		cmd = m.onPagerKey(k)
	default:
		cmd = m.onTableKey(k)
	}
	return m, cmd
}

// quit stops the current load and quits.
func (m *Model) quit() tea.Cmd {
	m.stop()
	return tea.Quit
}

// onTableKey handles a key press on the table.
func (m *Model) onTableKey(k tea.KeyPressMsg) tea.Cmd {
	m.message = ""
	switch k.String() {
	case keyQuit:
		return m.quit()
	case keyFilter:
		return m.openInput(modeFilter, keyFilter, m.query)
	case keyCommand:
		return m.openInput(modeCommand, keyCommand, "")
	case keyDetails, keyEnter:
		m.openDetails()
	case keyErrors:
		m.openErrors()
	case keyRefresh:
		return m.load(m.resource)
	case keyEsc:
		m.query = ""
		m.refresh()
	default:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(k)
		return cmd
	}
	return nil
}

// openInput shows the input line with prompt and value, for the filter or a command.
func (m *Model) openInput(md mode, prompt, value string) tea.Cmd {
	m.mode = md
	m.input.Prompt = promptStyle.Render(prompt)
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m.input.Focus()
}

// onInputKey handles a key press on the input line. The filter applies as you type. Backspace on
// an empty line closes it, as esc does.
func (m *Model) onInputKey(k tea.KeyPressMsg) tea.Cmd {
	key := k.String()
	if key == keyBack && m.input.Value() == "" {
		key = keyEsc
	}
	switch key {
	case keyEsc:
		if m.mode == modeFilter {
			m.query = ""
			m.refresh()
		}
		m.closeInput()
		return nil
	case keyEnter:
		md, value := m.mode, strings.TrimSpace(m.input.Value())
		m.closeInput()
		if md == modeCommand {
			return m.runCommand(value)
		}
		return nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	if m.mode == modeFilter {
		m.query = m.input.Value()
		m.refresh()
	}
	return cmd
}

// closeInput hides the input line and goes back to the table.
func (m *Model) closeInput() {
	m.input.Blur()
	m.mode = modeTable
}

// runCommand runs a : command: a resource to switch to, or q to quit.
func (m *Model) runCommand(cmd string) tea.Cmd {
	switch {
	case cmd == "":
		return nil
	case cmd == keyQuit || cmd == "quit":
		return m.quit()
	case slices.Contains(m.cfg.Resources, cmd):
		m.query = ""
		return m.load(cmd)
	default:
		m.message = fmt.Sprintf("unknown resource %q: use one of %s", cmd, strings.Join(m.cfg.Resources, ", "))
		return nil
	}
}

// onPagerKey handles a key press on the details or errors view.
func (m *Model) onPagerKey(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case keyEsc, keyQuit, keyDetails, keyErrors:
		m.mode = modeTable
		return nil
	}
	var cmd tea.Cmd
	m.pager, cmd = m.pager.Update(k)
	return cmd
}

// selected returns the item under the cursor, and false when the table is empty.
func (m *Model) selected() (item, bool) {
	c := m.table.Cursor()
	if c < 0 || c >= len(m.shown) {
		return item{}, false
	}
	return m.items[m.shown[c]], true
}

// openDetails shows the selected row as JSON.
func (m *Model) openDetails() {
	it, ok := m.selected()
	if !ok {
		return
	}
	b, err := json.MarshalIndent(it.row, "", "  ")
	body := string(b)
	if err != nil {
		body = fmt.Sprintf("cannot show the row: %v", err)
	}
	m.openPager(modeDetails, fmt.Sprintf("profile: %s\nregion:  %s\n\n%s", it.profile, it.region, body))
}

// openErrors shows the errors of the failed searches.
func (m *Model) openErrors() {
	if len(m.errs) == 0 {
		m.message = "no errors"
		return
	}
	m.openPager(modeErrors, strings.Join(m.errs, "\n"))
}

// openPager shows content in the scrollable pager, from the top.
func (m *Model) openPager(md mode, content string) {
	m.mode = md
	m.pager.SetContent(content)
	m.pager.GotoTop()
}
