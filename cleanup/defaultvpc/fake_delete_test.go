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
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// DeleteVpcEndpoints marks the endpoints deleting.
func (f *fakeEC2) DeleteVpcEndpoints(_ context.Context, in *ec2.DeleteVpcEndpointsInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteVpcEndpointsOutput, error,
) {
	if err := f.change("DeleteVpcEndpoints", in.VpcEndpointIds...); err != nil {
		return nil, err
	}
	for i := range f.endpoints {
		if slices.Contains(in.VpcEndpointIds, aws.ToString(f.endpoints[i].VpcEndpointId)) {
			f.endpoints[i].State = "deleting"
		}
	}
	return &ec2.DeleteVpcEndpointsOutput{Unsuccessful: f.unsuccessful}, nil
}

// DeleteNatGateway marks the NAT gateway deleting.
func (f *fakeEC2) DeleteNatGateway(_ context.Context, in *ec2.DeleteNatGatewayInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteNatGatewayOutput, error,
) {
	id := aws.ToString(in.NatGatewayId)
	if err := f.change("DeleteNatGateway", id); err != nil {
		return nil, err
	}
	for i := range f.nats {
		if aws.ToString(f.nats[i].NatGatewayId) == id {
			f.nats[i].State = types.NatGatewayStateDeleting
			return &ec2.DeleteNatGatewayOutput{}, nil
		}
	}
	return nil, apiErr("NatGatewayNotFound")
}

// ReleaseAddress releases an Elastic IP. It fails while a NAT gateway still lists the IP, as AWS
// does while the IP is associated.
func (f *fakeEC2) ReleaseAddress(_ context.Context, in *ec2.ReleaseAddressInput, _ ...func(*ec2.Options)) (
	*ec2.ReleaseAddressOutput, error,
) {
	id := aws.ToString(in.AllocationId)
	if err := f.change("ReleaseAddress", id); err != nil {
		return nil, err
	}
	for i := range f.nats {
		for _, a := range f.nats[i].NatGatewayAddresses {
			if aws.ToString(a.AllocationId) == id {
				return nil, apiErr("InvalidIPAddress.InUse")
			}
		}
	}
	return &ec2.ReleaseAddressOutput{}, nil
}

// DeleteVpcPeeringConnection deletes a peering connection.
func (f *fakeEC2) DeleteVpcPeeringConnection(
	_ context.Context, in *ec2.DeleteVpcPeeringConnectionInput, _ ...func(*ec2.Options),
) (*ec2.DeleteVpcPeeringConnectionOutput, error) {
	return &ec2.DeleteVpcPeeringConnectionOutput{}, deleteOne(f, "DeleteVpcPeeringConnection", "VpcPeeringConnectionID",
		&f.peerings,
		func(p *types.VpcPeeringConnection) *string { return p.VpcPeeringConnectionId }, in.VpcPeeringConnectionId)
}

// RejectVpcPeeringConnection rejects a peering request.
func (f *fakeEC2) RejectVpcPeeringConnection(
	_ context.Context, in *ec2.RejectVpcPeeringConnectionInput, _ ...func(*ec2.Options),
) (*ec2.RejectVpcPeeringConnectionOutput, error) {
	return &ec2.RejectVpcPeeringConnectionOutput{}, deleteOne(f, "RejectVpcPeeringConnection", "VpcPeeringConnectionID",
		&f.peerings,
		func(p *types.VpcPeeringConnection) *string { return p.VpcPeeringConnectionId }, in.VpcPeeringConnectionId)
}

// DeleteTransitGatewayVpcAttachment marks the attachment deleting.
func (f *fakeEC2) DeleteTransitGatewayVpcAttachment(
	_ context.Context, in *ec2.DeleteTransitGatewayVpcAttachmentInput, _ ...func(*ec2.Options),
) (*ec2.DeleteTransitGatewayVpcAttachmentOutput, error) {
	id := aws.ToString(in.TransitGatewayAttachmentId)
	if err := f.change("DeleteTransitGatewayVpcAttachment", id); err != nil {
		return nil, err
	}
	for i := range f.tgw {
		if aws.ToString(f.tgw[i].TransitGatewayAttachmentId) == id {
			f.tgw[i].State = types.TransitGatewayAttachmentStateDeleting
		}
	}
	return &ec2.DeleteTransitGatewayVpcAttachmentOutput{}, nil
}

// DetachVpnGateway marks the attachment of the VPN gateway detaching.
func (f *fakeEC2) DetachVpnGateway(_ context.Context, in *ec2.DetachVpnGatewayInput, _ ...func(*ec2.Options)) (
	*ec2.DetachVpnGatewayOutput, error,
) {
	if err := f.change("DetachVpnGateway", aws.ToString(in.VpnGatewayId)); err != nil {
		return nil, err
	}
	for i := range f.vpn {
		for j := range f.vpn[i].VpcAttachments {
			if aws.ToString(f.vpn[i].VpnGatewayId) == aws.ToString(in.VpnGatewayId) {
				f.vpn[i].VpcAttachments[j].State = types.AttachmentStatusDetaching
			}
		}
	}
	return &ec2.DetachVpnGatewayOutput{}, nil
}

// DeleteNetworkInterface deletes an ENI.
func (f *fakeEC2) DeleteNetworkInterface(
	_ context.Context, in *ec2.DeleteNetworkInterfaceInput, _ ...func(*ec2.Options),
) (*ec2.DeleteNetworkInterfaceOutput, error) {
	return &ec2.DeleteNetworkInterfaceOutput{}, deleteOne(f, "DeleteNetworkInterface", "NetworkInterfaceID", &f.enis,
		func(e *types.NetworkInterface) *string { return e.NetworkInterfaceId }, in.NetworkInterfaceId)
}

// groupRefs returns the groups perms reference, for the call record.
func groupRefs(perms []types.IpPermission) []string {
	var refs []string
	for i := range perms {
		for _, pair := range perms[i].UserIdGroupPairs {
			refs = append(refs, aws.ToString(pair.GroupId))
		}
	}
	return refs
}

// RevokeSecurityGroupIngress records the groups the revoked inbound rules referenced.
func (f *fakeEC2) RevokeSecurityGroupIngress(
	_ context.Context, in *ec2.RevokeSecurityGroupIngressInput, _ ...func(*ec2.Options),
) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	return &ec2.RevokeSecurityGroupIngressOutput{}, f.change("RevokeSecurityGroupIngress",
		fmt.Sprintf("%s<-%s", aws.ToString(in.GroupId), strings.Join(groupRefs(in.IpPermissions), "+")))
}

// RevokeSecurityGroupEgress records the groups the revoked outbound rules referenced.
func (f *fakeEC2) RevokeSecurityGroupEgress(
	_ context.Context, in *ec2.RevokeSecurityGroupEgressInput, _ ...func(*ec2.Options),
) (*ec2.RevokeSecurityGroupEgressOutput, error) {
	return &ec2.RevokeSecurityGroupEgressOutput{}, f.change("RevokeSecurityGroupEgress",
		fmt.Sprintf("%s->%s", aws.ToString(in.GroupId), strings.Join(groupRefs(in.IpPermissions), "+")))
}

// DeleteSecurityGroup deletes a security group.
func (f *fakeEC2) DeleteSecurityGroup(_ context.Context, in *ec2.DeleteSecurityGroupInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteSecurityGroupOutput, error,
) {
	return &ec2.DeleteSecurityGroupOutput{}, deleteOne(f, "DeleteSecurityGroup", "Group", &f.sgs,
		func(g *types.SecurityGroup) *string { return g.GroupId }, in.GroupId)
}

// DeleteSubnet deletes a subnet.
func (f *fakeEC2) DeleteSubnet(_ context.Context, in *ec2.DeleteSubnetInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteSubnetOutput, error,
) {
	return &ec2.DeleteSubnetOutput{}, deleteOne(f, "DeleteSubnet", "SubnetID", &f.subnets,
		func(s *types.Subnet) *string { return s.SubnetId }, in.SubnetId)
}

// DisassociateRouteTable records the association removed.
func (f *fakeEC2) DisassociateRouteTable(
	_ context.Context, in *ec2.DisassociateRouteTableInput, _ ...func(*ec2.Options),
) (*ec2.DisassociateRouteTableOutput, error) {
	return &ec2.DisassociateRouteTableOutput{}, f.change("DisassociateRouteTable", aws.ToString(in.AssociationId))
}

// DeleteRouteTable deletes a route table.
func (f *fakeEC2) DeleteRouteTable(_ context.Context, in *ec2.DeleteRouteTableInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteRouteTableOutput, error,
) {
	return &ec2.DeleteRouteTableOutput{}, deleteOne(f, "DeleteRouteTable", "RouteTableID", &f.rts,
		func(r *types.RouteTable) *string { return r.RouteTableId }, in.RouteTableId)
}

// DeleteNetworkAcl deletes a network ACL.
//
//nolint:revive // the name is the SDK's, which the ec2API interface requires
func (f *fakeEC2) DeleteNetworkAcl(_ context.Context, in *ec2.DeleteNetworkAclInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteNetworkAclOutput, error,
) {
	return &ec2.DeleteNetworkAclOutput{}, deleteOne(f, "DeleteNetworkAcl", "NetworkAclID", &f.acls,
		func(a *types.NetworkAcl) *string { return a.NetworkAclId }, in.NetworkAclId)
}

// DetachInternetGateway records the detach.
func (f *fakeEC2) DetachInternetGateway(
	_ context.Context, in *ec2.DetachInternetGatewayInput, _ ...func(*ec2.Options),
) (*ec2.DetachInternetGatewayOutput, error) {
	return &ec2.DetachInternetGatewayOutput{}, f.change("DetachInternetGateway", aws.ToString(in.InternetGatewayId))
}

// DeleteInternetGateway deletes an internet gateway.
func (f *fakeEC2) DeleteInternetGateway(
	_ context.Context, in *ec2.DeleteInternetGatewayInput, _ ...func(*ec2.Options),
) (*ec2.DeleteInternetGatewayOutput, error) {
	return &ec2.DeleteInternetGatewayOutput{}, deleteOne(f, "DeleteInternetGateway", "InternetGatewayID", &f.igws,
		func(g *types.InternetGateway) *string { return g.InternetGatewayId }, in.InternetGatewayId)
}

// DeleteEgressOnlyInternetGateway deletes an egress-only internet gateway.
func (f *fakeEC2) DeleteEgressOnlyInternetGateway(
	_ context.Context, in *ec2.DeleteEgressOnlyInternetGatewayInput, _ ...func(*ec2.Options),
) (*ec2.DeleteEgressOnlyInternetGatewayOutput, error) {
	return &ec2.DeleteEgressOnlyInternetGatewayOutput{}, deleteOne(f, "DeleteEgressOnlyInternetGateway", "GatewayID",
		&f.eigws, func(g *types.EgressOnlyInternetGateway) *string { return g.EgressOnlyInternetGatewayId },
		in.EgressOnlyInternetGatewayId)
}

// DeleteFlowLogs deletes flow logs.
func (f *fakeEC2) DeleteFlowLogs(_ context.Context, in *ec2.DeleteFlowLogsInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteFlowLogsOutput, error,
) {
	if err := f.change("DeleteFlowLogs", in.FlowLogIds...); err != nil {
		return nil, err
	}
	f.flowLogs = slices.DeleteFunc(f.flowLogs, func(l types.FlowLog) bool {
		return slices.Contains(in.FlowLogIds, aws.ToString(l.FlowLogId))
	})
	return &ec2.DeleteFlowLogsOutput{Unsuccessful: f.unsuccessful}, nil
}

// DeleteVpc deletes the VPC, or answers a DryRun with dryRunCode without recording a call.
func (f *fakeEC2) DeleteVpc(_ context.Context, in *ec2.DeleteVpcInput, _ ...func(*ec2.Options)) (
	*ec2.DeleteVpcOutput, error,
) {
	if aws.ToBool(in.DryRun) {
		if f.dryRunCode == "" {
			return nil, apiErr("DryRunOperation")
		}
		return nil, apiErr(f.dryRunCode)
	}
	return &ec2.DeleteVpcOutput{}, deleteOne(f, "DeleteVpc", "VpcID", &f.vpcs,
		func(v *types.Vpc) *string { return v.VpcId }, in.VpcId)
}
