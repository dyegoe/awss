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

const (
	// maxResults is the page size of the Describe calls that accept 1000, the largest AWS allows.
	// AWS recommends paginated calls only, and the SDK paginator sets no page size by itself.
	maxResults int32 = 1000

	// maxResultsSmall is the page size of the calls that accept at most 100.
	maxResultsSmall int32 = 100

	// maxResultsEgressOnly is the largest page DescribeEgressOnlyInternetGateways accepts.
	maxResultsEgressOnly int32 = 255
)

// natGateway is a NAT gateway to delete, with the Elastic IPs to release after it is gone.
type natGateway struct {
	id            string
	allocationIDs []string
	publicIPs     []string
}

// peering is a peering connection of the VPC. reject is set for a request this VPC was asked to
// accept: only the requester may delete a pending request; the accepter rejects it.
type peering struct {
	id, peerVpc, peerAccount string
	reject                   bool
}

// tgwAttachment is the attachment of the VPC to a Transit Gateway, which stays.
type tgwAttachment struct {
	id, tgwID string
}

// routeTable is a route table that is not the main one, with its associations.
type routeTable struct {
	id           string
	associations []string
}

// inventory is everything inside one VPC that a cleanup deletes.
type inventory struct {
	vpcID            string
	enis             []string
	endpoints        []string
	nats             []natGateway
	peerings         []peering
	tgwAttachments   []tgwAttachment
	vpnGateways      []string
	securityGroups   []types.SecurityGroup
	subnets          []string
	routeTables      []routeTable
	networkACLs      []string
	internetGateways []string
	egressOnly       []string
	flowLogs         []string
}

// pager is an SDK paginator that returns pages of type O.
type pager[O any] interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*ec2.Options)) (O, error)
}

// all follows every page of p and returns the items that items picks from each page.
func all[O, T any](ctx context.Context, p pager[O], items func(O) []T) ([]T, error) {
	var out []T
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, items(page)...)
	}
	return out, nil
}

// filter returns one EC2 filter of name and value.
func filter(name, value string) []types.Filter {
	return []types.Filter{{Name: aws.String(name), Values: []string{value}}}
}

// plumbingENIs are the ENI types that belong to the plumbing the cleanup deletes: AWS removes
// them with their owner.
//
// The SDK enum spells the NAT gateway type "natGateway", while the EC2 API documents
// "nat_gateway": both are accepted, so a NAT gateway never reads as a workload.
var plumbingENIs = []types.NetworkInterfaceType{
	types.NetworkInterfaceTypeVpcEndpoint,
	types.NetworkInterfaceTypeGatewayLoadBalancerEndpoint,
	types.NetworkInterfaceTypeNatGateway,
	"nat_gateway",
	types.NetworkInterfaceTypeTransitGateway,
}

// listBlockers lists the ENIs of the VPC and returns the blockers, named after what created
// them, and the detached ENIs the account owns, which the cleanup deletes.
func listBlockers(ctx context.Context, c ec2API, vpcID string) (blockers, owned []string, err error) {
	enis, err := all(ctx, ec2.NewDescribeNetworkInterfacesPaginator(c, &ec2.DescribeNetworkInterfacesInput{
		Filters: filter("vpc-id", vpcID), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeNetworkInterfacesOutput) []types.NetworkInterface { return o.NetworkInterfaces })
	if err != nil {
		return nil, nil, fmt.Errorf("error describing the network interfaces of %s: %w", vpcID, err)
	}
	for i := range enis {
		blocker, own := classifyENI(&enis[i])
		if blocker != "" {
			blockers = append(blockers, blocker)
		}
		if own {
			owned = append(owned, aws.ToString(enis[i].NetworkInterfaceId))
		}
	}
	return blockers, owned, nil
}

// classifyENI returns why eni blocks the deletion, or "" when it does not, and whether it is a
// detached ENI of the account that the cleanup deletes.
func classifyENI(eni *types.NetworkInterface) (blocker string, owned bool) {
	if slices.Contains(plumbingENIs, eni.InterfaceType) {
		return "", false
	}
	id := aws.ToString(eni.NetworkInterfaceId)
	if eni.Status == types.NetworkInterfaceStatusAvailable {
		if aws.ToBool(eni.RequesterManaged) {
			return fmt.Sprintf("AWS-managed detached ENI %s (%s), retry later", id, aws.ToString(eni.Description)), false
		}
		return "", true
	}
	return eniOwner(eni, id), false
}

// eniOwner names what uses eni: its instance, its type, or its description.
func eniOwner(eni *types.NetworkInterface, id string) string {
	desc := aws.ToString(eni.Description)
	switch {
	case eni.Attachment != nil && aws.ToString(eni.Attachment.InstanceId) != "":
		return fmt.Sprintf("EC2 instance %s (%s)", aws.ToString(eni.Attachment.InstanceId), id)
	case eni.InterfaceType == types.NetworkInterfaceTypeLambda:
		return fmt.Sprintf("Lambda: %s (%s)", desc, id)
	case eni.InterfaceType != "" && eni.InterfaceType != types.NetworkInterfaceTypeInterface:
		return strings.TrimSuffix(fmt.Sprintf("%s: %s", eni.InterfaceType, desc), ": ") + " (" + id + ")"
	case desc != "":
		return fmt.Sprintf("%s (%s)", desc, id)
	default:
		return fmt.Sprintf("ENI %s in use", id)
	}
}

// lister fills one part of an inventory.
type lister struct {
	what string
	fn   func(context.Context, ec2API, *inventory) error
}

// listers lists every part of an inventory but the ENIs, which listBlockers handles.
var listers = []lister{
	{"vpc endpoints", listEndpoints},
	{"nat gateways", listNATs},
	{"peering connections", listPeerings},
	{"transit gateway attachments", listTGWAttachments},
	{"vpn gateways", listVPNGateways},
	{"security groups", listSecurityGroups},
	{"subnets", listSubnets},
	{"route tables", listRouteTables},
	{"network acls", listNetworkACLs},
	{"internet gateways", listInternetGateways},
	{"egress-only internet gateways", listEgressOnly},
	{"flow logs", listFlowLogs},
}

// listInventory lists everything inside the VPC that a cleanup deletes, but the ENIs.
func listInventory(ctx context.Context, c ec2API, vpcID string) (*inventory, error) {
	inv := &inventory{vpcID: vpcID}
	for _, l := range listers {
		if err := l.fn(ctx, c, inv); err != nil {
			return nil, fmt.Errorf("error describing the %s of %s: %w", l.what, vpcID, err)
		}
	}
	return inv, nil
}

// isGone reports whether a resource state means it no longer exists. AWS spells some states in
// lower case and others capitalised.
func isGone(state string, gone ...string) bool {
	for _, g := range gone {
		if strings.EqualFold(state, g) {
			return true
		}
	}
	return false
}

// listEndpoints lists the VPC endpoints that still exist.
func listEndpoints(ctx context.Context, c ec2API, inv *inventory) error {
	eps, err := all(ctx, ec2.NewDescribeVpcEndpointsPaginator(c, &ec2.DescribeVpcEndpointsInput{
		Filters: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeVpcEndpointsOutput) []types.VpcEndpoint { return o.VpcEndpoints })
	inv.endpoints = nil
	for i := range eps {
		if !isGone(string(eps[i].State), "deleted", "rejected", "failed", "expired") {
			inv.endpoints = append(inv.endpoints, aws.ToString(eps[i].VpcEndpointId))
		}
	}
	return err
}

// listNATs lists the NAT gateways that still exist, with their Elastic IPs.
func listNATs(ctx context.Context, c ec2API, inv *inventory) error {
	nats, err := all(ctx, ec2.NewDescribeNatGatewaysPaginator(c, &ec2.DescribeNatGatewaysInput{
		Filter: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeNatGatewaysOutput) []types.NatGateway { return o.NatGateways })
	inv.nats = nil
	for i := range nats {
		if nats[i].State == types.NatGatewayStateDeleted {
			continue
		}
		nat := natGateway{id: aws.ToString(nats[i].NatGatewayId)}
		for _, a := range nats[i].NatGatewayAddresses {
			if a.AllocationId != nil {
				nat.allocationIDs = append(nat.allocationIDs, *a.AllocationId)
				nat.publicIPs = append(nat.publicIPs, aws.ToString(a.PublicIp))
			}
		}
		inv.nats = append(inv.nats, nat)
	}
	return err
}

// listPeerings lists the peering connections of the VPC, as requester and as accepter.
func listPeerings(ctx context.Context, c ec2API, inv *inventory) error {
	var pcxs []types.VpcPeeringConnection
	for _, side := range []string{"requester-vpc-info.vpc-id", "accepter-vpc-info.vpc-id"} {
		page, err := all(ctx, ec2.NewDescribeVpcPeeringConnectionsPaginator(c, &ec2.DescribeVpcPeeringConnectionsInput{
			Filters: filter(side, inv.vpcID), MaxResults: aws.Int32(maxResults),
		}), func(o *ec2.DescribeVpcPeeringConnectionsOutput) []types.VpcPeeringConnection {
			return o.VpcPeeringConnections
		})
		if err != nil {
			return err
		}
		pcxs = append(pcxs, page...)
	}
	inv.peerings = nil
	for i := range pcxs {
		if p, ok := parsePeering(&pcxs[i], inv.vpcID); ok {
			inv.peerings = append(inv.peerings, p)
		}
	}
	return nil
}

// parsePeering returns the peering connection pcx of vpcID, and false when it no longer exists.
func parsePeering(pcx *types.VpcPeeringConnection, vpcID string) (peering, bool) {
	code := types.VpcPeeringConnectionStateReasonCode("")
	if pcx.Status != nil {
		code = pcx.Status.Code
	}
	if isGone(string(code), "deleted", "deleting", "rejected", "failed", "expired") {
		return peering{}, false
	}
	p := peering{id: aws.ToString(pcx.VpcPeeringConnectionId)}
	peer := pcx.AccepterVpcInfo
	weAccept := pcx.AccepterVpcInfo != nil && aws.ToString(pcx.AccepterVpcInfo.VpcId) == vpcID
	if weAccept {
		peer = pcx.RequesterVpcInfo
	}
	if peer != nil {
		p.peerVpc, p.peerAccount = aws.ToString(peer.VpcId), aws.ToString(peer.OwnerId)
	}
	p.reject = weAccept && code == types.VpcPeeringConnectionStateReasonCodePendingAcceptance
	return p, true
}

// listTGWAttachments lists the Transit Gateway attachments of the VPC that still exist.
func listTGWAttachments(ctx context.Context, c ec2API, inv *inventory) error {
	atts, err := all(ctx, ec2.NewDescribeTransitGatewayVpcAttachmentsPaginator(c,
		&ec2.DescribeTransitGatewayVpcAttachmentsInput{
			Filters: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
		}), func(o *ec2.DescribeTransitGatewayVpcAttachmentsOutput) []types.TransitGatewayVpcAttachment {
		return o.TransitGatewayVpcAttachments
	})
	inv.tgwAttachments = nil
	for i := range atts {
		if !isGone(string(atts[i].State), "deleted", "rejected", "failed") {
			inv.tgwAttachments = append(inv.tgwAttachments, tgwAttachment{
				id: aws.ToString(atts[i].TransitGatewayAttachmentId), tgwID: aws.ToString(atts[i].TransitGatewayId),
			})
		}
	}
	return err
}

// listVPNGateways lists the VPN gateways attached to the VPC, or still detaching from it.
// DescribeVpnGateways is not paginated.
func listVPNGateways(ctx context.Context, c ec2API, inv *inventory) error {
	out, err := c.DescribeVpnGateways(ctx, &ec2.DescribeVpnGatewaysInput{Filters: filter("attachment.vpc-id", inv.vpcID)})
	if err != nil {
		return err
	}
	inv.vpnGateways = nil
	for i := range out.VpnGateways {
		for _, a := range out.VpnGateways[i].VpcAttachments {
			if aws.ToString(a.VpcId) == inv.vpcID && a.State != types.AttachmentStatusDetached {
				inv.vpnGateways = append(inv.vpnGateways, aws.ToString(out.VpnGateways[i].VpnGatewayId))
				break
			}
		}
	}
	return nil
}

// listSecurityGroups lists every security group of the VPC, the default one included: its rules
// may reference the others.
func listSecurityGroups(ctx context.Context, c ec2API, inv *inventory) error {
	sgs, err := all(ctx, ec2.NewDescribeSecurityGroupsPaginator(c, &ec2.DescribeSecurityGroupsInput{
		Filters: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeSecurityGroupsOutput) []types.SecurityGroup { return o.SecurityGroups })
	inv.securityGroups = sgs
	return err
}

// listSubnets lists the subnets of the VPC.
func listSubnets(ctx context.Context, c ec2API, inv *inventory) error {
	subnets, err := all(ctx, ec2.NewDescribeSubnetsPaginator(c, &ec2.DescribeSubnetsInput{
		Filters: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeSubnetsOutput) []types.Subnet { return o.Subnets })
	inv.subnets = ids(subnets, func(s *types.Subnet) *string { return s.SubnetId })
	return err
}

// listRouteTables lists the route tables of the VPC but the main one, which goes with the VPC.
func listRouteTables(ctx context.Context, c ec2API, inv *inventory) error {
	rts, err := all(ctx, ec2.NewDescribeRouteTablesPaginator(c, &ec2.DescribeRouteTablesInput{
		Filters: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResultsSmall),
	}), func(o *ec2.DescribeRouteTablesOutput) []types.RouteTable { return o.RouteTables })
	inv.routeTables = nil
	for i := range rts {
		rt := routeTable{id: aws.ToString(rts[i].RouteTableId)}
		main := false
		for _, a := range rts[i].Associations {
			main = main || aws.ToBool(a.Main)
			if !aws.ToBool(a.Main) && a.RouteTableAssociationId != nil {
				rt.associations = append(rt.associations, *a.RouteTableAssociationId)
			}
		}
		if !main {
			inv.routeTables = append(inv.routeTables, rt)
		}
	}
	return err
}

// listNetworkACLs lists the network ACLs of the VPC but the default one, which goes with the VPC.
func listNetworkACLs(ctx context.Context, c ec2API, inv *inventory) error {
	acls, err := all(ctx, ec2.NewDescribeNetworkAclsPaginator(c, &ec2.DescribeNetworkAclsInput{
		Filters: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResultsSmall),
	}), func(o *ec2.DescribeNetworkAclsOutput) []types.NetworkAcl { return o.NetworkAcls })
	inv.networkACLs = nil
	for i := range acls {
		if !aws.ToBool(acls[i].IsDefault) {
			inv.networkACLs = append(inv.networkACLs, aws.ToString(acls[i].NetworkAclId))
		}
	}
	return err
}

// listInternetGateways lists the internet gateways attached to the VPC.
func listInternetGateways(ctx context.Context, c ec2API, inv *inventory) error {
	igws, err := all(ctx, ec2.NewDescribeInternetGatewaysPaginator(c, &ec2.DescribeInternetGatewaysInput{
		Filters: filter("attachment.vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeInternetGatewaysOutput) []types.InternetGateway { return o.InternetGateways })
	inv.internetGateways = ids(igws, func(g *types.InternetGateway) *string { return g.InternetGatewayId })
	return err
}

// listEgressOnly lists the egress-only internet gateways attached to the VPC. The call has no
// VPC filter, so the attachments are matched here.
func listEgressOnly(ctx context.Context, c ec2API, inv *inventory) error {
	gws, err := all(ctx, ec2.NewDescribeEgressOnlyInternetGatewaysPaginator(c,
		&ec2.DescribeEgressOnlyInternetGatewaysInput{MaxResults: aws.Int32(maxResultsEgressOnly)}),
		func(o *ec2.DescribeEgressOnlyInternetGatewaysOutput) []types.EgressOnlyInternetGateway {
			return o.EgressOnlyInternetGateways
		})
	inv.egressOnly = nil
	for i := range gws {
		attached := slices.ContainsFunc(gws[i].Attachments, func(a types.InternetGatewayAttachment) bool {
			return aws.ToString(a.VpcId) == inv.vpcID
		})
		if attached {
			inv.egressOnly = append(inv.egressOnly, aws.ToString(gws[i].EgressOnlyInternetGatewayId))
		}
	}
	return err
}

// listFlowLogs lists the flow logs of the VPC.
func listFlowLogs(ctx context.Context, c ec2API, inv *inventory) error {
	logs, err := all(ctx, ec2.NewDescribeFlowLogsPaginator(c, &ec2.DescribeFlowLogsInput{
		Filter: filter("resource-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
	}), func(o *ec2.DescribeFlowLogsOutput) []types.FlowLog { return o.FlowLogs })
	inv.flowLogs = ids(logs, func(l *types.FlowLog) *string { return l.FlowLogId })
	return err
}

// ids returns the ID that id reads from every item.
func ids[T any](items []T, id func(*T) *string) []string {
	var out []string
	for i := range items {
		out = append(out, aws.ToString(id(&items[i])))
	}
	return out
}

// plan returns what a cleanup of inv deletes, one line per kind of resource. The lines with an
// effect that cannot be undone, or outside the account, say so.
func (inv *inventory) plan() []string {
	var lines []string
	add := func(kind string, ids []string) {
		if len(ids) > 0 {
			lines = append(lines, kind+": "+strings.Join(ids, ", "))
		}
	}
	add("VPC endpoints", inv.endpoints)
	for _, n := range inv.nats {
		line := "NAT gateway " + n.id
		if len(n.publicIPs) > 0 {
			line += ", Elastic IP " + strings.Join(n.publicIPs, ", ") + " released (permanent)"
		}
		lines = append(lines, line)
	}
	for _, p := range inv.peerings {
		lines = append(lines, fmt.Sprintf("peering %s to %s in account %s (its routes on that side stop working)",
			p.id, p.peerVpc, p.peerAccount))
	}
	for _, a := range inv.tgwAttachments {
		lines = append(lines, fmt.Sprintf("TGW attachment %s (Transit Gateway %s kept)", a.id, a.tgwID))
	}
	for _, v := range inv.vpnGateways {
		lines = append(lines, fmt.Sprintf("VPN gateway %s detached (the gateway and its VPN connections kept)", v))
	}
	add("detached ENIs", inv.enis)
	add("security groups", inv.customSecurityGroups())
	add("subnets", inv.subnets)
	add("route tables", routeTableIDs(inv.routeTables))
	add("network ACLs", inv.networkACLs)
	add("internet gateways", inv.internetGateways)
	add("egress-only internet gateways", inv.egressOnly)
	add("flow logs", inv.flowLogs)
	return append(lines, "VPC: "+inv.vpcID)
}

// customSecurityGroups returns the IDs of the security groups but the default one, which goes
// with the VPC.
func (inv *inventory) customSecurityGroups() []string {
	var ids []string
	for i := range inv.securityGroups {
		if aws.ToString(inv.securityGroups[i].GroupName) != "default" {
			ids = append(ids, aws.ToString(inv.securityGroups[i].GroupId))
		}
	}
	return ids
}

// routeTableIDs returns the IDs of rts.
func routeTableIDs(rts []routeTable) []string {
	var ids []string
	for _, rt := range rts {
		ids = append(ids, rt.id)
	}
	return ids
}
