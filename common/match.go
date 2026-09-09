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

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Matcher matches strings against a set of patterns, client-side.
//
// Patterns are globs (`*` and `?`, see path.Match) unless the matcher was built with regex,
// in which case they are Go regular expressions. A value matches when any pattern matches.
// An empty pattern set matches everything.
type Matcher struct {
	globs    []string
	regexes  []*regexp.Regexp
	patterns []string // the regex sources, kept to derive Prefix
}

// NewMatcher compiles the patterns. It returns an error for an invalid glob or regular expression.
func NewMatcher(patterns []string, regex bool) (*Matcher, error) {
	m := &Matcher{}
	for _, p := range patterns {
		if regex {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("invalid regular expression %q: %w", p, err)
			}
			m.regexes = append(m.regexes, re)
			m.patterns = append(m.patterns, p)
			continue
		}
		if _, err := path.Match(p, ""); err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", p, err)
		}
		m.globs = append(m.globs, p)
	}
	return m, nil
}

// Match reports whether value matches any of the patterns.
func (m *Matcher) Match(value string) bool {
	if len(m.globs) == 0 && len(m.regexes) == 0 {
		return true
	}
	for _, g := range m.globs {
		if ok, _ := path.Match(g, value); ok {
			return true
		}
	}
	for _, re := range m.regexes {
		if re.MatchString(value) {
			return true
		}
	}
	return false
}

// Prefix returns the literal prefix every match must start with, or "" when there is none.
//
// It is only defined when the matcher has exactly one pattern, so callers can narrow a
// server-side listing (for example an S3 prefix) without excluding matches of other patterns.
// A regular expression only yields a prefix when it is anchored with `^`, since an unanchored
// one matches anywhere in the value.
func (m *Matcher) Prefix() string {
	switch {
	case len(m.globs) == 1 && len(m.regexes) == 0:
		return globPrefix(m.globs[0])
	case len(m.regexes) == 1 && len(m.globs) == 0:
		return regexPrefix(m.patterns[0])
	default:
		return ""
	}
}

// regexPrefix returns the literal prefix of an anchored regular expression, or "" otherwise.
func regexPrefix(pattern string) string {
	if !strings.HasPrefix(pattern, "^") {
		return ""
	}
	re, err := regexp.Compile(strings.TrimPrefix(pattern, "^"))
	if err != nil {
		return ""
	}
	prefix, _ := re.LiteralPrefix()
	return prefix
}

// globPrefix returns the part of a glob before its first metacharacter.
func globPrefix(glob string) string {
	if i := strings.IndexAny(glob, `*?[\`); i >= 0 {
		return glob[:i]
	}
	return glob
}
