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

package org

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/smithy-go"
)

// fakeOrg is a fake Organizations client: it returns one page per call and records the inputs.
type fakeOrg struct {
	pages  [][]types.Account
	err    error
	inputs []*organizations.ListAccountsInput
}

func (f *fakeOrg) ListAccounts(
	_ context.Context, in *organizations.ListAccountsInput, _ ...func(*organizations.Options),
) (*organizations.ListAccountsOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	out := &organizations.ListAccountsOutput{}
	page := len(f.inputs) - 1
	if page < len(f.pages) {
		out.Accounts = f.pages[page]
	}
	if page < len(f.pages)-1 {
		out.NextToken = aws.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

// account returns an active account that joined on 2024-03-01.
func account(id, name string) types.Account {
	return types.Account{
		Id: aws.String(id), Name: aws.String(name), Email: aws.String(name + "@example.com"),
		State: types.AccountStateActive, JoinedTimestamp: aws.Time(time.Date(2024, 3, 1, 23, 0, 0, 0, time.UTC)),
	}
}

// accessDenied returns the error Organizations sends to a caller without the permission.
func accessDenied() error {
	return &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized"}
}

// mockClient makes newClient return client, or err, for the duration of the test.
func mockClient(t *testing.T, client organizations.ListAccountsAPIClient, err error) {
	t.Helper()
	old := newClient
	t.Cleanup(func() { newClient = old })
	newClient = func(string) (organizations.ListAccountsAPIClient, error) { return client, err }
}

// TestResults_collect checks the rows of every page, the sort, and that a failed call is an
// error naming the profile, never an empty list.
func TestResults_collect(t *testing.T) {
	tests := []struct {
		name       string
		client     *fakeOrg
		sortField  string
		wantIDs    []string
		wantErrors []string
	}{
		{
			name: "several pages, sorted by name",
			client: &fakeOrg{pages: [][]types.Account{
				{account("333333333333", "c"), account("111111111111", "a")},
				{account("222222222222", "b")},
			}},
			sortField: "name",
			wantIDs:   []string{"111111111111", "222222222222", "333333333333"},
		},
		{
			name:   "access denied",
			client: &fakeOrg{err: accessDenied()},
			wantErrors: []string{`error with profile "org": listing the accounts of the organization: ` +
				"api error AccessDeniedException"},
		},
		{
			name:       "bad sort field keeps the rows",
			client:     &fakeOrg{pages: [][]types.Account{{account("111111111111", "a")}}},
			sortField:  "nope",
			wantIDs:    []string{"111111111111"},
			wantErrors: []string{"invalid sort field"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("org", Region, tt.sortField)
			r.collect(context.Background(), tt.client)

			var got []string
			for _, row := range r.Data {
				got = append(got, row.ID)
			}
			if !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("collect() IDs = %v, want %v", got, tt.wantIDs)
			}
			if len(r.Errors) != len(tt.wantErrors) {
				t.Fatalf("collect() errors = %q, want %d", r.Errors, len(tt.wantErrors))
			}
			for i, want := range tt.wantErrors {
				if !strings.Contains(r.Errors[i], want) {
					t.Errorf("collect() error %d = %q, want it to contain %q", i, r.Errors[i], want)
				}
			}
		})
	}
}

// TestListAccounts_pageSize checks that every ListAccounts call asks for a page of pageSize.
func TestListAccounts_pageSize(t *testing.T) {
	client := &fakeOrg{pages: [][]types.Account{{account("1", "a")}, {account("2", "b")}}}
	if _, err := listAccounts(context.Background(), client); err != nil {
		t.Fatalf("listAccounts() error = %v", err)
	}
	if len(client.inputs) != 2 {
		t.Fatalf("ListAccounts calls = %d, want 2", len(client.inputs))
	}
	for i, in := range client.inputs {
		if got := aws.ToInt32(in.MaxResults); got != pageSize {
			t.Errorf("call %d MaxResults = %d, want %d", i, got, pageSize)
		}
	}
}

// TestParseAccount checks the row of an account, with nil fields and the deprecated Status.
func TestParseAccount(t *testing.T) {
	tests := []struct {
		name    string
		account types.Account
		want    dataRow
	}{
		{
			name:    "every field",
			account: account("111111111111", "network"),
			want: dataRow{
				ID: "111111111111", Name: "network", Email: "network@example.com", Status: "ACTIVE", Joined: "2024-03-01",
			},
		},
		{name: "nil fields", account: types.Account{}, want: dataRow{}},
		{
			name:    "deprecated Status when State is not set",
			account: types.Account{Id: aws.String("1"), Status: types.AccountStatusSuspended},
			want:    dataRow{ID: "1", Status: "SUSPENDED"},
		},
		{
			name: "joined date in UTC",
			account: types.Account{
				JoinedTimestamp: aws.Time(time.Date(2024, 3, 2, 1, 0, 0, 0, time.FixedZone("CET", 3600*2))),
			},
			want: dataRow{Joined: "2024-03-01"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAccount(&tt.account); got != tt.want {
				t.Errorf("parseAccount() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestGetSortFields checks the sort fields of org.
func TestGetSortFields(t *testing.T) {
	want := []string{"email", "id", "joined", "name", "status"}
	if got := SortFieldNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
	for _, f := range want {
		if _, err := GetSortFields(f); err != nil {
			t.Errorf("GetSortFields(%q) error = %v, want nil", f, err)
		}
	}
	if _, err := GetSortFields("owner"); err == nil {
		t.Error("GetSortFields(owner) error = nil, want invalid sort field")
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("org", Region, "id")
	r.Data = []dataRow{{ID: "1"}, {ID: "2"}}
	r.AddError("e")
	if got := r.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if r.GetProfile() != "org" || r.GetRegion() != Region || r.GetSortField() != "id" {
		t.Errorf("getters = %q %q %q, want org %s id", r.GetProfile(), r.GetRegion(), r.GetSortField(), Region)
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"e"}) {
		t.Errorf("GetErrors() = %q, want [e]", got)
	}
	if got := len(r.GetHeaders()); got != 5 {
		t.Errorf("len(GetHeaders()) = %d, want 5", got)
	}
	if got := len(r.GetRows()); got != 2 {
		t.Errorf("len(GetRows()) = %d, want 2", got)
	}
}

// TestResults_Search checks that Search lists through the client of the profile, and reports a
// client that cannot be built.
func TestResults_Search(t *testing.T) {
	tests := []struct {
		name      string
		client    *fakeOrg
		clientErr error
		wantLen   int
		wantError string
	}{
		{name: "lists the accounts", client: &fakeOrg{pages: [][]types.Account{{account("1", "a")}}}, wantLen: 1},
		{name: "no client", clientErr: errors.New("no profile"), wantError: "error getting aws config: no profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient(t, tt.client, tt.clientErr)
			r := New("org", Region, "")
			r.Search(context.Background())
			if r.Len() != tt.wantLen {
				t.Errorf("Search() rows = %d, want %d", r.Len(), tt.wantLen)
			}
			if got := strings.Join(r.Errors, ";"); got != tt.wantError {
				t.Errorf("Search() errors = %q, want %q", got, tt.wantError)
			}
		})
	}
}

// TestAccountNames checks the ID to name map: accounts without an ID or a name are left out, and
// a failed call is returned as an error.
func TestAccountNames(t *testing.T) {
	tests := []struct {
		name      string
		client    *fakeOrg
		clientErr error
		want      map[string]string
		wantErr   bool
	}{
		{
			name: "names of every page",
			client: &fakeOrg{pages: [][]types.Account{
				{account("111111111111", "network"), {Id: aws.String("222222222222")}},
				{{Name: aws.String("no-id")}, account("333333333333", "shared")},
			}},
			want: map[string]string{"111111111111": "network", "333333333333": "shared"},
		},
		{name: "access denied", client: &fakeOrg{err: accessDenied()}, wantErr: true},
		{name: "no client", clientErr: errors.New("no profile"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient(t, tt.client, tt.clientErr)
			got, err := AccountNames(context.Background(), "org")
			if (err != nil) != tt.wantErr {
				t.Fatalf("AccountNames() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AccountNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewClient checks that a profile missing from the AWS config file is an error before any call.
func TestNewClient(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	if _, err := newClient("awss-test-missing-profile"); err == nil {
		t.Error("newClient(missing profile) error = nil, want an error")
	}
	if _, err := newClient(""); err != nil {
		t.Errorf("newClient(default) error = %v, want nil", err)
	}
}
