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

// Package defaultvpc deletes the default VPC of a profile and region, with the network plumbing
// inside it.
//
// The rule: everything inside the VPC that only exists because of the VPC is deleted; anything
// that runs or holds data blocks the deletion; for what lives outside the VPC (a Transit Gateway,
// a VPN gateway), only the link to the VPC is removed. It is all-or-nothing: with one blocker,
// nothing in the VPC is touched.
//
// Every service that runs in a VPC places a network interface (ENI) in its subnets, so one listing
// of the VPC's ENIs finds every workload.
//
// Without Apply it only reports the plan. It implements the common.Results interface, so the
// search engine runs it over every profile and region in parallel.
package defaultvpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// The statuses of a default VPC.
const (
	// StatusDeletable is a preview with no blocker: --apply deletes what the details list.
	StatusDeletable = "deletable"

	// StatusBlocked is a VPC with at least one blocker, named in the details. Nothing is deleted.
	StatusBlocked = "blocked"

	// StatusDenied is a preview where the role may not delete the VPC.
	StatusDenied = "denied"

	// StatusDeleted is a VPC that --apply deleted.
	StatusDeleted = "deleted"

	// StatusFailed is a VPC whose deletion stopped partway; the details give the step and the
	// error. A new run resumes it.
	StatusFailed = "failed"
)

const (
	// defaultPollInterval is how often a wait checks again whether a deletion finished.
	defaultPollInterval = 5 * time.Second

	// waitLimit bounds one wait, in case --timeout is disabled.
	waitLimit = 20 * time.Minute
)

// Results describes the default VPC of one profile and region: no row when the region has none,
// otherwise one row.
type Results struct {
	common.BaseResults

	// Data holds the default VPC, if any.
	Data []dataRow `json:"data"`

	// Apply deletes the VPC; without it the run only reports the plan.
	Apply bool `json:"-"`

	// pollInterval is how often a wait checks again. Tests make it short.
	pollInterval time.Duration

	// newClients builds the AWS clients of the profile and region. Tests replace it.
	newClients func(profile, region string) (ec2API, stsAPI, error)
}

// dataRow is the default VPC of one profile and region.
type dataRow struct {
	// VpcID is the ID of the default VPC.
	VpcID string `json:"id" header:"VPC ID" sort:"id"`

	// CidrBlock is the primary IPv4 CIDR block of the VPC.
	CidrBlock string `json:"cidr,omitempty" header:"CIDR"`

	// Status is one of the Status constants.
	Status string `json:"status" header:"Status" sort:"status"`

	// Details are the blockers of a blocked VPC, the plan of a deletable or deleted one, or the
	// failed step.
	Details []string `json:"details,omitempty" header:"Details"`
}

// New returns the cleanup of the default VPC of profile and region. apply deletes it; without
// apply the run only reports the plan.
func New(profile, region string, apply bool) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile: profile,
			Region:  region,
			Errors:  []string{},
		},
		Data:         []dataRow{},
		Apply:        apply,
		pollInterval: defaultPollInterval,
		newClients:   awsClients,
	}
}

// awsClients builds the EC2 and STS clients of profile and region.
func awsClients(profile, region string) (ec2API, stsAPI, error) {
	cfg, err := common.AwsConfig(profile, region)
	if err != nil {
		return nil, nil, err
	}
	return ec2.NewFromConfig(cfg), sts.NewFromConfig(cfg), nil
}

// Search finds the default VPC and reports its plan, or deletes it with Apply.
//
// An error listing resources is recorded in the result set; it never stops the run.
func (r *Results) Search(ctx context.Context) {
	ec2Client, stsClient, err := r.newClients(r.Profile, r.Region)
	if err != nil {
		r.AddError(fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	row, err := r.run(ctx, ec2Client, stsClient)
	if err != nil {
		r.AddError(err.Error())
		return
	}
	if row != nil {
		r.Data = append(r.Data, *row)
	}
}

// run returns the row of the default VPC, or nil when the region has none.
func (r *Results) run(ctx context.Context, c ec2API, s stsAPI) (*dataRow, error) {
	vpc, err := defaultVPC(ctx, c)
	if err != nil || vpc == nil {
		return nil, err
	}
	row := &dataRow{VpcID: aws.ToString(vpc.VpcId), CidrBlock: aws.ToString(vpc.CidrBlock)}

	account, err := callerAccount(ctx, s)
	if err != nil {
		return nil, err
	}
	if owner := aws.ToString(vpc.OwnerId); owner != account {
		row.Status = StatusBlocked
		row.Details = []string{fmt.Sprintf("owned by account %s, not by this account (%s)", owner, account)}
		return row, nil
	}

	blockers, owned, err := listBlockers(ctx, c, row.VpcID)
	if err != nil {
		return nil, err
	}
	if len(blockers) > 0 {
		row.Status, row.Details = StatusBlocked, blockers
		return row, nil
	}

	inv, err := listInventory(ctx, c, row.VpcID)
	if err != nil {
		return nil, err
	}
	inv.enis = owned
	row.Details = inv.plan()

	if !r.Apply {
		return r.preview(ctx, c, row)
	}
	// A failed deletion is a row, not an error of the run: the details give the step and the error.
	row.Status = StatusDeleted
	if err := r.apply(ctx, c, inv); err != nil {
		row.Status, row.Details = StatusFailed, []string{err.Error()}
	}
	return row, nil
}

// preview sets the status of a VPC with no blocker: deletable, or denied when a DryRun of
// DeleteVpc says the role may not delete it.
func (r *Results) preview(ctx context.Context, c ec2API, row *dataRow) (*dataRow, error) {
	_, err := c.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String(row.VpcID), DryRun: aws.Bool(true)})
	switch errorCode(err) {
	case "DryRunOperation":
		row.Status = StatusDeletable
	case "UnauthorizedOperation":
		row.Status = StatusDenied
		row.Details = []string{"this role may not delete VPCs (ec2:DeleteVpc); --apply needs a role that can"}
	default:
		return nil, fmt.Errorf("checking the permission to delete %s: %w", row.VpcID, err)
	}
	return row, nil
}

// defaultVPC returns the default VPC of the region, or nil when it has none.
func defaultVPC(ctx context.Context, c ec2API) (*types.Vpc, error) {
	vpcs, err := all(ctx, ec2.NewDescribeVpcsPaginator(c, &ec2.DescribeVpcsInput{
		Filters: filter("is-default", "true"), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeVpcsOutput) []types.Vpc { return o.Vpcs })
	if err != nil {
		return nil, fmt.Errorf("error describing the default vpc: %w", err)
	}
	if len(vpcs) == 0 {
		return nil, nil
	}
	return &vpcs[0], nil
}

// callerAccount returns the account ID of the profile.
func callerAccount(ctx context.Context, s stsAPI) (string, error) {
	out, err := s.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("error getting the account of the profile: %w", err)
	}
	return aws.ToString(out.Account), nil
}

// errorCode returns the AWS error code of err, or "" when err is nil or not an AWS API error.
func errorCode(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}
	return ""
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the dataRow fields.
func (r *Results) GetHeaders() []interface{} { return common.Headers(dataRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} { return common.Rows(r.Data) }

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The search engine needs it. A result set has at most one row, so the command has no --sort.
func GetSortFields(f string) (map[string]string, error) {
	return common.SortFields(dataRow{}, f)
}

// SortFieldNames returns the valid sort fields, sorted alphabetically.
func SortFieldNames() []string {
	return common.SortFieldNames(dataRow{})
}
