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

package dhcpoptions

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dyegoe/awss/cleanup"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

const testAccount = "111111111111"

// fakeEC2 is an ec2API that serves pages of DHCP options sets and of VPCs, and records the
// deletions.
type fakeEC2 struct {
	setPages [][]types.DhcpOptions
	vpcPages [][]types.Vpc
	// errs fails the call of that name.
	errs map[string]error
	// deleteErrs fails the deletion of that set.
	deleteErrs map[string]error
	// dryRunCode is the error code of a DryRun deletion; "" means DryRunOperation.
	dryRunCode string

	deleted   []string
	pageSizes map[string][]int32
}

// page returns page i of pages, and the next token.
func page[T any](pages [][]T, token *string) (items []T, next *string) {
	i := 0
	if token != nil {
		i = len(*token)
	}
	if i >= len(pages) {
		return nil, nil
	}
	if i+1 < len(pages) {
		next = aws.String(strings.Repeat("x", i+1))
	}
	return pages[i], next
}

// record records the page size of a call and returns its injected error.
func (f *fakeEC2) record(name string, size *int32) error {
	if f.pageSizes == nil {
		f.pageSizes = map[string][]int32{}
	}
	f.pageSizes[name] = append(f.pageSizes[name], aws.ToInt32(size))
	return f.errs[name]
}

// DescribeDhcpOptions returns one page of sets.
func (f *fakeEC2) DescribeDhcpOptions(_ context.Context, in *ec2.DescribeDhcpOptionsInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeDhcpOptionsOutput, error,
) {
	if err := f.record("DescribeDhcpOptions", in.MaxResults); err != nil {
		return nil, err
	}
	items, next := page(f.setPages, in.NextToken)
	return &ec2.DescribeDhcpOptionsOutput{DhcpOptions: items, NextToken: next}, nil
}

// DescribeVpcs returns one page of VPCs.
func (f *fakeEC2) DescribeVpcs(_ context.Context, in *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeVpcsOutput, error,
) {
	if err := f.record("DescribeVpcs", in.MaxResults); err != nil {
		return nil, err
	}
	items, next := page(f.vpcPages, in.NextToken)
	return &ec2.DescribeVpcsOutput{Vpcs: items, NextToken: next}, nil
}

// DeleteDhcpOptions answers a DryRun with dryRunCode, or deletes the set.
func (f *fakeEC2) DeleteDhcpOptions(_ context.Context, in *ec2.DeleteDhcpOptionsInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteDhcpOptionsOutput, error,
) {
	if aws.ToBool(in.DryRun) {
		code := f.dryRunCode
		if code == "" {
			code = "DryRunOperation"
		}
		return nil, apiErr(code)
	}
	id := aws.ToString(in.DhcpOptionsId)
	if err := f.deleteErrs[id]; err != nil {
		return nil, err
	}
	f.deleted = append(f.deleted, id)
	return &ec2.DeleteDhcpOptionsOutput{}, nil
}

// fakeSTS is an STSAPI that answers testAccount, or err.
type fakeSTS struct{ err error }

// GetCallerIdentity returns the test account.
func (f *fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (
	*sts.GetCallerIdentityOutput, error,
) {
	if f.err != nil {
		return nil, f.err
	}
	return &sts.GetCallerIdentityOutput{Account: aws.String(testAccount)}, nil
}

// apiErr returns an AWS API error with code.
func apiErr(code string) error {
	return &smithy.GenericAPIError{Code: code, Message: code + " message"}
}

// set returns a DHCP options set of owner with the AWS default options of eu-west-1.
func set(id, owner string) types.DhcpOptions {
	return types.DhcpOptions{
		DhcpOptionsId: aws.String(id), OwnerId: aws.String(owner),
		DhcpConfigurations: []types.DhcpConfiguration{
			{Key: aws.String("domain-name"), Values: []types.AttributeValue{{Value: aws.String("eu-west-1.compute.internal")}}},
			{Key: aws.String("domain-name-servers"), Values: []types.AttributeValue{{Value: aws.String("AmazonProvidedDNS")}}},
		},
	}
}

// vpc returns a VPC that uses the set dopt.
func vpc(dopt string) types.Vpc {
	return types.Vpc{VpcId: aws.String("vpc-" + dopt), DhcpOptionsId: aws.String(dopt)}
}

// region returns a fake region: dopt-used is used by a VPC on the second page, dopt-b and dopt-a
// are unused (on two pages), and dopt-shared belongs to another account.
func region() *fakeEC2 {
	return &fakeEC2{
		setPages: [][]types.DhcpOptions{
			{set("dopt-b", testAccount), set("dopt-used", testAccount)},
			{set("dopt-a", testAccount), set("dopt-shared", "999999999999")},
		},
		vpcPages: [][]types.Vpc{{vpc("default")}, {vpc("dopt-used"), vpc("dopt-shared")}},
	}
}

// runTest runs the cleanup against f.
func runTest(t *testing.T, f *fakeEC2, apply bool) (*Results, error) {
	t.Helper()
	r := New("p", "eu-west-1", apply)
	err := r.run(context.Background(), f, &fakeSTS{})
	return r, err
}

// ids returns the IDs and statuses of the rows of r.
func ids(r *Results) []string {
	var out []string
	for _, row := range r.Data {
		out = append(out, row.ID+" "+row.Status)
	}
	return out
}

// TestNew checks the fields New sets.
func TestNew(t *testing.T) {
	r := New("p", "eu-west-1", true)
	if r.Profile != "p" || r.Region != "eu-west-1" || !r.Apply || r.Data == nil || r.Errors == nil || r.newClients == nil {
		t.Errorf("New(p, eu-west-1, true) = %+v", r)
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("p", "eu-west-1", false)
	r.Data = append(r.Data, dataRow{ID: "dopt-1"})
	r.AddError("boom")
	if r.Len() != 1 || r.GetProfile() != "p" || r.GetRegion() != "eu-west-1" || r.GetSortField() != "" {
		t.Errorf("accessors = %d %q %q %q", r.Len(), r.GetProfile(), r.GetRegion(), r.GetSortField())
	}
	if got := r.GetErrors(); !reflect.DeepEqual(got, []string{"boom"}) {
		t.Errorf("GetErrors() = %v, want [boom]", got)
	}
}

// TestResults_GetHeaders checks the table headers.
func TestResults_GetHeaders(t *testing.T) {
	got := New("p", "r", false).GetHeaders()
	want := []interface{}{"ID", "Name", "Options", "Status", "Details", "Tags"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetHeaders() = %v, want %v", got, want)
	}
}

// TestResults_GetRows checks that every row is returned.
func TestResults_GetRows(t *testing.T) {
	r := New("p", "r", false)
	r.Data = []dataRow{{ID: "dopt-1"}}
	if got := r.GetRows(); len(got) != 1 || got[0].(dataRow).ID != "dopt-1" {
		t.Errorf("GetRows() = %v", got)
	}
}

// TestGetSortFields checks a valid and an invalid sort field, and the sort field names.
func TestGetSortFields(t *testing.T) {
	if _, err := GetSortFields("id"); err != nil {
		t.Errorf("GetSortFields(id) error = %v", err)
	}
	if _, err := GetSortFields("nope"); err == nil {
		t.Error("GetSortFields(nope) error = nil, want an error")
	}
	if got, want := SortFieldNames(), []string{"id", "name", "status"}; !reflect.DeepEqual(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
}

// TestResults_run_preview checks that a preview lists the unused sets of the account only, sorted,
// pages both listings with a page size, and deletes nothing.
func TestResults_run_preview(t *testing.T) {
	f := region()
	r, err := runTest(t, f, false)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := ids(r), []string{"dopt-a deletable", "dopt-b deletable"}; !reflect.DeepEqual(got, want) {
		t.Errorf("run() rows = %v, want %v", got, want)
	}
	if len(f.deleted) != 0 {
		t.Errorf("run() deleted %v, want nothing", f.deleted)
	}
	for name, sizes := range f.pageSizes {
		if len(sizes) != 2 || sizes[0] != pageSize || sizes[1] != pageSize {
			t.Errorf("%s page sizes = %v, want two pages of %d", name, sizes, pageSize)
		}
	}
}

// TestResults_run_previewPermission checks the DryRun permission check of a preview.
func TestResults_run_previewPermission(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		wantStatus string
		wantErr    bool
	}{
		{name: "allowed", code: "DryRunOperation", wantStatus: cleanup.StatusDeletable},
		{name: "denied", code: "UnauthorizedOperation", wantStatus: cleanup.StatusDenied},
		{name: "other error", code: "RequestLimitExceeded", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := region()
			f.dryRunCode = tt.code
			r, err := runTest(t, f, false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("run() error = %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && r.Data[0].Status != tt.wantStatus {
				t.Errorf("run() status = %s, want %s", r.Data[0].Status, tt.wantStatus)
			}
		})
	}
}

// TestResults_run_apply checks that apply deletes the unused sets, counts a set already gone as
// deleted, and reports any other error as failed with the AWS code.
func TestResults_run_apply(t *testing.T) {
	f := region()
	f.deleteErrs = map[string]error{"dopt-a": apiErr("DependencyViolation")}
	r, err := runTest(t, f, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := ids(r), []string{"dopt-a failed", "dopt-b deleted"}; !reflect.DeepEqual(got, want) {
		t.Errorf("run() rows = %v, want %v", got, want)
	}
	if want := []string{"error: api error DependencyViolation: DependencyViolation message"}; !reflect.DeepEqual(
		r.Data[0].Details, want) {
		t.Errorf("run() details = %v, want %v", r.Data[0].Details, want)
	}

	f = region()
	f.deleteErrs = map[string]error{"dopt-a": apiErr("InvalidDhcpOptionID.NotFound")}
	if r, err = runTest(t, f, true); err != nil || r.Data[0].Status != cleanup.StatusDeleted {
		t.Errorf("run() with a set already gone = %v, %v, want deleted", ids(r), err)
	}
}

// TestResults_run_errors checks that a listing error is returned.
func TestResults_run_errors(t *testing.T) {
	tests := []struct {
		name string
		call string
		sts  error
	}{
		{name: "account", sts: errors.New("expired")},
		{name: "sets", call: "DescribeDhcpOptions"},
		{name: "vpcs", call: "DescribeVpcs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := region()
			f.errs = map[string]error{tt.call: errors.New("boom")}
			if err := New("p", "r", false).run(context.Background(), f, &fakeSTS{err: tt.sts}); err == nil {
				t.Error("run() error = nil, want an error")
			}
		})
	}
}

// TestResults_Search checks that Search records a client error and a listing error, and keeps
// the rows.
func TestResults_Search(t *testing.T) {
	tests := []struct {
		name       string
		clientErr  error
		fake       *fakeEC2
		wantRows   int
		wantErrors int
	}{
		{name: "client error", clientErr: errors.New("no profile"), fake: region(), wantErrors: 1},
		{name: "listing error", fake: &fakeEC2{errs: map[string]error{"DescribeVpcs": errors.New("boom")}}, wantErrors: 1},
		{name: "rows", fake: region(), wantRows: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("p", "r", false)
			r.newClients = func(string, string) (ec2API, cleanup.STSAPI, error) { return tt.fake, &fakeSTS{}, tt.clientErr }
			r.Search(context.Background())
			if r.Len() != tt.wantRows || len(r.Errors) != tt.wantErrors {
				t.Errorf("Search() rows = %d errors = %v, want %d rows %d errors", r.Len(), r.Errors, tt.wantRows, tt.wantErrors)
			}
		})
	}
}

// TestParseSet checks the name, the tags, and the options by key and as sorted lines, with nil
// fields.
func TestParseSet(t *testing.T) {
	s := set("dopt-1", testAccount)
	s.Tags = []types.Tag{{Key: aws.String("Name"), Value: aws.String("corp")}}
	s.DhcpConfigurations = append(s.DhcpConfigurations,
		types.DhcpConfiguration{Key: aws.String("ntp-servers"), Values: []types.AttributeValue{
			{Value: aws.String("10.0.0.1")}, {Value: aws.String("10.0.0.2")}}},
		types.DhcpConfiguration{})
	got := parseSet(&s)
	// The empty configuration has no value, so it has no line.
	wantLines := []string{
		"domain-name: eu-west-1.compute.internal", "domain-name-servers: AmazonProvidedDNS",
		"ntp-servers: 10.0.0.1, 10.0.0.2",
	}
	if got.ID != "dopt-1" || got.Name != "corp" || got.Tags["Name"] != "corp" {
		t.Errorf("parseSet() = %+v", got)
	}
	if !reflect.DeepEqual(got.OptionLines, wantLines) {
		t.Errorf("parseSet() lines = %q, want %q", got.OptionLines, wantLines)
	}
	if !reflect.DeepEqual(got.Options["ntp-servers"], []string{"10.0.0.1", "10.0.0.2"}) {
		t.Errorf("parseSet() options = %v", got.Options)
	}
}
