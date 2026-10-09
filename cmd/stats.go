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
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search"
)

const (
	// elapsedPrecision is how precisely --stats prints an elapsed time of a second or more: 1.97s.
	// A shorter one is printed in milliseconds.
	elapsedPrecision = 10 * time.Millisecond

	// wholeUnits is the size from which formatBytes drops the decimal: 146 MB, but 1.4 GB.
	wholeUnits = 100
)

// runStats is what --stats prints about a run.
type runStats struct {
	elapsed time.Duration

	// peakMemory is in bytes; hasPeakMemory is false when the OS does not report it.
	peakMemory    int64
	hasPeakMemory bool

	summary search.Summary
	api     common.APICallCounts
}

// collectStats gathers the stats of the run that started at start and ended with summary.
func collectStats(start time.Time, summary *search.Summary) runStats {
	peak, ok := common.PeakMemory()
	return runStats{
		elapsed:       time.Since(start),
		peakMemory:    peak,
		hasPeakMemory: ok,
		summary:       *summary,
		api:           common.APICalls.Counts(),
	}
}

// writeFailures writes one line on w when at least one search failed, and nothing otherwise. The
// failed searches already show their errors in their own result sets; the line makes sure they
// are not missed among the others.
func writeFailures(w io.Writer, s *search.Summary) {
	if s.Failed() == 0 {
		return
	}
	var kinds []string
	if s.Errors > 0 {
		kinds = append(kinds, plural(s.Errors, "error", "errors"))
	}
	if s.TimedOut > 0 {
		kinds = append(kinds, thousands(int64(s.TimedOut))+" timed out")
	}
	fmt.Fprintf(w, "awss: %s of %s failed (%s); see the result sets marked with errors\n",
		thousands(int64(s.Failed())), plural(s.Searches, "search", "searches"), strings.Join(kinds, ", "))
}

// writeStats writes the stats of a run on w, one value per line.
func writeStats(w io.Writer, st *runStats) {
	s := &st.summary
	memory := "n/a"
	if st.hasPeakMemory {
		memory = formatBytes(st.peakMemory)
	}
	lines := [][2]string{
		{"elapsed", formatElapsed(st.elapsed)},
		{"peak memory", memory},
		{"profiles", fmt.Sprintf("%s   regions %s   searches %s (concurrency %s)",
			thousands(int64(s.Profiles)), thousands(int64(s.Regions)),
			thousands(int64(s.Searches)), thousands(int64(s.Concurrency)))},
		{"searches", fmt.Sprintf("%s with results, %s empty, %s failed, %s timed out",
			thousands(int64(s.WithResults)), thousands(int64(s.Empty)),
			thousands(int64(s.Errors)), thousands(int64(s.TimedOut)))},
		{"resources", thousands(int64(s.Resources))},
		{"API calls", fmt.Sprintf("%s (%s retries, %s throttled)",
			thousands(st.api.Calls), thousands(st.api.Retries), thousands(st.api.Throttled))},
	}
	fmt.Fprintln(w, "awss stats")
	for _, l := range lines {
		fmt.Fprintf(w, "  %-15s%s\n", l[0], l[1])
	}
}

// formatElapsed rounds d for --stats: 1.97s, 2m3.46s, or 3ms under a second.
func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(elapsedPrecision).String()
}

// plural returns n with the singular or plural noun: "1 search", "2,550 searches".
func plural(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return thousands(int64(n)) + " " + plural
}

// thousands formats n with a comma between groups of three digits: 2550 is "2,550".
func thousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, s = "-", s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return sign + s
}

// formatBytes formats a size in bytes with a decimal unit: 146 MB, 1.4 GB.
func formatBytes(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	v := float64(b)
	units := []string{"KB", "MB", "GB", "TB"}
	i := -1
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	if v >= wholeUnits {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
