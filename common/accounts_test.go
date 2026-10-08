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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// profileAccountNamesCase is one table entry of TestProfileAccountNames.
type profileAccountNamesCase struct {
	name    string
	config  string
	missing bool
	want    map[string]string
	wantErr string
}

// profileAccountNamesCases are the AWS config files of TestProfileAccountNames.
func profileAccountNamesCases() []profileAccountNamesCase {
	return []profileAccountNamesCase{
		{
			name:   "sso_account_id",
			config: "[profile prod]\nsso_account_id = 111111111111\nsso_role_name = Admin\n",
			want:   map[string]string{"111111111111": "prod"},
		},
		{
			name:   "role_arn",
			config: "[profile network]\nrole_arn = arn:aws:iam::222222222222:role/ReadOnly\nsource_profile = default\n",
			want:   map[string]string{"222222222222": "network"},
		},
		{
			name: "granted_sso_account_id",
			config: "[profile shared]\ngranted_sso_account_id = 333333333333\n" +
				"credential_process = granted credential-process --profile shared\n",
			want: map[string]string{"333333333333": "shared"},
		},
		{
			name:   "default profile",
			config: "[default]\nsso_account_id = 444444444444\n",
			want:   map[string]string{"444444444444": "default"},
		},
		{
			name:   "role_arn wins over sso_account_id",
			config: "[profile chained]\nsso_account_id = 111111111111\nrole_arn = arn:aws:iam::222222222222:role/x\n",
			want:   map[string]string{"222222222222": "chained"},
		},
		{
			name: "several profiles for one account: first alphabetically",
			config: "[profile prod-admin]\nsso_account_id = 111111111111\n" +
				"[profile prod-readonly]\nsso_account_id = 111111111111\n" +
				"[profile a-prod]\nsso_account_id = 111111111111\n",
			want: map[string]string{"111111111111": "a-prod"},
		},
		{
			name: "profiles without an account ID are skipped",
			config: "[profile keys]\naws_access_key_id = AKIA\n" +
				"[profile process]\ncredential_process = /bin/creds\n" +
				"[profile bad-arn]\nrole_arn = not-an-arn\n" +
				"[profile short]\nsso_account_id = 1234\n",
			want: map[string]string{},
		},
		{
			name:   "sections that are not profiles are skipped",
			config: "[sso-session corp]\nsso_account_id = 555555555555\n[services dev]\n",
			want:   map[string]string{},
		},
		{name: "no AWS config file", missing: true, want: map[string]string{}},
		{
			name:    "a file that cannot be parsed",
			config:  "[profile unclosed\nsso_account_id = 111111111111\n",
			wantErr: "reading the AWS config file",
		},
	}
}

// TestProfileAccountNames checks the account names read from the AWS config file: each key that
// names an account, the precedence between profiles of one account, and the files that give none.
func TestProfileAccountNames(t *testing.T) {
	for _, tt := range profileAccountNamesCases() {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config")
			if !tt.missing {
				if err := os.WriteFile(path, []byte(tt.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("AWS_CONFIG_FILE", path)

			got, err := ProfileAccountNames()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ProfileAccountNames() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProfileAccountNames() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ProfileAccountNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
