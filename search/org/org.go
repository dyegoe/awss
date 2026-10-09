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

// Package org contains the listing of the accounts of the AWS Organization.
//
// It implements the common.Results interface. Organizations is a global service: one call lists
// the whole organization, so the search runs for one profile and no region.
package org

import (
	"context"
	"fmt"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"
)

// Region is the region of the org result set: Organizations is global, so --regions does not apply.
const Region = "global"

// pageSize is the number of accounts asked per ListAccounts call, its maximum.
//
// The paginator sets no page size by itself; asking for one follows the paginated-calls rule of
// the EC2 Describe* calls.
const pageSize int32 = 20

// joinedLayout formats the date an account joined the organization: it sorts as text.
const joinedLayout = "2006-01-02"

// Results describes the accounts of the organization.
type Results struct {
	common.BaseResults

	// Data contains the accounts found.
	Data []dataRow `json:"data"`
}

// dataRow represents one account of the organization.
type dataRow struct {
	// ID is the account ID.
	ID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Name is the account name.
	Name string `json:"name,omitempty" header:"Name" sort:"name"`

	// Email is the email address of the account's root user.
	Email string `json:"email,omitempty" header:"Email" sort:"email"`

	// Status is the state of the account in the organization, such as ACTIVE or SUSPENDED.
	Status string `json:"status,omitempty" header:"Status" sort:"status"`

	// Joined is the date the account joined the organization, as YYYY-MM-DD in UTC.
	Joined string `json:"joined,omitempty" header:"Joined" sort:"joined"`
}

// New initiates and returns a new instance of org results. region is kept for the output only:
// the search always calls the global endpoint.
func New(profile, region, sortField string) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data: []dataRow{},
	}
}

// newClient builds the Organizations client of a profile. It is a variable so tests can replace
// the AWS config with a fake client.
var newClient = func(profile string) (organizations.ListAccountsAPIClient, error) {
	// The global endpoint of the aws partition is served from us-east-1.
	cfg, err := common.AwsConfig(profile, common.DefaultRegion)
	if err != nil {
		return nil, err
	}
	return organizations.NewFromConfig(cfg), nil
}

// Search lists the accounts of the organization. A missing permission, or an account that is
// not in an organization, is an error in the result set, never an empty list.
func (r *Results) Search(ctx context.Context) {
	client, err := newClient(r.Profile)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	r.collect(ctx, client)
}

// collect lists the accounts with client and sorts the rows.
func (r *Results) collect(ctx context.Context, client organizations.ListAccountsAPIClient) {
	accounts, err := listAccounts(ctx, client)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error with profile %q: %v", r.Profile, err))
		return
	}
	for i := range accounts {
		r.Data = append(r.Data, parseAccount(&accounts[i]))
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// listAccounts returns every account of the organization, following every page.
func listAccounts(ctx context.Context, client organizations.ListAccountsAPIClient) ([]types.Account, error) {
	paginator := organizations.NewListAccountsPaginator(client, &organizations.ListAccountsInput{
		MaxResults: aws.Int32(pageSize),
	})
	var accounts []types.Account
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing the accounts of the organization: %w", err)
		}
		accounts = append(accounts, page.Accounts...)
	}
	return accounts, nil
}

// AccountNames returns the names of the organization's accounts, keyed by account ID, listed with
// profile. Accounts without an ID or a name are left out.
func AccountNames(ctx context.Context, profile string) (map[string]string, error) {
	client, err := newClient(profile)
	if err != nil {
		return nil, err
	}
	return accountNames(ctx, client)
}

// accountNames returns the names of the accounts listed with client, keyed by account ID.
func accountNames(ctx context.Context, client organizations.ListAccountsAPIClient) (map[string]string, error) {
	accounts, err := listAccounts(ctx, client)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(accounts))
	for i := range accounts {
		id, name := aws.ToString(accounts[i].Id), aws.ToString(accounts[i].Name)
		if id != "" && name != "" {
			names[id] = name
		}
	}
	return names, nil
}

// parseAccount converts an account into a dataRow. Status reads State, and the deprecated
// Status only when State is not set.
func parseAccount(a *types.Account) dataRow {
	row := dataRow{
		ID:     aws.ToString(a.Id),
		Name:   aws.ToString(a.Name),
		Email:  aws.ToString(a.Email),
		Status: string(a.State),
	}
	if row.Status == "" {
		row.Status = string(a.Status)
	}
	if a.JoinedTimestamp != nil {
		row.Joined = a.JoinedTimestamp.UTC().Format(joinedLayout)
	}
	return row
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the dataRow fields.
func (r *Results) GetHeaders() []interface{} { return common.Headers(dataRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} { return common.Rows(r.Data) }

// sortResults sorts the results by the given field.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}
	common.SortByField(r.Data, sortFields[field])
	return nil
}

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The sort fields are defined in the struct tag `sort` on dataRow.
// The function returns an error if the given field is not a valid sort field.
func GetSortFields(f string) (map[string]string, error) {
	return common.SortFields(dataRow{}, f)
}

// SortFieldNames returns the valid sort fields, sorted alphabetically.
func SortFieldNames() []string {
	return common.SortFieldNames(dataRow{})
}
