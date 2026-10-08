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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"gopkg.in/ini.v1"
)

// accountIDLength is the number of digits of an AWS account ID.
const accountIDLength = 12

// accountIDKeys are the AWS config file keys that name a profile's account, in the order they are
// read. role_arn comes first: in a chained profile it is the account the profile ends up in.
var accountIDKeys = []string{"role_arn", "sso_account_id", "granted_sso_account_id"}

// ProfileAccountNames returns the account names found in the AWS config file (see awsConfigFile),
// keyed by account ID. The name of an account is the name of a profile that points to it.
//
// A profile names its account in role_arn (the account inside the ARN), sso_account_id or, for
// Granted, granted_sso_account_id. A profile with none of them is skipped. When several profiles
// point to the same account, the first in alphabetical order wins.
//
// A missing file gives an empty map and no error: account names are optional. A file that
// exists but cannot be read gives an error.
func ProfileAccountNames() (map[string]string, error) {
	path := awsConfigFile()
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	cfg, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("reading the AWS config file: %w", err)
	}

	type profile struct{ name, id string }
	profiles := []profile{}
	for _, section := range cfg.Sections() {
		name, ok := profileName(section.Name())
		if !ok {
			continue
		}
		if id := sectionAccountID(section); id != "" {
			profiles = append(profiles, profile{name: name, id: id})
		}
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].name < profiles[j].name })

	names := map[string]string{}
	for _, p := range profiles {
		if _, taken := names[p.id]; !taken {
			names[p.id] = p.name
		}
	}
	return names, nil
}

// profileName returns the profile name of an AWS config file section: "default" for the bare
// [default] section, and the name after "profile " for named profiles.
func profileName(section string) (string, bool) {
	switch {
	case section == "default":
		return "default", true
	case strings.HasPrefix(section, "profile "):
		return strings.TrimPrefix(section, "profile "), true
	}
	return "", false
}

// sectionAccountID returns the account ID a profile section points to, or "" when it names none.
func sectionAccountID(section *ini.Section) string {
	for _, key := range accountIDKeys {
		if !section.HasKey(key) {
			continue
		}
		value := strings.TrimSpace(section.Key(key).String())
		if key == "role_arn" {
			value = arnAccountID(value)
		}
		if isAccountID(value) {
			return value
		}
	}
	return ""
}

// arnAccountID returns the account field of an ARN (arn:partition:service:region:account:resource),
// or "" when s is not an ARN.
func arnAccountID(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) < 6 || parts[0] != "arn" {
		return ""
	}
	return parts[4]
}

// isAccountID reports whether s is a 12-digit AWS account ID.
func isAccountID(s string) bool {
	return len(s) == accountIDLength && isDigits(s)
}

// isDigits reports whether s is a non-empty string of ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ConfiguredAccountNames reads the accounts map of the awss config file: account ID to name.
//
// raw is the value as Viper returns it. An ID written without quotes is read by YAML as a number
// and loses its leading zeros; account IDs always have 12 digits, so a shorter run of digits is
// left-padded with zeros, which restores it. An entry that is not an account ID, or has no name,
// is skipped with a warning. The warnings are returned, not printed: a bad entry never fails a run.
func ConfiguredAccountNames(raw interface{}) (names map[string]string, warnings []string) {
	names = map[string]string{}
	if raw == nil {
		return names, nil
	}
	entries, ok := raw.(map[string]interface{})
	if !ok {
		return names, []string{"accounts: expected a map of account ID to name; ignored"}
	}

	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	warnings = []string{}
	for _, key := range keys {
		id, ok := normalizeAccountID(key)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("accounts: %q is not a 12-digit account ID; ignored", key))
			continue
		}
		name := ""
		if entries[key] != nil {
			name = strings.TrimSpace(fmt.Sprint(entries[key]))
		}
		switch {
		case name == "":
			warnings = append(warnings, fmt.Sprintf("accounts: %s has no name; ignored", id))
		case names[id] != "":
			warnings = append(warnings, fmt.Sprintf("accounts: %s is listed twice; keeping %q", id, names[id]))
		default:
			names[id] = name
		}
	}
	return names, warnings
}

// normalizeAccountID returns key as a 12-digit account ID, restoring the leading zeros YAML drops
// from an unquoted number. It reports false when key is not 1 to 12 digits.
func normalizeAccountID(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if !isDigits(key) || len(key) > accountIDLength {
		return "", false
	}
	return strings.Repeat("0", accountIDLength-len(key)) + key, true
}
