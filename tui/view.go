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
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

// Help lines of the footer.
const (
	tableHelp = "/ filter (~ fuzzy)  : resource  d details  e errors  r refresh  esc clear  q quit"
	pagerHelp = "↑/↓ scroll  esc back"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	helpStyle  = lipgloss.NewStyle().Faint(true)
)

// refresh applies the filter and puts the matching items in the table.
func (m *Model) refresh() {
	m.shown = match(m.items, m.query)

	rows := make([]table.Row, len(m.shown))
	for i, idx := range m.shown {
		it := m.items[idx]
		rows[i] = append(table.Row{it.profile, it.region}, it.cells...)
	}

	// The rows go first: the table renders every row with the columns it has.
	m.table.SetRows(nil)
	m.table.SetColumns(tableColumns(m.headers, rows))
	// The table sizes its rows from the height of its header, which changes with the columns.
	m.table.SetHeight(max(m.height-chromeLines, 1))
	m.table.SetRows(rows)
	if m.table.Cursor() < 0 && len(rows) > 0 {
		m.table.SetCursor(0)
	}
}

// tableColumns sizes each column to its widest value, between its title and maxColumnWidth.
func tableColumns(headers []string, rows []table.Row) []table.Column {
	cols := make([]table.Column, len(headers))
	for i, h := range headers {
		cols[i] = table.Column{Title: h, Width: utf8.RuneCountInString(h)}
	}
	for _, row := range rows {
		for i := 0; i < len(row) && i < len(cols); i++ {
			cols[i].Width = max(cols[i].Width, min(utf8.RuneCountInString(row[i]), maxColumnWidth))
		}
	}
	return cols
}

// render returns the screen of the current mode.
func (m *Model) render() string {
	switch m.mode {
	case modeDetails, modeErrors:
		return lipgloss.JoinVertical(lipgloss.Left, m.title(), m.pager.View(), helpStyle.Render(pagerHelp))
	default:
		return lipgloss.JoinVertical(lipgloss.Left, m.title(), m.table.View(), m.footer())
	}
}

// title returns the top line: the resource, the rows shown, the progress and the failures.
func (m *Model) title() string {
	parts := []string{"awss " + m.resource}

	rows := fmt.Sprintf("%d rows", len(m.items))
	if m.query != "" {
		rows = fmt.Sprintf("%d/%d rows match %q", len(m.shown), len(m.items), m.query)
	}
	parts = append(parts, rows)

	if m.loading {
		parts = append(parts, fmt.Sprintf("loading %d/%d searches", m.done, m.cfg.Searches))
	} else {
		parts = append(parts, fmt.Sprintf("loaded in %s", m.elapsed.Round(time.Millisecond)))
	}
	if len(m.errs) > 0 {
		parts = append(parts, fmt.Sprintf("%d errors (e)", len(m.errs)))
	}
	return titleStyle.Render(strings.Join(parts, " · "))
}

// footer returns the bottom line: the input line, a message or the help.
func (m *Model) footer() string {
	switch {
	case m.mode == modeFilter || m.mode == modeCommand:
		return m.input.View()
	case m.message != "":
		return m.message
	default:
		return helpStyle.Render(tableHelp)
	}
}
