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
	"strings"
)

// accountIDLength is the number of digits of an AWS account ID.
const accountIDLength = 12

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
