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

package common

import "testing"

// TestNewMatcher_errors checks that an invalid glob or regular expression is rejected.
func TestNewMatcher_errors(t *testing.T) {
	if _, err := NewMatcher([]string{"[unclosed"}, false); err == nil {
		t.Error("NewMatcher(bad glob) error = nil, want error")
	}
	if _, err := NewMatcher([]string{"(unclosed"}, true); err == nil {
		t.Error("NewMatcher(bad regex) error = nil, want error")
	}
}

// TestMatcher_Match checks glob and regex matching: globs are anchored, regexes match anywhere
// unless anchored, and no pattern matches everything.
func TestMatcher_Match(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		regex    bool
		value    string
		want     bool
	}{
		{name: "no patterns matches everything", patterns: nil, value: "anything", want: true},
		{name: "glob star", patterns: []string{"prod-*"}, value: "prod-logs", want: true},
		{name: "glob star no match", patterns: []string{"prod-*"}, value: "dev-logs", want: false},
		{name: "glob is anchored", patterns: []string{"logs"}, value: "prod-logs", want: false},
		{name: "glob question mark", patterns: []string{"bucket-?"}, value: "bucket-1", want: true},
		{name: "glob any of several", patterns: []string{"dev-*", "prod-*"}, value: "prod-logs", want: true},
		{name: "regex substring", patterns: []string{"logs"}, regex: true, value: "prod-logs", want: true},
		{name: "regex anchored", patterns: []string{"^logs$"}, regex: true, value: "prod-logs", want: false},
		{name: "regex alternation", patterns: []string{"^(dev|prod)-"}, regex: true, value: "dev-x", want: true},
		{name: "glob metachars literal in regex mode", patterns: []string{"a.c"}, regex: true, value: "abc", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.patterns, tt.regex)
			if err != nil {
				t.Fatalf("NewMatcher() error = %v", err)
			}
			if got := m.Match(tt.value); got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestMatcher_MatchAny checks that any matching value is enough, that no value does not match,
// and that no pattern matches everything, even no values.
func TestMatcher_MatchAny(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		values   []string
		want     bool
	}{
		{name: "no patterns, no values", patterns: nil, values: nil, want: true},
		{name: "no patterns, values", patterns: nil, values: []string{"a"}, want: true},
		{name: "patterns, no values", patterns: []string{"*.example.com"}, values: nil, want: false},
		{
			name: "second value matches", patterns: []string{"*.example.com"},
			values: []string{"x.org", "www.example.com"}, want: true,
		},
		{name: "no value matches", patterns: []string{"*.example.com"}, values: []string{"x.org", "y.net"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.patterns, false)
			if err != nil {
				t.Fatalf("NewMatcher() error = %v", err)
			}
			if got := m.MatchAny(tt.values); got != tt.want {
				t.Errorf("MatchAny(%q) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

// TestMatcher_Prefix checks the literal prefix sent to S3: the text before the first glob
// metacharacter, or after ^ in a regex, and none when the patterns share no prefix.
func TestMatcher_Prefix(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		regex    bool
		want     string
	}{
		{name: "no patterns", patterns: nil, want: ""},
		{name: "glob literal before star", patterns: []string{"prod-*-logs"}, want: "prod-"},
		{name: "glob no metachar is whole", patterns: []string{"exact"}, want: "exact"},
		{name: "glob starting with star", patterns: []string{"*-logs"}, want: ""},
		{name: "glob question mark", patterns: []string{"ab?c"}, want: "ab"},
		{name: "glob bracket", patterns: []string{"ab[cd]"}, want: "ab"},
		{name: "two globs have no common prefix", patterns: []string{"prod-*", "prod-x"}, want: ""},
		{name: "regex literal prefix", patterns: []string{"^prod-.*"}, regex: true, want: "prod-"},
		{name: "regex unanchored matches anywhere so no prefix", patterns: []string{"prod-.*"}, regex: true, want: ""},
		{name: "regex anchored alternation has none", patterns: []string{"^(a|b)"}, regex: true, want: ""},
		{name: "regex alternation has none", patterns: []string{"(a|b)"}, regex: true, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.patterns, tt.regex)
			if err != nil {
				t.Fatalf("NewMatcher() error = %v", err)
			}
			if got := m.Prefix(); got != tt.want {
				t.Errorf("Prefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
