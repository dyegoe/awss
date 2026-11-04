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

// Package dhcpoptions deletes the DHCP options sets that no VPC uses, in one profile and region.
//
// A set is only ever used by being associated with a VPC, so a set of the account that no VPC
// names is unused, and deleting it changes nothing for anything running. Deleting a VPC, for
// example with the default-vpc cleanup, leaves its set behind.
//
// Without Apply it only reports the unused sets. It implements the common.Results interface, so
// the search engine runs it over every profile and region in parallel.
package dhcpoptions

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/dyegoe/awss/cleanup"
	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// pageSize is the page size of the two listings: 1000 is the largest AWS allows. AWS recommends
// paginated calls only, and the SDK paginator sets no page size by itself.
const pageSize int32 = 1000

// ec2API is the part of the EC2 client the cleanup uses. Tests pass a fake.
type ec2API interface {
	ec2.DescribeDhcpOptionsAPIClient
	ec2.DescribeVpcsAPIClient
	DeleteDhcpOptions(context.Context, *ec2.DeleteDhcpOptionsInput, ...func(*ec2.Options)) (
		*ec2.DeleteDhcpOptionsOutput, error)
}

// Results describes the unused DHCP options sets of one profile and region.
type Results struct {
	common.BaseResults

	// Data holds one row per unused set.
	Data []dataRow `json:"data"`

	// Apply deletes the sets; without it the run only reports them.
	Apply bool `json:"-"`

	// newClients builds the AWS clients of the profile and region. Tests replace it.
	newClients func(profile, region string) (ec2API, cleanup.STSAPI, error)
}

// dataRow is one unused DHCP options set.
type dataRow struct {
	// ID is the ID of the set.
	ID string `json:"id" header:"ID" sort:"id"`

	// Name is the tag:Name of the set.
	Name string `json:"name,omitempty" header:"Name" sort:"name"`

	// Options are the configuration of the set, by key, for JSON.
	Options map[string][]string `json:"options,omitempty"`

	// OptionLines are the configuration of the set, one "key: values" per line, for the table.
	OptionLines []string `json:"-" header:"Options"`

	// Status is one of the cleanup statuses.
	Status string `json:"status" header:"Status" sort:"status"`

	// Details say why a set is denied or failed.
	Details []string `json:"details,omitempty" header:"Details"`

	// Tags are the tags of the set.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New returns the cleanup of the unused DHCP options sets of profile and region. apply deletes
// them; without apply the run only reports them.
func New(profile, region string, apply bool) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile: profile,
			Region:  region,
			Errors:  []string{},
		},
		Data:       []dataRow{},
		Apply:      apply,
		newClients: awsClients,
	}
}

// awsClients builds the EC2 and STS clients of profile and region.
func awsClients(profile, region string) (ec2API, cleanup.STSAPI, error) {
	cfg, err := common.AwsConfig(profile, region)
	if err != nil {
		return nil, nil, err
	}
	return ec2.NewFromConfig(cfg), sts.NewFromConfig(cfg), nil
}

// Search finds the unused DHCP options sets and reports them, or deletes them with Apply.
//
// An error listing them is recorded in the result set; it never stops the run.
func (r *Results) Search(ctx context.Context) {
	ec2Client, stsClient, err := r.newClients(r.Profile, r.Region)
	if err != nil {
		r.AddError(fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	if err := r.run(ctx, ec2Client, stsClient); err != nil {
		r.AddError(err.Error())
	}
}

// run fills Data with the unused sets of the account, each with its status.
func (r *Results) run(ctx context.Context, c ec2API, s cleanup.STSAPI) error {
	account, err := cleanup.CallerAccount(ctx, s)
	if err != nil {
		return err
	}
	unused, err := unusedSets(ctx, c, account)
	if err != nil {
		return err
	}
	for i := range unused {
		row := parseSet(&unused[i])
		if r.Apply {
			r.delete(ctx, c, &row)
		} else if err := r.preview(ctx, c, &row); err != nil {
			return err
		}
		r.Data = append(r.Data, row)
	}
	common.SortByField(r.Data, "ID")
	return nil
}

// unusedSets returns the DHCP options sets of account that no VPC of the region uses.
//
// A participant of a shared VPC sees that VPC, but the VPC uses its owner's set, so only the sets
// of account are candidates.
func unusedSets(ctx context.Context, c ec2API, account string) ([]types.DhcpOptions, error) {
	var sets []types.DhcpOptions
	p := ec2.NewDescribeDhcpOptionsPaginator(c, &ec2.DescribeDhcpOptionsInput{
		Filters:    []types.Filter{{Name: aws.String("owner-id"), Values: []string{account}}},
		MaxResults: aws.Int32(pageSize),
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error describing the dhcp options sets: %w", err)
		}
		sets = append(sets, page.DhcpOptions...)
	}

	used, err := usedSets(ctx, c)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(sets, func(set types.DhcpOptions) bool {
		return used[aws.ToString(set.DhcpOptionsId)] || aws.ToString(set.OwnerId) != account
	}), nil
}

// usedSets returns the IDs of the DHCP options sets the VPCs of the region use.
func usedSets(ctx context.Context, c ec2API) (map[string]bool, error) {
	used := map[string]bool{}
	p := ec2.NewDescribeVpcsPaginator(c, &ec2.DescribeVpcsInput{MaxResults: aws.Int32(pageSize)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error describing the vpcs: %w", err)
		}
		for i := range page.Vpcs {
			used[aws.ToString(page.Vpcs[i].DhcpOptionsId)] = true
		}
	}
	return used, nil
}

// parseSet converts a DHCP options set into a row with no status yet.
func parseSet(set *types.DhcpOptions) dataRow {
	row := dataRow{
		ID:      aws.ToString(set.DhcpOptionsId),
		Name:    common.TagName(set.Tags),
		Options: map[string][]string{},
		Tags:    common.TagsToMap(set.Tags),
	}
	for _, conf := range set.DhcpConfigurations {
		key := aws.ToString(conf.Key)
		for _, v := range conf.Values {
			row.Options[key] = append(row.Options[key], aws.ToString(v.Value))
		}
	}
	keys := make([]string, 0, len(row.Options))
	for k := range row.Options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		row.OptionLines = append(row.OptionLines, k+": "+strings.Join(row.Options[k], ", "))
	}
	return row
}

// preview sets the status of an unused set: deletable, or denied when a DryRun of
// DeleteDhcpOptions says the role may not delete it.
func (r *Results) preview(ctx context.Context, c ec2API, row *dataRow) error {
	_, err := c.DeleteDhcpOptions(ctx,
		&ec2.DeleteDhcpOptionsInput{DhcpOptionsId: aws.String(row.ID), DryRun: aws.Bool(true)})
	allowed, err := cleanup.DryRunAllowed(err)
	if err != nil {
		return fmt.Errorf("checking the permission to delete %s: %w", row.ID, err)
	}
	row.Status = cleanup.StatusDeletable
	if !allowed {
		row.Status = cleanup.StatusDenied
		row.Details = []string{"this role may not delete DHCP options sets (ec2:DeleteDhcpOptions)"}
	}
	return nil
}

// delete deletes an unused set. A set already gone counts as deleted; any other error, such as
// a VPC that started to use the set during the run, makes the row failed.
func (r *Results) delete(ctx context.Context, c ec2API, row *dataRow) {
	_, err := c.DeleteDhcpOptions(ctx, &ec2.DeleteDhcpOptionsInput{DhcpOptionsId: aws.String(row.ID)})
	if err := cleanup.IgnoreNotFound(err); err != nil {
		row.Status, row.Details = cleanup.StatusFailed, []string{"error: " + cleanup.ConciseError(err)}
		return
	}
	row.Status = cleanup.StatusDeleted
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the dataRow fields.
func (r *Results) GetHeaders() []interface{} { return common.Headers(dataRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} { return common.Rows(r.Data) }

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The search engine needs it. The rows are always sorted by ID, so the command has no --sort.
func GetSortFields(f string) (map[string]string, error) {
	return common.SortFields(dataRow{}, f)
}

// SortFieldNames returns the valid sort fields, sorted alphabetically.
func SortFieldNames() []string {
	return common.SortFieldNames(dataRow{})
}
