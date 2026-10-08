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
	"reflect"
	"strings"
	"testing"
)

// configuredAccountNamesCase is one table entry of TestConfiguredAccountNames.
type configuredAccountNamesCase struct {
	name         string
	raw          interface{}
	want         map[string]string
	wantWarnings []string
}

// configuredAccountNamesCases are the accounts maps of TestConfiguredAccountNames.
func configuredAccountNamesCases() []configuredAccountNamesCase {
	return []configuredAccountNamesCase{
		{name: "no accounts key", raw: nil, want: map[string]string{}},
		{
			name: "names keep their case",
			raw:  map[string]interface{}{"123456789012": "Network-Prod", "210987654321": "shared"},
			want: map[string]string{"123456789012": "Network-Prod", "210987654321": "shared"},
		},
		{
			name: "an unquoted ID loses its leading zero, which is restored",
			raw:  map[string]interface{}{"12345678901": "zero-prefixed"},
			want: map[string]string{"012345678901": "zero-prefixed"},
		},
		{
			name: "a name YAML read as a number",
			raw:  map[string]interface{}{"123456789012": 2024},
			want: map[string]string{"123456789012": "2024"},
		},
		{
			name: "entries that are not account IDs or have no name are skipped",
			raw: map[string]interface{}{
				"prod": "123456789012", "1234567890123": "too-long", "123456789012": "", "210987654321": nil,
				"111111111111": "kept",
			},
			want: map[string]string{"111111111111": "kept"},
			wantWarnings: []string{
				"accounts: 123456789012 has no name; ignored",
				`accounts: "1234567890123" is not a 12-digit account ID; ignored`,
				"accounts: 210987654321 has no name; ignored",
				`accounts: "prod" is not a 12-digit account ID; ignored`,
			},
		},
		{
			name: "the same ID quoted and unquoted",
			raw:  map[string]interface{}{"012345678901": "quoted", "12345678901": "unquoted"},
			want: map[string]string{"012345678901": "quoted"},
			wantWarnings: []string{
				`accounts: 012345678901 is listed twice; keeping "quoted"`,
			},
		},
		{
			name:         "not a map",
			raw:          []interface{}{"123456789012"},
			want:         map[string]string{},
			wantWarnings: []string{"accounts: expected a map of account ID to name; ignored"},
		},
	}
}

// TestConfiguredAccountNames checks the accounts map of the awss config file: the IDs it accepts,
// the leading zeros it restores, and the entries it skips with a warning.
func TestConfiguredAccountNames(t *testing.T) {
	for _, tt := range configuredAccountNamesCases() {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings := ConfiguredAccountNames(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConfiguredAccountNames(%v) = %v, want %v", tt.raw, got, tt.want)
			}
			if strings.Join(warnings, "\n") != strings.Join(tt.wantWarnings, "\n") {
				t.Errorf("ConfiguredAccountNames(%v) warnings = %q, want %q", tt.raw, warnings, tt.wantWarnings)
			}
		})
	}
}
