/*
Copyright © 2026 Dyego Alexandre Eugenio github@dyego.com.br

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
	"slices"
	"strings"
	"unicode"

	"github.com/aws/smithy-go"
)

// DescribeByIDs runs describe with ids and, while AWS answers that some of them do not exist,
// runs it again without those IDs.
//
// Searching by ID across profiles and regions is a lookup: an ID that is not in this account or
// region is not a failure. AWS rejects the whole call with notFoundCode (for example
// "InvalidInstanceID.NotFound") when one ID is missing, so the IDs it names are dropped and the
// rest are asked again. When every ID is missing, describe is not called again and the result
// is empty, with no error.
//
// Any other error, or a not-found error that names none of ids, is returned unchanged. AWS checks
// the IDs before it returns the first page, so describe has kept nothing when it fails this way.
// With no ids, describe runs once.
func DescribeByIDs(ids []string, notFoundCode string, describe func(ids []string) error) error {
	for {
		err := describe(ids)
		if err == nil {
			return nil
		}
		missing := MissingIDs(err, notFoundCode, ids)
		if len(missing) == 0 {
			return err
		}
		ids = slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return slices.Contains(missing, id) })
		if len(ids) == 0 {
			return nil
		}
	}
}

// MissingIDs returns the IDs of ids that err reports as missing.
//
// err must be an AWS API error with the code notFoundCode; otherwise the result is empty. The
// IDs are read from the error message, such as "The instance IDs 'i-1, i-2' do not exist". When
// the message names none of ids and only one ID was asked, that ID is the missing one.
func MissingIDs(err error, notFoundCode string, ids []string) []string {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != notFoundCode || len(ids) == 0 {
		return nil
	}

	words := strings.FieldsFunc(apiErr.ErrorMessage(), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
	var missing []string
	for _, id := range ids {
		if slices.Contains(words, id) && !slices.Contains(missing, id) {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 && len(ids) == 1 {
		missing = ids
	}
	return missing
}
