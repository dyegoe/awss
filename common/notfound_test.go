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
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/aws/smithy-go"
)

// notFound returns the AWS error for codeInstanceNotFound naming ids, as EC2 words it.
func notFound(ids string) error {
	return &smithy.GenericAPIError{Code: codeInstanceNotFound, Message: "The instance IDs '" + ids + "' do not exist"}
}

// codeInstanceNotFound is the error code the tests of this file use.
const codeInstanceNotFound = "InvalidInstanceID.NotFound"

// TestMissingIDs tests which asked IDs are read from a not-found error.
func TestMissingIDs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		ids  []string
		want []string
	}{
		{name: "one of several", err: notFound("i-2"), ids: []string{"i-1", "i-2"}, want: []string{"i-2"}},
		{
			name: "several, wrapped", err: fmt.Errorf("describing: %w", notFound("i-3, i-1")),
			ids: []string{"i-1", "i-2", "i-3"}, want: []string{"i-1", "i-3"},
		},
		{
			name: "singular message",
			err: &smithy.GenericAPIError{
				Code: codeInstanceNotFound, Message: "The instance ID 'i-1' does not exist",
			},
			ids: []string{"i-1"}, want: []string{"i-1"},
		},
		{name: "an ID is not a prefix match", err: notFound("i-12"), ids: []string{"i-1", "i-2"}, want: nil},
		{name: "one ID, message names none", err: notFound("?"), ids: []string{"i-1"}, want: []string{"i-1"}},
		{name: "several IDs, message names none", err: notFound("?"), ids: []string{"i-1", "i-2"}, want: nil},
		{name: "duplicate asked ID", err: notFound("i-1"), ids: []string{"i-1", "i-1", "i-2"}, want: []string{"i-1"}},
		{name: "no IDs asked", err: notFound("i-1"), ids: nil, want: nil},
		{
			name: "other code",
			err:  &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "i-1"},
			ids:  []string{"i-1"}, want: nil,
		},
		{name: "not an API error", err: errors.New("i-1"), ids: []string{"i-1"}, want: nil},
		{name: "nil error", err: nil, ids: []string{"i-1"}, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MissingIDs(tt.err, codeInstanceNotFound, tt.ids)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MissingIDs(%v, %v) = %v, want %v", tt.err, tt.ids, got, tt.want)
			}
		})
	}
}

// awsLike returns a describe function that, like AWS, rejects a call naming any ID not in exist
// with a not-found error naming them, and records the IDs of every call in asked.
func awsLike(exist []string, asked *[][]string) func([]string) error {
	return func(ids []string) error {
		*asked = append(*asked, ids)
		var missing []string
		for _, id := range ids {
			if !slices.Contains(exist, id) {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			return notFound(fmt.Sprint(missing))
		}
		return nil
	}
}

// TestDescribeByIDs tests that missing IDs are dropped and asked again, and that other errors
// are returned.
func TestDescribeByIDs(t *testing.T) {
	denied := errors.New("denied")
	tests := []struct {
		name      string
		ids       []string
		exist     []string
		failWith  error
		wantAsked [][]string
		wantErr   error
	}{
		{
			name: "all found", ids: []string{"i-1", "i-2"}, exist: []string{"i-1", "i-2"},
			wantAsked: [][]string{{"i-1", "i-2"}},
		},
		{
			name: "some missing are dropped", ids: []string{"i-1", "i-2", "i-3"}, exist: []string{"i-2"},
			wantAsked: [][]string{{"i-1", "i-2", "i-3"}, {"i-2"}},
		},
		{name: "all missing is empty, not an error", ids: []string{"i-1", "i-2"}, wantAsked: [][]string{{"i-1", "i-2"}}},
		{name: "no IDs runs once", ids: nil, wantAsked: [][]string{nil}},
		{
			name: "other error is returned", ids: []string{"i-1"}, failWith: denied,
			wantAsked: [][]string{{"i-1"}}, wantErr: denied,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var asked [][]string
			describe := awsLike(tt.exist, &asked)
			if tt.failWith != nil {
				describe = func(ids []string) error { asked = append(asked, ids); return tt.failWith }
			}

			err := DescribeByIDs(tt.ids, codeInstanceNotFound, describe)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("DescribeByIDs(%v) error = %v, want %v", tt.ids, err, tt.wantErr)
			}
			if !reflect.DeepEqual(asked, tt.wantAsked) {
				t.Errorf("DescribeByIDs(%v) asked %v, want %v", tt.ids, asked, tt.wantAsked)
			}
		})
	}
}

// TestDescribeByIDs_keepsCallerIDs tests that dropping missing IDs does not change the caller's slice.
func TestDescribeByIDs_keepsCallerIDs(t *testing.T) {
	ids := []string{"i-1", "i-2"}
	var asked [][]string

	if err := DescribeByIDs(ids, codeInstanceNotFound, awsLike([]string{"i-2"}, &asked)); err != nil {
		t.Fatalf("DescribeByIDs() error = %v", err)
	}
	if want := []string{"i-1", "i-2"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("caller IDs = %v, want %v", ids, want)
	}
}
