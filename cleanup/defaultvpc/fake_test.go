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
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// fakeEC2 is an ec2API holding the resources of one region in memory. The Describe calls return
// everything they hold, except the peering connections, matched by side. The delete calls remove
// the resource and record the call; a missing resource answers a NotFound error, as AWS does.
//
// Like AWS, some deletions finish later: a deleted endpoint or Transit Gateway attachment, and a
// detaching VPN gateway, are still listed once by the next Describe; a deleting NAT gateway is
// listed once more, then stays listed as deleted, with its addresses; the ENIs of the plumbing are
// listed once more after any deletion started.
type fakeEC2 struct {
	vpcs      []types.Vpc
	enis      []types.NetworkInterface
	endpoints []types.VpcEndpoint
	nats      []types.NatGateway
	peerings  []types.VpcPeeringConnection
	tgw       []types.TransitGatewayVpcAttachment
	vpn       []types.VpnGateway
	sgs       []types.SecurityGroup
	subnets   []types.Subnet
	rts       []types.RouteTable
	acls      []types.NetworkAcl
	igws      []types.InternetGateway
	eigws     []types.EgressOnlyInternetGateway
	flowLogs  []types.FlowLog
	addresses []types.Address

	// errs fails the call of that name with that error.
	errs map[string]error
	// dryRunCode is the error code of a DryRun DeleteVpc; "" means DryRunOperation.
	dryRunCode string
	// inUse is how many more times ReleaseAddress answers that the IP is in use, by allocation ID,
	// as AWS does a little after a NAT gateway is deleted.
	inUse map[string]int
	// unsuccessful is returned as the Unsuccessful items of DeleteVpcEndpoints and DeleteFlowLogs.
	unsuccessful []types.UnsuccessfulItem

	// calls records the calls that change resources, as "Name id".
	calls []string
	// pageSizes records the MaxResults of every Describe call, by call name.
	pageSizes map[string]int32
	// deleting is set once a deletion started.
	deleting bool
}

// fakeSTS is an stsAPI that answers account, or err.
type fakeSTS struct {
	account string
	err     error
}

// GetCallerIdentity returns the account of the fake.
func (f *fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (
	*sts.GetCallerIdentityOutput, error,
) {
	if f.err != nil {
		return nil, f.err
	}
	return &sts.GetCallerIdentityOutput{Account: aws.String(f.account)}, nil
}

// apiErr returns an AWS API error with code.
func apiErr(code string) error {
	return &smithy.GenericAPIError{Code: code, Message: code + " message"}
}

// describe records the page size of a Describe call and returns its injected error.
func (f *fakeEC2) describe(name string, maxResults *int32) error {
	if f.pageSizes == nil {
		f.pageSizes = map[string]int32{}
	}
	f.pageSizes[name] = aws.ToInt32(maxResults)
	return f.errs[name]
}

// change records a call that changes resources and returns its injected error.
func (f *fakeEC2) change(name string, ids ...string) error {
	f.deleting = true
	f.calls = append(f.calls, strings.TrimSpace(name+" "+strings.Join(ids, ",")))
	return f.errs[name]
}

// deleteOne records the call name for the resource want, and removes from s the item whose id is
// want. A missing item answers the NotFound error of kind, as AWS does.
//
// An empty name records no call: the caller recorded it.
func deleteOne[T any](f *fakeEC2, name, kind string, s *[]T, id func(*T) *string, want *string) error {
	if name != "" {
		if err := f.change(name, aws.ToString(want)); err != nil {
			return err
		}
	}
	n := len(*s)
	*s = slices.DeleteFunc(*s, func(v T) bool { return aws.ToString(id(&v)) == aws.ToString(want) })
	if len(*s) < n {
		return nil
	}
	return apiErr("Invalid" + kind + ".NotFound")
}

// DescribeVpcs returns the VPCs.
func (f *fakeEC2) DescribeVpcs(_ context.Context, in *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeVpcsOutput, error,
) {
	return &ec2.DescribeVpcsOutput{Vpcs: slices.Clone(f.vpcs)}, f.describe("DescribeVpcs", in.MaxResults)
}

// DescribeNetworkInterfaces returns the ENIs; once a deletion started, the plumbing ENIs are
// returned one last time.
func (f *fakeEC2) DescribeNetworkInterfaces(
	_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options),
) (*ec2.DescribeNetworkInterfacesOutput, error) {
	out := &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: slices.Clone(f.enis)}
	if f.deleting {
		f.enis = slices.DeleteFunc(f.enis, func(e types.NetworkInterface) bool {
			return slices.Contains(plumbingENIs, e.InterfaceType)
		})
	}
	return out, f.describe("DescribeNetworkInterfaces", in.MaxResults)
}

// DescribeVpcEndpoints returns the endpoints; a deleting one is returned one last time.
func (f *fakeEC2) DescribeVpcEndpoints(_ context.Context, in *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeVpcEndpointsOutput, error,
) {
	out := &ec2.DescribeVpcEndpointsOutput{VpcEndpoints: slices.Clone(f.endpoints)}
	f.endpoints = slices.DeleteFunc(f.endpoints, func(e types.VpcEndpoint) bool { return e.State == "deleting" })
	return out, f.describe("DescribeVpcEndpoints", in.MaxResults)
}

// DescribeNatGateways returns the NAT gateways; a deleting one is returned one last time.
func (f *fakeEC2) DescribeNatGateways(_ context.Context, in *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeNatGatewaysOutput, error,
) {
	out := &ec2.DescribeNatGatewaysOutput{NatGateways: slices.Clone(f.nats)}
	for i := range f.nats {
		if f.nats[i].State == types.NatGatewayStateDeleting {
			f.nats[i].State = types.NatGatewayStateDeleted
		}
	}
	return out, f.describe("DescribeNatGateways", in.MaxResults)
}

// DescribeAddresses returns the Elastic IPs of the allocation-id filter.
func (f *fakeEC2) DescribeAddresses(_ context.Context, in *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeAddressesOutput, error,
) {
	out := &ec2.DescribeAddressesOutput{}
	for i := range f.addresses {
		if slices.Contains(in.Filters[0].Values, aws.ToString(f.addresses[i].AllocationId)) {
			out.Addresses = append(out.Addresses, f.addresses[i])
		}
	}
	return out, f.describe("DescribeAddresses", nil)
}

// DescribeVpcPeeringConnections returns the peering connections whose side, named by the filter,
// is the filter's VPC.
func (f *fakeEC2) DescribeVpcPeeringConnections(
	_ context.Context, in *ec2.DescribeVpcPeeringConnectionsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeVpcPeeringConnectionsOutput, error) {
	out := &ec2.DescribeVpcPeeringConnectionsOutput{}
	for _, p := range f.peerings {
		info := p.RequesterVpcInfo
		if aws.ToString(in.Filters[0].Name) == "accepter-vpc-info.vpc-id" {
			info = p.AccepterVpcInfo
		}
		if info != nil && aws.ToString(info.VpcId) == in.Filters[0].Values[0] {
			out.VpcPeeringConnections = append(out.VpcPeeringConnections, p)
		}
	}
	return out, f.describe("DescribeVpcPeeringConnections", in.MaxResults)
}

// DescribeTransitGatewayVpcAttachments returns the attachments; a deleting one is returned one
// last time.
func (f *fakeEC2) DescribeTransitGatewayVpcAttachments(
	_ context.Context, in *ec2.DescribeTransitGatewayVpcAttachmentsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	out := &ec2.DescribeTransitGatewayVpcAttachmentsOutput{TransitGatewayVpcAttachments: slices.Clone(f.tgw)}
	f.tgw = slices.DeleteFunc(f.tgw, func(a types.TransitGatewayVpcAttachment) bool {
		return a.State == types.TransitGatewayAttachmentStateDeleting
	})
	return out, f.describe("DescribeTransitGatewayVpcAttachments", in.MaxResults)
}

// DescribeVpnGateways returns the VPN gateways; a detaching attachment is returned one last time.
func (f *fakeEC2) DescribeVpnGateways(_ context.Context, _ *ec2.DescribeVpnGatewaysInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeVpnGatewaysOutput, error,
) {
	out := &ec2.DescribeVpnGatewaysOutput{}
	for i := range f.vpn {
		gw := f.vpn[i]
		gw.VpcAttachments = slices.Clone(gw.VpcAttachments)
		out.VpnGateways = append(out.VpnGateways, gw)
		for j := range f.vpn[i].VpcAttachments {
			if f.vpn[i].VpcAttachments[j].State == types.AttachmentStatusDetaching {
				f.vpn[i].VpcAttachments[j].State = types.AttachmentStatusDetached
			}
		}
	}
	return out, f.describe("DescribeVpnGateways", nil)
}

// DescribeSecurityGroups returns the security groups.
func (f *fakeEC2) DescribeSecurityGroups(
	_ context.Context, in *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: slices.Clone(f.sgs)},
		f.describe("DescribeSecurityGroups", in.MaxResults)
}

// DescribeSubnets returns the subnets.
func (f *fakeEC2) DescribeSubnets(_ context.Context, in *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeSubnetsOutput, error,
) {
	return &ec2.DescribeSubnetsOutput{Subnets: slices.Clone(f.subnets)}, f.describe("DescribeSubnets", in.MaxResults)
}

// DescribeRouteTables returns the route tables.
func (f *fakeEC2) DescribeRouteTables(_ context.Context, in *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeRouteTablesOutput, error,
) {
	return &ec2.DescribeRouteTablesOutput{RouteTables: slices.Clone(f.rts)},
		f.describe("DescribeRouteTables", in.MaxResults)
}

// DescribeNetworkAcls returns the network ACLs.
func (f *fakeEC2) DescribeNetworkAcls(_ context.Context, in *ec2.DescribeNetworkAclsInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeNetworkAclsOutput, error,
) {
	return &ec2.DescribeNetworkAclsOutput{NetworkAcls: slices.Clone(f.acls)},
		f.describe("DescribeNetworkAcls", in.MaxResults)
}

// DescribeInternetGateways returns the internet gateways.
func (f *fakeEC2) DescribeInternetGateways(
	_ context.Context, in *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options),
) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{InternetGateways: slices.Clone(f.igws)},
		f.describe("DescribeInternetGateways", in.MaxResults)
}

// DescribeEgressOnlyInternetGateways returns every egress-only internet gateway: the call has no
// VPC filter.
func (f *fakeEC2) DescribeEgressOnlyInternetGateways(
	_ context.Context, in *ec2.DescribeEgressOnlyInternetGatewaysInput, _ ...func(*ec2.Options),
) (*ec2.DescribeEgressOnlyInternetGatewaysOutput, error) {
	return &ec2.DescribeEgressOnlyInternetGatewaysOutput{EgressOnlyInternetGateways: slices.Clone(f.eigws)},
		f.describe("DescribeEgressOnlyInternetGateways", in.MaxResults)
}

// DescribeFlowLogs returns the flow logs.
func (f *fakeEC2) DescribeFlowLogs(_ context.Context, in *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (
	*ec2.DescribeFlowLogsOutput, error,
) {
	return &ec2.DescribeFlowLogsOutput{FlowLogs: slices.Clone(f.flowLogs)}, f.describe("DescribeFlowLogs", in.MaxResults)
}
