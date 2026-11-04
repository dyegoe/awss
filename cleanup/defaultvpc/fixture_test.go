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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	testVpc     = "vpc-1"
	testAccount = "111111111111"
)

// eni returns an ENI of the test VPC.
func eni(
	id string, typ types.NetworkInterfaceType, status types.NetworkInterfaceStatus, desc string,
) types.NetworkInterface {
	return types.NetworkInterface{
		NetworkInterfaceId: aws.String(id), InterfaceType: typ, Status: status, Description: aws.String(desc),
		VpcId: aws.String(testVpc),
	}
}

// groupRule returns an inbound or outbound rule that allows traffic from or to the groups.
func groupRule(groups ...string) types.IpPermission {
	p := types.IpPermission{IpProtocol: aws.String("-1")}
	for _, g := range groups {
		p.UserIdGroupPairs = append(p.UserIdGroupPairs, types.UserIdGroupPair{GroupId: aws.String(g)})
	}
	return p
}

// emptyVPC returns a fake region whose default VPC holds only what a fresh default VPC has.
func emptyVPC() *fakeEC2 {
	return &fakeEC2{
		vpcs: []types.Vpc{{
			VpcId: aws.String(testVpc), CidrBlock: aws.String("172.31.0.0/16"), OwnerId: aws.String(testAccount),
			IsDefault: aws.Bool(true),
		}},
		sgs: []types.SecurityGroup{{
			GroupId: aws.String("sg-0"), GroupName: aws.String("default"),
			IpPermissions: []types.IpPermission{groupRule("sg-0")},
		}},
		subnets: []types.Subnet{{SubnetId: aws.String("subnet-1")}, {SubnetId: aws.String("subnet-2")}},
		rts: []types.RouteTable{{
			RouteTableId: aws.String("rtb-main"), Associations: []types.RouteTableAssociation{{Main: aws.Bool(true)}},
		}},
		acls: []types.NetworkAcl{{NetworkAclId: aws.String("acl-default"), IsDefault: aws.Bool(true)}},
		igws: []types.InternetGateway{{InternetGatewayId: aws.String("igw-1")}},
	}
}

// fullVPC returns a fake region whose default VPC holds every kind of plumbing the cleanup
// deletes, and no workload.
func fullVPC() *fakeEC2 {
	f := emptyVPC()
	f.enis = []types.NetworkInterface{
		eni("eni-vpce", types.NetworkInterfaceTypeVpcEndpoint, types.NetworkInterfaceStatusInUse, "VPC Endpoint Interface"),
		eni("eni-nat", types.NetworkInterfaceTypeNatGateway, types.NetworkInterfaceStatusInUse, "NAT gateway"),
		eni("eni-d", types.NetworkInterfaceTypeInterface, types.NetworkInterfaceStatusAvailable, "spare"),
	}
	f.endpoints = []types.VpcEndpoint{
		{VpcEndpointId: aws.String("vpce-1"), State: "available"},
		{VpcEndpointId: aws.String("vpce-2"), State: "available"},
		{VpcEndpointId: aws.String("vpce-old"), State: "deleted"},
	}
	f.nats = []types.NatGateway{
		{NatGatewayId: aws.String("nat-1"), State: types.NatGatewayStateAvailable,
			NatGatewayAddresses: []types.NatGatewayAddress{
				{AllocationId: aws.String("eipalloc-1"), PublicIp: aws.String("3.3.3.3"),
					AssociationId: aws.String("eipassoc-1")},
			}},
		{NatGatewayId: aws.String("nat-2"), State: types.NatGatewayStateAvailable,
			NatGatewayAddresses: []types.NatGatewayAddress{{PrivateIp: aws.String("172.31.0.9")}}},
		{NatGatewayId: aws.String("nat-old"), State: types.NatGatewayStateDeleted},
	}
	f.peerings = fixturePeerings()
	f.tgw = []types.TransitGatewayVpcAttachment{{
		TransitGatewayAttachmentId: aws.String("tgw-attach-1"), TransitGatewayId: aws.String("tgw-1"),
		State: types.TransitGatewayAttachmentStateAvailable,
	}}
	f.vpn = []types.VpnGateway{{VpnGatewayId: aws.String("vgw-1"), VpcAttachments: []types.VpcAttachment{
		{VpcId: aws.String(testVpc), State: types.AttachmentStatusAttached},
	}}}
	f.sgs = append(f.sgs,
		types.SecurityGroup{GroupId: aws.String("sg-a"), GroupName: aws.String("a"),
			IpPermissions: []types.IpPermission{groupRule("sg-b")}},
		types.SecurityGroup{GroupId: aws.String("sg-b"), GroupName: aws.String("b"),
			IpPermissions:       []types.IpPermission{groupRule("sg-a"), {IpProtocol: aws.String("tcp")}},
			IpPermissionsEgress: []types.IpPermission{groupRule("sg-a", "sg-other")}},
	)
	f.sgs[0].IpPermissions = append(f.sgs[0].IpPermissions, groupRule("sg-a"))
	f.rts = append(f.rts, types.RouteTable{RouteTableId: aws.String("rtb-1"), Associations: []types.RouteTableAssociation{
		{RouteTableAssociationId: aws.String("rtbassoc-1"), SubnetId: aws.String("subnet-1"), Main: aws.Bool(false)},
		{RouteTableAssociationId: aws.String("rtbassoc-g"), GatewayId: aws.String("igw-1")},
	}})
	f.acls = append(f.acls, types.NetworkAcl{NetworkAclId: aws.String("acl-1"), IsDefault: aws.Bool(false)})
	f.eigws = []types.EgressOnlyInternetGateway{
		{EgressOnlyInternetGatewayId: aws.String("eigw-1"), Attachments: []types.InternetGatewayAttachment{
			{VpcId: aws.String(testVpc)},
		}},
		{EgressOnlyInternetGatewayId: aws.String("eigw-9"), Attachments: []types.InternetGatewayAttachment{
			{VpcId: aws.String("vpc-9")},
		}},
	}
	f.flowLogs = []types.FlowLog{{FlowLogId: aws.String("fl-1")}}
	f.addresses = []types.Address{{AllocationId: aws.String("eipalloc-1"), AssociationId: aws.String("eipassoc-1")}}
	return f
}

// peeringOf returns a peering connection from the requester VPC to the accepter VPC.
func peeringOf(id, requester, requesterOwner, accepter, accepterOwner string,
	code types.VpcPeeringConnectionStateReasonCode,
) types.VpcPeeringConnection {
	return types.VpcPeeringConnection{
		VpcPeeringConnectionId: aws.String(id),
		RequesterVpcInfo: &types.VpcPeeringConnectionVpcInfo{
			VpcId: aws.String(requester), OwnerId: aws.String(requesterOwner),
		},
		AccepterVpcInfo: &types.VpcPeeringConnectionVpcInfo{
			VpcId: aws.String(accepter), OwnerId: aws.String(accepterOwner),
		},
		Status: &types.VpcPeeringConnectionStateReason{Code: code},
	}
}

// fixturePeerings returns an active peering the test VPC requested, a request it was asked to
// accept, and a deleted one.
func fixturePeerings() []types.VpcPeeringConnection {
	return []types.VpcPeeringConnection{
		peeringOf("pcx-1", testVpc, testAccount, "vpc-9", "222222222222", types.VpcPeeringConnectionStateReasonCodeActive),
		peeringOf("pcx-2", "vpc-8", "333333333333", testVpc, testAccount,
			types.VpcPeeringConnectionStateReasonCodePendingAcceptance),
		peeringOf("pcx-old", testVpc, testAccount, "vpc-7", testAccount, types.VpcPeeringConnectionStateReasonCodeDeleted),
	}
}

// fullPlan returns the plan of fullVPC.
func fullPlan() []string {
	return []string{
		"VPC endpoints: vpce-1, vpce-2",
		"NAT gateway nat-1, Elastic IP 3.3.3.3 released (permanent)",
		"NAT gateway nat-2",
		"peering pcx-1 to vpc-9 in account 222222222222 (its routes on that side stop working)",
		"peering pcx-2 to vpc-8 in account 333333333333 (its routes on that side stop working)",
		"TGW attachment tgw-attach-1 (Transit Gateway tgw-1 kept)",
		"VPN gateway vgw-1 detached (the gateway and its VPN connections kept)",
		"detached ENIs: eni-d",
		"security groups: sg-a, sg-b",
		"subnets: subnet-1, subnet-2",
		"route tables: rtb-1",
		"network ACLs: acl-1",
		"internet gateways: igw-1",
		"egress-only internet gateways: eigw-1",
		"flow logs: fl-1",
		"VPC: vpc-1",
	}
}

// fullCalls returns the calls that delete fullVPC, in order.
func fullCalls() []string {
	return []string{
		"DeleteVpcEndpoints vpce-1,vpce-2",
		"DeleteNatGateway nat-1",
		"DeleteNatGateway nat-2",
		"ReleaseAddress eipalloc-1",
		"DeleteVpcPeeringConnection pcx-1",
		"RejectVpcPeeringConnection pcx-2",
		"DeleteTransitGatewayVpcAttachment tgw-attach-1",
		"DetachVpnGateway vgw-1",
		"DeleteNetworkInterface eni-d",
		"RevokeSecurityGroupIngress sg-0<-sg-a",
		"RevokeSecurityGroupIngress sg-a<-sg-b",
		"RevokeSecurityGroupIngress sg-b<-sg-a",
		"RevokeSecurityGroupEgress sg-b->sg-a",
		"DeleteSecurityGroup sg-a",
		"DeleteSecurityGroup sg-b",
		"DeleteSubnet subnet-1",
		"DeleteSubnet subnet-2",
		"DisassociateRouteTable rtbassoc-1",
		"DisassociateRouteTable rtbassoc-g",
		"DeleteRouteTable rtb-1",
		"DeleteNetworkAcl acl-1",
		"DetachInternetGateway igw-1",
		"DeleteInternetGateway igw-1",
		"DeleteEgressOnlyInternetGateway eigw-1",
		"DeleteFlowLogs fl-1",
		"DeleteVpc vpc-1",
	}
}
