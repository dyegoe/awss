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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// ec2API is the part of the EC2 client the cleanup uses. The real client is *ec2.Client; tests
// pass a fake that holds the resources of one VPC in memory.
type ec2API interface {
	ec2.DescribeVpcsAPIClient
	ec2.DescribeNetworkInterfacesAPIClient
	ec2.DescribeVpcEndpointsAPIClient
	ec2.DescribeNatGatewaysAPIClient
	ec2.DescribeVpcPeeringConnectionsAPIClient
	ec2.DescribeTransitGatewayVpcAttachmentsAPIClient
	ec2.DescribeSecurityGroupsAPIClient
	ec2.DescribeSubnetsAPIClient
	ec2.DescribeRouteTablesAPIClient
	ec2.DescribeNetworkAclsAPIClient
	ec2.DescribeInternetGatewaysAPIClient
	ec2.DescribeEgressOnlyInternetGatewaysAPIClient
	ec2.DescribeFlowLogsAPIClient
	DescribeVpnGateways(context.Context, *ec2.DescribeVpnGatewaysInput, ...func(*ec2.Options)) (
		*ec2.DescribeVpnGatewaysOutput, error)
	DescribeAddresses(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (
		*ec2.DescribeAddressesOutput, error)

	deleteAPI
}

// deleteAPI holds the calls that change resources.
type deleteAPI interface {
	DeleteVpcEndpoints(context.Context, *ec2.DeleteVpcEndpointsInput, ...func(*ec2.Options)) (
		*ec2.DeleteVpcEndpointsOutput, error)
	DeleteNatGateway(context.Context, *ec2.DeleteNatGatewayInput, ...func(*ec2.Options)) (
		*ec2.DeleteNatGatewayOutput, error)
	ReleaseAddress(context.Context, *ec2.ReleaseAddressInput, ...func(*ec2.Options)) (
		*ec2.ReleaseAddressOutput, error)
	DeleteVpcPeeringConnection(context.Context, *ec2.DeleteVpcPeeringConnectionInput, ...func(*ec2.Options)) (
		*ec2.DeleteVpcPeeringConnectionOutput, error)
	RejectVpcPeeringConnection(context.Context, *ec2.RejectVpcPeeringConnectionInput, ...func(*ec2.Options)) (
		*ec2.RejectVpcPeeringConnectionOutput, error)
	DeleteTransitGatewayVpcAttachment(
		context.Context, *ec2.DeleteTransitGatewayVpcAttachmentInput, ...func(*ec2.Options),
	) (*ec2.DeleteTransitGatewayVpcAttachmentOutput, error)
	DetachVpnGateway(context.Context, *ec2.DetachVpnGatewayInput, ...func(*ec2.Options)) (
		*ec2.DetachVpnGatewayOutput, error)
	DeleteNetworkInterface(context.Context, *ec2.DeleteNetworkInterfaceInput, ...func(*ec2.Options)) (
		*ec2.DeleteNetworkInterfaceOutput, error)
	RevokeSecurityGroupIngress(context.Context, *ec2.RevokeSecurityGroupIngressInput, ...func(*ec2.Options)) (
		*ec2.RevokeSecurityGroupIngressOutput, error)
	RevokeSecurityGroupEgress(context.Context, *ec2.RevokeSecurityGroupEgressInput, ...func(*ec2.Options)) (
		*ec2.RevokeSecurityGroupEgressOutput, error)
	DeleteSecurityGroup(context.Context, *ec2.DeleteSecurityGroupInput, ...func(*ec2.Options)) (
		*ec2.DeleteSecurityGroupOutput, error)
	DeleteSubnet(context.Context, *ec2.DeleteSubnetInput, ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)
	DisassociateRouteTable(context.Context, *ec2.DisassociateRouteTableInput, ...func(*ec2.Options)) (
		*ec2.DisassociateRouteTableOutput, error)
	DeleteRouteTable(context.Context, *ec2.DeleteRouteTableInput, ...func(*ec2.Options)) (
		*ec2.DeleteRouteTableOutput, error)
	DeleteNetworkAcl(context.Context, *ec2.DeleteNetworkAclInput, ...func(*ec2.Options)) (
		*ec2.DeleteNetworkAclOutput, error)
	DetachInternetGateway(context.Context, *ec2.DetachInternetGatewayInput, ...func(*ec2.Options)) (
		*ec2.DetachInternetGatewayOutput, error)
	DeleteInternetGateway(context.Context, *ec2.DeleteInternetGatewayInput, ...func(*ec2.Options)) (
		*ec2.DeleteInternetGatewayOutput, error)
	DeleteEgressOnlyInternetGateway(
		context.Context, *ec2.DeleteEgressOnlyInternetGatewayInput, ...func(*ec2.Options),
	) (*ec2.DeleteEgressOnlyInternetGatewayOutput, error)
	DeleteFlowLogs(context.Context, *ec2.DeleteFlowLogsInput, ...func(*ec2.Options)) (*ec2.DeleteFlowLogsOutput, error)
	DeleteVpc(context.Context, *ec2.DeleteVpcInput, ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)
}
