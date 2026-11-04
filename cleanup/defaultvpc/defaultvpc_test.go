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

package defaultvpc

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dyegoe/awss/cleanup"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// newTest returns the cleanup of profile p in eu-west-1, polling every millisecond.
func newTest(apply bool) *Results {
	r := New("p", "eu-west-1", apply)
	r.pollInterval = time.Millisecond
	return r
}

// runTest runs the cleanup against f and the test account.
func runTest(t *testing.T, f *fakeEC2, apply bool) (*dataRow, error) {
	t.Helper()
	return newTest(apply).run(context.Background(), f, &fakeSTS{account: testAccount})
}

// deletes returns the calls of f that change resources.
func deletes(f *fakeEC2) []string { return f.calls }

// TestNew checks the fields New sets.
func TestNew(t *testing.T) {
	r := New("p", "eu-west-1", true)
	if r.Profile != "p" || r.Region != "eu-west-1" || !r.Apply || r.pollInterval != defaultPollInterval {
		t.Errorf("New(p, eu-west-1, true) = %+v", r)
	}
	if r.Data == nil || r.Errors == nil || r.newClients == nil {
		t.Errorf("New() left Data, Errors or newClients nil: %+v", r)
	}
}

// TestResults_accessors checks Len and the BaseResults getters.
func TestResults_accessors(t *testing.T) {
	r := New("p", "eu-west-1", false)
	r.Data = append(r.Data, dataRow{VpcID: testVpc})
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
	want := []interface{}{"VPC ID", "CIDR", "Status", "Details"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetHeaders() = %v, want %v", got, want)
	}
}

// TestResults_GetRows checks that every row is returned.
func TestResults_GetRows(t *testing.T) {
	r := New("p", "r", false)
	r.Data = []dataRow{{VpcID: testVpc, Status: StatusBlocked}}
	if got := r.GetRows(); len(got) != 1 || got[0].(dataRow).VpcID != testVpc {
		t.Errorf("GetRows() = %v", got)
	}
}

// TestGetSortFields checks a valid and an invalid sort field.
func TestGetSortFields(t *testing.T) {
	if _, err := GetSortFields("status"); err != nil {
		t.Errorf("GetSortFields(status) error = %v", err)
	}
	if _, err := GetSortFields("nope"); err == nil {
		t.Error("GetSortFields(nope) error = nil, want an error")
	}
}

// TestSortFieldNames checks the sort fields.
func TestSortFieldNames(t *testing.T) {
	if got, want := SortFieldNames(), []string{"id", "status"}; !reflect.DeepEqual(got, want) {
		t.Errorf("SortFieldNames() = %v, want %v", got, want)
	}
}

// TestResults_Search checks that Search records a client error, a listing error and a row.
func TestResults_Search(t *testing.T) {
	tests := []struct {
		name       string
		clientErr  error
		fake       *fakeEC2
		wantRows   int
		wantErrors int
	}{
		{name: "client error", clientErr: errors.New("no profile"), fake: emptyVPC(), wantErrors: 1},
		{name: "listing error", fake: &fakeEC2{errs: map[string]error{"DescribeVpcs": errors.New("boom")}}, wantErrors: 1},
		{name: "no default vpc", fake: &fakeEC2{}},
		{name: "deletable", fake: emptyVPC(), wantRows: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTest(false)
			r.newClients = func(string, string) (ec2API, cleanup.STSAPI, error) {
				return tt.fake, &fakeSTS{account: testAccount}, tt.clientErr
			}
			r.Search(context.Background())
			if r.Len() != tt.wantRows || len(r.Errors) != tt.wantErrors {
				t.Errorf("Search() rows = %d errors = %v, want %d rows %d errors", r.Len(), r.Errors, tt.wantRows, tt.wantErrors)
			}
		})
	}
}

// TestResults_run_noDefaultVPC checks that a region without a default VPC has no row and no call.
func TestResults_run_noDefaultVPC(t *testing.T) {
	f := &fakeEC2{}
	row, err := runTest(t, f, true)
	if row != nil || err != nil || len(deletes(f)) > 0 {
		t.Errorf("run() = %v, %v, calls %v; want nil, nil, none", row, err, deletes(f))
	}
}

// TestResults_run_otherOwner checks that a VPC owned by another account is never touched.
func TestResults_run_otherOwner(t *testing.T) {
	f := fullVPC()
	f.vpcs[0].OwnerId = aws.String("999999999999")
	row, err := runTest(t, f, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if row.Status != StatusBlocked || !strings.Contains(row.Details[0], "999999999999") || len(deletes(f)) > 0 {
		t.Errorf("run() = %+v, calls %v; want blocked by the owner, no call", row, deletes(f))
	}
}

// TestResults_run_blocked checks all-or-nothing: with one workload, nothing is deleted, even with
// apply, and the details name every blocker.
func TestResults_run_blocked(t *testing.T) {
	f := fullVPC()
	ec2ENI := eni("eni-i", types.NetworkInterfaceTypeInterface, types.NetworkInterfaceStatusInUse, "")
	ec2ENI.Attachment = &types.NetworkInterfaceAttachment{InstanceId: aws.String("i-1")}
	f.enis = append(f.enis, ec2ENI,
		eni("eni-rds", types.NetworkInterfaceTypeInterface, types.NetworkInterfaceStatusInUse, "RDSNetworkInterface"))
	row, err := runTest(t, f, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	want := []string{"EC2 instance i-1 (eni-i)", "RDSNetworkInterface (eni-rds)"}
	if row.Status != StatusBlocked || !reflect.DeepEqual(row.Details, want) {
		t.Errorf("run() = %s %v, want blocked %v", row.Status, row.Details, want)
	}
	if len(deletes(f)) > 0 {
		t.Errorf("run() calls = %v, want none", deletes(f))
	}
}

// TestResults_run_preview checks that a preview lists the plan and changes nothing.
func TestResults_run_preview(t *testing.T) {
	f := fullVPC()
	row, err := runTest(t, f, false)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if row.Status != StatusDeletable || row.VpcID != testVpc || row.CidrBlock != "172.31.0.0/16" {
		t.Errorf("run() = %+v, want deletable vpc-1 172.31.0.0/16", row)
	}
	if !reflect.DeepEqual(row.Details, fullPlan()) {
		t.Errorf("run() details =\n%s\nwant\n%s", strings.Join(row.Details, "\n"), strings.Join(fullPlan(), "\n"))
	}
	if len(deletes(f)) > 0 {
		t.Errorf("run() calls = %v, want none", deletes(f))
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
		{name: "allowed", code: "DryRunOperation", wantStatus: StatusDeletable},
		{name: "denied", code: "UnauthorizedOperation", wantStatus: StatusDenied},
		{name: "other error", code: "RequestLimitExceeded", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := emptyVPC()
			f.dryRunCode = tt.code
			row, err := runTest(t, f, false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("run() error = %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && row.Status != tt.wantStatus {
				t.Errorf("run() status = %s, want %s", row.Status, tt.wantStatus)
			}
		})
	}
}

// TestResults_run_apply checks the deletion order, the waits, and that only the plumbing is
// deleted: the Transit Gateway, the VPN gateway and the other VPC's gateway stay.
func TestResults_run_apply(t *testing.T) {
	f := fullVPC()
	row, err := runTest(t, f, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if row.Status != StatusDeleted || !reflect.DeepEqual(row.Details, fullPlan()) {
		t.Errorf("run() = %s %v, want deleted with the plan", row.Status, row.Details)
	}
	if got := deletes(f); !reflect.DeepEqual(got, fullCalls()) {
		t.Errorf("run() calls =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(fullCalls(), "\n"))
	}
	if len(f.vpcs) != 0 || len(f.eigws) != 1 || len(f.vpn) != 1 {
		t.Errorf("after run(): vpcs %d, egress-only gateways %d, vpn gateways %d; want 0, 1, 1",
			len(f.vpcs), len(f.eigws), len(f.vpn))
	}
}

// TestResults_run_applyResume checks that a run after a partial one deletes what is left, and
// that a resource already gone counts as deleted.
func TestResults_run_applyResume(t *testing.T) {
	f := emptyVPC()
	f.errs = map[string]error{"DeleteSubnet": apiErr("InvalidSubnetID.NotFound")}
	row, err := runTest(t, f, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if row.Status != StatusDeleted {
		t.Errorf("run() = %+v, want deleted", row)
	}
}

// TestResults_run_applyFailed checks that a failed step stops the run with the step and the error.
func TestResults_run_applyFailed(t *testing.T) {
	tests := []struct {
		name     string
		fake     func() *fakeEC2
		wantPart string
	}{
		{name: "vpc", fake: func() *fakeEC2 {
			f := emptyVPC()
			f.errs = map[string]error{"DeleteVpc": apiErr("DependencyViolation")}
			return f
		}, wantPart: "failed step: delete the VPC\nerror: vpc-1: api error DependencyViolation"},
		{name: "unsuccessful endpoint", fake: func() *fakeEC2 {
			f := fullVPC()
			f.unsuccessful = []types.UnsuccessfulItem{{ResourceId: aws.String("vpce-1"),
				Error: &types.UnsuccessfulItemError{Code: aws.String("Boom"), Message: aws.String("cannot")}}}
			return f
		}, wantPart: "failed step: delete the VPC endpoints\nerror: vpce-1: cannot"},
		{name: "security group", fake: func() *fakeEC2 {
			f := fullVPC()
			f.errs = map[string]error{"DeleteSecurityGroup": apiErr("DependencyViolation")}
			return f
		}, wantPart: "failed step: delete the security groups\nerror: sg-a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, err := runTest(t, tt.fake(), true)
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if row.Status != StatusFailed || !strings.Contains(strings.Join(row.Details, "\n"), tt.wantPart) {
				t.Errorf("run() = %s %v, want failed with %q", row.Status, row.Details, tt.wantPart)
			}
		})
	}
}

// TestResults_run_errors checks that an AWS error while listing is returned.
func TestResults_run_errors(t *testing.T) {
	tests := []struct {
		name string
		call string
		sts  error
	}{
		{name: "vpcs", call: "DescribeVpcs"},
		{name: "account", sts: errors.New("expired")},
		{name: "enis", call: "DescribeNetworkInterfaces"},
		{name: "peerings", call: "DescribeVpcPeeringConnections"},
		{name: "vpn gateways", call: "DescribeVpnGateways"},
		{name: "subnets", call: "DescribeSubnets"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fullVPC()
			f.errs = map[string]error{tt.call: errors.New("boom")}
			_, err := newTest(false).run(context.Background(), f, &fakeSTS{account: testAccount, err: tt.sts})
			if err == nil {
				t.Errorf("run() error = nil, want an error")
			}
		})
	}
}

// TestListInventory_pageSize checks that every paginated Describe call sends MaxResults.
func TestListInventory_pageSize(t *testing.T) {
	f := fullVPC()
	if _, err := runTest(t, f, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	for name, size := range f.pageSizes {
		if name != "DescribeVpnGateways" && name != "DescribeAddresses" && size == 0 {
			t.Errorf("%s MaxResults = 0, want a page size", name)
		}
	}
	if got := len(f.pageSizes); got != 15 {
		t.Errorf("Describe calls = %d, want 15", got)
	}
}

// TestClassifyENI checks which ENIs block, which are plumbing and which are deleted.
func TestClassifyENI(t *testing.T) {
	managed := eni("eni-m", types.NetworkInterfaceTypeInterface, types.NetworkInterfaceStatusAvailable, "Lambda leftover")
	managed.RequesterManaged = aws.Bool(true)
	tests := []struct {
		name        string
		eni         types.NetworkInterface
		wantBlocker string
		wantOwned   bool
	}{
		{name: "endpoint", eni: eni("eni-1", types.NetworkInterfaceTypeVpcEndpoint, types.NetworkInterfaceStatusInUse, "x")},
		{name: "nat", eni: eni("eni-1", types.NetworkInterfaceTypeNatGateway, types.NetworkInterfaceStatusInUse, "x")},
		{name: "nat as the API spells it", eni: eni("eni-1", "nat_gateway", types.NetworkInterfaceStatusInUse, "x")},
		{name: "tgw", eni: eni("eni-1", types.NetworkInterfaceTypeTransitGateway, types.NetworkInterfaceStatusInUse, "x")},
		{name: "owned detached", eni: eni("eni-1", "", types.NetworkInterfaceStatusAvailable, ""), wantOwned: true},
		{name: "aws-managed detached", eni: managed,
			wantBlocker: "AWS-managed detached ENI eni-m (Lambda leftover), retry later"},
		{name: "lambda", eni: eni("eni-1", types.NetworkInterfaceTypeLambda, types.NetworkInterfaceStatusInUse,
			"AWS Lambda VPC ENI-fn"), wantBlocker: "Lambda: AWS Lambda VPC ENI-fn (eni-1)"},
		{name: "load balancer", eni: eni("eni-1", types.NetworkInterfaceTypeNetworkLoadBalancer,
			types.NetworkInterfaceStatusInUse, "ELB net/nlb/1"), wantBlocker: "network_load_balancer: ELB net/nlb/1 (eni-1)"},
		{name: "typed without description", eni: eni("eni-1", types.NetworkInterfaceTypeEfa,
			types.NetworkInterfaceStatusInUse, ""), wantBlocker: "efa (eni-1)"},
		{name: "described", eni: eni("eni-1", types.NetworkInterfaceTypeInterface, types.NetworkInterfaceStatusInUse,
			"ElastiCache cache-1"), wantBlocker: "ElastiCache cache-1 (eni-1)"},
		{name: "unknown", eni: eni("eni-1", "", types.NetworkInterfaceStatusInUse, ""), wantBlocker: "ENI eni-1 in use"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocker, owned := classifyENI(&tt.eni)
			if blocker != tt.wantBlocker || owned != tt.wantOwned {
				t.Errorf("classifyENI(%s) = %q, %v, want %q, %v", tt.name, blocker, owned, tt.wantBlocker, tt.wantOwned)
			}
		})
	}
}

// TestParsePeering checks the peer side, the reject choice, and the connections that are gone.
func TestParsePeering(t *testing.T) {
	pcxs := fixturePeerings()
	tests := []struct {
		name   string
		pcx    types.VpcPeeringConnection
		want   peering
		wantOK bool
	}{
		{name: "requester", pcx: pcxs[0], wantOK: true,
			want: peering{id: "pcx-1", peerVpc: "vpc-9", peerAccount: "222222222222"}},
		{name: "pending accepter", pcx: pcxs[1], wantOK: true,
			want: peering{id: "pcx-2", peerVpc: "vpc-8", peerAccount: "333333333333", reject: true}},
		{name: "deleted", pcx: pcxs[2]},
		{name: "no status or sides", pcx: types.VpcPeeringConnection{VpcPeeringConnectionId: aws.String("pcx-3")},
			wantOK: true, want: peering{id: "pcx-3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePeering(&tt.pcx, testVpc)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("parsePeering(%s) = %+v, %v, want %+v, %v", tt.name, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestReferencing checks that only the group references to delete are revoked.
func TestReferencing(t *testing.T) {
	perms := []types.IpPermission{groupRule("sg-a", "sg-keep"), groupRule("sg-keep"), {IpProtocol: aws.String("tcp")}}
	got := referencing(perms, []string{"sg-a"})
	if len(got) != 1 || !reflect.DeepEqual(groupRefs(got), []string{"sg-a"}) {
		t.Errorf("referencing() = %v, want one rule referencing sg-a", groupRefs(got))
	}
	if len(perms[0].UserIdGroupPairs) != 2 {
		t.Errorf("referencing() changed its input: %v", groupRefs(perms))
	}
}

// TestUnsuccessful checks that the items already gone are ignored.
func TestUnsuccessful(t *testing.T) {
	items := []types.UnsuccessfulItem{
		{ResourceId: aws.String("a"), Error: &types.UnsuccessfulItemError{Code: aws.String("X.NotFound")}},
		{ResourceId: aws.String("b")},
	}
	if err := unsuccessful(items); err != nil {
		t.Errorf("unsuccessful(gone) = %v, want nil", err)
	}
	items = append(items, types.UnsuccessfulItem{ResourceId: aws.String("c"),
		Error: &types.UnsuccessfulItemError{Code: aws.String("Boom"), Message: aws.String("no")}})
	if err := unsuccessful(items); err == nil || err.Error() != "c: no" {
		t.Errorf("unsuccessful() = %v, want c: no", err)
	}
}

// TestResults_waitUntil checks that a wait ends with the context, with an error of done, and at
// the first success.
func TestResults_waitUntil(t *testing.T) {
	r := newTest(true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.waitUntil(ctx, "x", func() (bool, error) { return false, nil }); !errors.Is(err, context.Canceled) {
		t.Errorf("waitUntil(canceled) = %v, want context.Canceled", err)
	}
	boom := errors.New("boom")
	if err := r.waitUntil(context.Background(), "x", func() (bool, error) { return false, boom }); !errors.Is(err, boom) {
		t.Errorf("waitUntil(error) = %v, want boom", err)
	}
	n := 0
	err := r.waitUntil(context.Background(), "x", func() (bool, error) { n++; return n == 3, nil })
	if err != nil || n != 3 {
		t.Errorf("waitUntil(third time) = %v after %d calls, want nil after 3", err, n)
	}
}

// TestResults_waitENIs_newBlocker checks that a workload that appears during the run fails the
// wait at once.
func TestResults_waitENIs_newBlocker(t *testing.T) {
	f := emptyVPC()
	f.enis = []types.NetworkInterface{eni("eni-new", "", types.NetworkInterfaceStatusInUse, "new instance")}
	err := newTest(true).waitENIs(context.Background(), f, &inventory{vpcID: testVpc})
	if err == nil || !strings.Contains(err.Error(), "new blocker: new instance (eni-new)") {
		t.Errorf("waitENIs() = %v, want a new blocker error", err)
	}
}

// TestResults_apply_skipsEmptySteps checks that a VPC with only the default plumbing makes only
// the calls it needs.
func TestResults_apply_skipsEmptySteps(t *testing.T) {
	f := emptyVPC()
	if _, err := runTest(t, f, true); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	want := []string{
		"DeleteSubnet subnet-1", "DeleteSubnet subnet-2",
		"DetachInternetGateway igw-1", "DeleteInternetGateway igw-1", "DeleteVpc vpc-1",
	}
	if got := deletes(f); !slices.Equal(got, want) {
		t.Errorf("run() calls = %v, want %v", got, want)
	}
}

// count returns how many times call appears in calls.
func count(calls []string, call string) int {
	n := 0
	for _, c := range calls {
		if c == call {
			n++
		}
	}
	return n
}

// TestResults_run_applyReleaseRetry checks that a release refused while AWS still holds the IP
// of the deleted NAT gateway is tried again until it succeeds.
func TestResults_run_applyReleaseRetry(t *testing.T) {
	f := fullVPC()
	f.inUse = map[string]int{"eipalloc-1": 2}
	row, err := runTest(t, f, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if row.Status != StatusDeleted || count(f.calls, "ReleaseAddress eipalloc-1") != 3 || len(f.addresses) != 0 {
		t.Errorf("run() = %s %v, releases %d, addresses left %d; want deleted after 3 releases, none left",
			row.Status, row.Details, count(f.calls, "ReleaseAddress eipalloc-1"), len(f.addresses))
	}
}

// deletedNAT returns emptyVPC with a NAT gateway a previous run deleted, whose Elastic IP has
// the association association; "" means the IP no longer exists.
func deletedNAT(association string) *fakeEC2 {
	f := emptyVPC()
	f.nats = []types.NatGateway{{NatGatewayId: aws.String("nat-1"), State: types.NatGatewayStateDeleted,
		NatGatewayAddresses: []types.NatGatewayAddress{{AllocationId: aws.String("eipalloc-1"),
			PublicIp: aws.String("3.3.3.3"), AssociationId: aws.String("eipassoc-1")}}}}
	switch association {
	case "":
	case "none":
		f.addresses = []types.Address{{AllocationId: aws.String("eipalloc-1")}}
	default:
		f.addresses = []types.Address{{AllocationId: aws.String("eipalloc-1"), AssociationId: aws.String(association)}}
	}
	return f
}

// TestResults_run_resumeDeletedNAT checks that a run after one that deleted a NAT gateway
// releases its Elastic IP, unless the IP is gone or associated with something else.
func TestResults_run_resumeDeletedNAT(t *testing.T) {
	line := "deleted NAT gateway nat-1, Elastic IP 3.3.3.3 released (permanent)"
	tests := []struct {
		name        string
		association string
		wantRelease bool
	}{
		{name: "still associated with the gateway", association: "eipassoc-1", wantRelease: true},
		{name: "associated with nothing", association: "none", wantRelease: true},
		{name: "already released", association: ""},
		{name: "associated with something else", association: "eipassoc-other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := deletedNAT(tt.association)
			row, err := runTest(t, f, true)
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if got := slices.Contains(row.Details, line); got != tt.wantRelease {
				t.Errorf("run() details = %v, want the release line %v", row.Details, tt.wantRelease)
			}
			if got := count(f.calls, "ReleaseAddress eipalloc-1"); got != map[bool]int{true: 1}[tt.wantRelease] {
				t.Errorf("run() releases = %d, want release %v", got, tt.wantRelease)
			}
			if row.Status != StatusDeleted || count(f.calls, "DeleteNatGateway nat-1") != 0 {
				t.Errorf("run() = %s, calls %v; want deleted, no DeleteNatGateway", row.Status, f.calls)
			}
		})
	}
}

// TestResults_releaseAddress checks that a permission error fails at once, that an IP AWS
// released by itself counts as released, and that a failed lookup fails.
func TestResults_releaseAddress(t *testing.T) {
	tests := []struct {
		name      string
		errs      map[string]error
		addresses []types.Address
		wantErr   string
	}{
		{name: "unauthorized", errs: map[string]error{"ReleaseAddress": apiErr("UnauthorizedOperation")},
			addresses: []types.Address{{AllocationId: aws.String("eipalloc-1")}}, wantErr: "UnauthorizedOperation"},
		{name: "released by aws", errs: map[string]error{"ReleaseAddress": apiErr("AuthFailure")}},
		{name: "lookup fails", errs: map[string]error{
			"ReleaseAddress": apiErr("AuthFailure"), "DescribeAddresses": errors.New("boom"),
		}, wantErr: "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeEC2{errs: tt.errs, addresses: tt.addresses}
			err := newTest(true).releaseAddress(context.Background(), f, "eipalloc-1")
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("releaseAddress() = %v, want error %q", err, tt.wantErr)
			}
		})
	}
}

// TestResults_releaseAddress_ctx checks that a release still refused when the run ends reports
// the last AWS error.
func TestResults_releaseAddress_ctx(t *testing.T) {
	f := &fakeEC2{errs: map[string]error{"ReleaseAddress": apiErr("InvalidIPAddress.InUse")},
		addresses: []types.Address{{AllocationId: aws.String("eipalloc-1")}}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := newTest(true).releaseAddress(ctx, f, "eipalloc-1")
	if err == nil || !strings.Contains(err.Error(), "the last release error") ||
		!errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("releaseAddress() = %v, want the deadline and the last release error", err)
	}
}

// TestResults_run_awsManagedNATAddress checks that the Elastic IP AWS manages for a NAT gateway
// created without one is named in the plan, never released, and not left to release once the
// gateway is deleted.
func TestResults_run_awsManagedNATAddress(t *testing.T) {
	f := fullVPC()
	f.addresses[0].ServiceManaged = types.ServiceManagedRnat
	row, err := runTest(t, f, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	want := "NAT gateway nat-1, Elastic IP 3.3.3.3 managed by AWS (freed with the gateway)"
	if row.Status != StatusDeleted || row.Details[1] != want {
		t.Errorf("run() = %s, plan line %q, want deleted with %q", row.Status, row.Details[1], want)
	}
	if got := count(f.calls, "ReleaseAddress eipalloc-1"); got != 0 {
		t.Errorf("run() releases = %d, want 0", got)
	}

	resumed := deletedNAT("none")
	resumed.addresses[0].ServiceManaged = types.ServiceManagedRnat
	row, err = runTest(t, resumed, true)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if slices.ContainsFunc(row.Details, func(l string) bool { return strings.Contains(l, "nat-1") }) {
		t.Errorf("run() after the gateway was deleted = %v, want no line for nat-1", row.Details)
	}
}
