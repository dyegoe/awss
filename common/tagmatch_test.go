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
	"strings"
	"testing"
)

// TestTagMatcher_Match checks the AND between keys, the OR between the values of a key, globs in
// values, and a missing key.
func TestTagMatcher_Match(t *testing.T) {
	tags := map[string]string{"Env": "prod", "Team": "network", "Empty": ""}
	tests := []struct {
		name    string
		filters []string
		want    bool
	}{
		{name: "no filter matches", filters: nil, want: true},
		{name: "one key, one value", filters: []string{"Env=prod"}, want: true},
		{name: "one key, wrong value", filters: []string{"Env=dev"}, want: false},
		{name: "values of one key are alternatives", filters: []string{"Env=dev:prod"}, want: true},
		{name: "every key must match", filters: []string{"Env=prod", "Team=network"}, want: true},
		{name: "one key fails the AND", filters: []string{"Env=prod", "Team=data"}, want: false},
		{name: "glob in the value", filters: []string{"Team=net*"}, want: true},
		{name: "missing key", filters: []string{"Owner=*"}, want: false},
		{name: "any value of a present key", filters: []string{"Empty=*"}, want: true},
		{name: "values are case-sensitive", filters: []string{"Env=PROD"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewTagMatcher(tt.filters)
			if err != nil {
				t.Fatalf("NewTagMatcher(%v) error = %v", tt.filters, err)
			}
			if got := m.Match(tags); got != tt.want {
				t.Errorf("Match(%v) with %v = %v, want %v", tags, tt.filters, got, tt.want)
			}
		})
	}
}

// TestNewTagMatcher_errors checks that a filter without Key=Value and a bad glob are errors.
func TestNewTagMatcher_errors(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		wantErr string
	}{
		{name: "no equals sign", filters: []string{"Env"}, wantErr: "invalid tag format: Env"},
		{name: "bad glob", filters: []string{"Env=[prod"}, wantErr: "tag Env: invalid pattern"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTagMatcher(tt.filters)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("NewTagMatcher(%v) error = %v, want it to contain %q", tt.filters, err, tt.wantErr)
			}
		})
	}
}
