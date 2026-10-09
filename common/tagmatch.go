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
	"sort"
)

// TagMatcher matches a resource's tags against tag filters, client-side, for the searches whose
// API cannot filter on tags.
//
// The filters use the syntax of ParseTags: Key=Value1:Value2. A resource matches when it has every
// key (AND) with a value matching one of that key's values (OR). Values are globs, as in Matcher.
// No filter matches everything.
type TagMatcher struct {
	keys   []string
	values map[string]*Matcher
}

// NewTagMatcher parses the tag filters. It returns an error for a filter without Key=Value or a
// value that is not a valid glob.
func NewTagMatcher(tags []string) (*TagMatcher, error) {
	parsed, err := ParseTags(tags)
	if err != nil {
		return nil, err
	}
	m := &TagMatcher{values: make(map[string]*Matcher, len(parsed))}
	for key, values := range parsed {
		vm, err := NewMatcher(values, false)
		if err != nil {
			return nil, fmt.Errorf("tag %s: %w", key, err)
		}
		m.keys = append(m.keys, key)
		m.values[key] = vm
	}
	sort.Strings(m.keys)
	return m, nil
}

// Match reports whether tags has every key of the filters with a matching value.
func (m *TagMatcher) Match(tags map[string]string) bool {
	for _, key := range m.keys {
		value, ok := tags[key]
		if !ok || !m.values[key].Match(value) {
			return false
		}
	}
	return true
}
