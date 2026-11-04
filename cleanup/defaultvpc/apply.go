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
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

// maxEndpointsPerCall is how many VPC endpoints one DeleteVpcEndpoints call takes.
const maxEndpointsPerCall = 25

// step is one stage of a deletion, in order.
type step struct {
	name string
	fn   func(context.Context, ec2API, *inventory) error
}

// steps returns the stages of a deletion in the order AWS needs: what holds ENIs or routes first,
// the VPC last.
func (r *Results) steps() []step {
	return []step{
		{"delete the VPC endpoints", deleteEndpoints},
		{"delete the NAT gateways", deleteNATs},
		{"wait for the VPC endpoints to be deleted", r.waitEndpoints},
		{"wait for the NAT gateways to be deleted", r.waitNATs},
		{"release the Elastic IPs of the NAT gateways", r.releaseNATAddresses},
		{"delete the peering connections", deletePeerings},
		{"delete the Transit Gateway attachments", deleteTGWAttachments},
		{"wait for the Transit Gateway attachments to be deleted", r.waitTGWAttachments},
		{"detach the VPN gateways", detachVPNGateways},
		{"wait for the VPN gateways to be detached", r.waitVPNGateways},
		{"delete the detached ENIs", deleteENIs},
		{"wait for the ENIs to be released", r.waitENIs},
		{"delete the security groups", deleteSecurityGroups},
		{"delete the subnets", deleteSubnets},
		{"delete the route tables", deleteRouteTables},
		{"delete the network ACLs", deleteNetworkACLs},
		{"delete the internet gateways", deleteInternetGateways},
		{"delete the egress-only internet gateways", deleteEgressOnly},
		{"delete the flow logs", deleteFlowLogs},
		{"delete the VPC", deleteVPC},
	}
}

// apply deletes everything in inv, then the VPC. It stops at the first step that fails and
// returns the details of the failure; what was deleted stays deleted, and a new run resumes from
// what is left.
func (r *Results) apply(ctx context.Context, c ec2API, inv *inventory) []string {
	for _, s := range r.steps() {
		if err := s.fn(ctx, c, inv); err != nil {
			return []string{"failed step: " + s.name, "error: " + conciseError(err)}
		}
	}
	return nil
}

// conciseError returns the message of err with the SDK's operation error, which names the
// operation, the HTTP status and the request ID before the AWS error, cut down to "Code: message":
// the table has room for one line per detail.
func conciseError(err error) string {
	msg := err.Error()
	var opErr *smithy.OperationError
	var apiErr smithy.APIError
	if errors.As(err, &opErr) && errors.As(err, &apiErr) {
		msg = strings.Replace(msg, opErr.Error(), apiErr.ErrorCode()+": "+apiErr.ErrorMessage(), 1)
	}
	return msg
}

// isNotFound reports whether err says the resource is already gone, or already detached: the
// deletion of that resource is done.
func isNotFound(err error) bool {
	code := errorCode(err)
	return strings.HasSuffix(code, "NotFound") || code == "Gateway.NotAttached"
}

// ignoreNotFound returns err, or nil when it says the resource is already gone.
func ignoreNotFound(err error) error {
	if err == nil || isNotFound(err) {
		return nil
	}
	return err
}

// unsuccessful returns an error for the items AWS could not delete, ignoring the ones already gone.
func unsuccessful(items []types.UnsuccessfulItem) error {
	var msgs []string
	for _, item := range items {
		if item.Error == nil || strings.HasSuffix(aws.ToString(item.Error.Code), "NotFound") {
			continue
		}
		msgs = append(msgs, fmt.Sprintf("%s: %s", aws.ToString(item.ResourceId), aws.ToString(item.Error.Message)))
	}
	if len(msgs) == 0 {
		return nil
	}
	return errors.New(strings.Join(msgs, "; "))
}

// each calls fn for every id and returns the first error that does not say the resource is gone.
func each(ids []string, fn func(id string) error) error {
	for _, id := range ids {
		if err := ignoreNotFound(fn(id)); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
	}
	return nil
}

// deleteEndpoints starts the deletion of the VPC endpoints, at most maxEndpointsPerCall per call.
func deleteEndpoints(ctx context.Context, c ec2API, inv *inventory) error {
	for chunk := range slices.Chunk(inv.endpoints, maxEndpointsPerCall) {
		out, err := c.DeleteVpcEndpoints(ctx, &ec2.DeleteVpcEndpointsInput{VpcEndpointIds: chunk})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("deleting %s: %w", strings.Join(chunk, ", "), err)
		}
		if out != nil {
			if err := unsuccessful(out.Unsuccessful); err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteNATs starts the deletion of the NAT gateways.
func deleteNATs(ctx context.Context, c ec2API, inv *inventory) error {
	for _, n := range inv.nats {
		if n.deleted {
			continue
		}
		_, err := c.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{NatGatewayId: aws.String(n.id)})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("%s: %w", n.id, err)
		}
	}
	return nil
}

// releaseNATAddresses releases the Elastic IPs of the NAT gateways, once the gateways are gone.
// Only the IPs that belonged to those gateways are released.
func (r *Results) releaseNATAddresses(ctx context.Context, c ec2API, inv *inventory) error {
	for _, n := range inv.nats {
		for _, a := range n.addresses {
			if a.awsManaged {
				continue
			}
			if err := r.releaseAddress(ctx, c, a.allocationID); err != nil {
				return fmt.Errorf("%s: %w", a.allocationID, err)
			}
		}
	}
	return nil
}

// releaseAddress releases one Elastic IP of a deleted NAT gateway. AWS may keep the IP associated
// a little after the gateway is gone, so a release that fails is tried again while the IP still
// exists; an IP already gone, released by AWS or by hand, counts as released. A permission error
// fails at once.
func (r *Results) releaseAddress(ctx context.Context, c ec2API, id string) error {
	var last error
	err := r.waitUntil(ctx, "Elastic IP "+id, func() (bool, error) {
		_, err := c.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(id)})
		if err == nil || isNotFound(err) {
			return true, nil
		}
		if errorCode(err) == "UnauthorizedOperation" {
			return false, fmt.Errorf("releasing: %w", err)
		}
		last = err
		eips, err := lookupAddresses(ctx, c, []string{id})
		if err != nil {
			return false, err
		}
		_, exists := eips[id]
		return !exists, nil
	})
	if err != nil && last != nil {
		return fmt.Errorf("%w; the last release error: %w", err, last)
	}
	return err
}

// deletePeerings deletes the peering connections, or rejects the requests this VPC was asked to
// accept.
func deletePeerings(ctx context.Context, c ec2API, inv *inventory) error {
	for _, p := range inv.peerings {
		var err error
		if p.reject {
			_, err = c.RejectVpcPeeringConnection(ctx,
				&ec2.RejectVpcPeeringConnectionInput{VpcPeeringConnectionId: aws.String(p.id)})
		} else {
			_, err = c.DeleteVpcPeeringConnection(ctx,
				&ec2.DeleteVpcPeeringConnectionInput{VpcPeeringConnectionId: aws.String(p.id)})
		}
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("%s: %w", p.id, err)
		}
	}
	return nil
}

// deleteTGWAttachments starts the deletion of the VPC's Transit Gateway attachments. The Transit
// Gateways stay.
func deleteTGWAttachments(ctx context.Context, c ec2API, inv *inventory) error {
	for _, a := range inv.tgwAttachments {
		_, err := c.DeleteTransitGatewayVpcAttachment(ctx,
			&ec2.DeleteTransitGatewayVpcAttachmentInput{TransitGatewayAttachmentId: aws.String(a.id)})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("%s: %w", a.id, err)
		}
	}
	return nil
}

// detachVPNGateways starts detaching the VPN gateways from the VPC. The gateways and their VPN
// connections stay.
func detachVPNGateways(ctx context.Context, c ec2API, inv *inventory) error {
	return each(inv.vpnGateways, func(id string) error {
		_, err := c.DetachVpnGateway(ctx,
			&ec2.DetachVpnGatewayInput{VpnGatewayId: aws.String(id), VpcId: aws.String(inv.vpcID)})
		return err
	})
}

// deleteENIs deletes the detached ENIs of the account.
func deleteENIs(ctx context.Context, c ec2API, inv *inventory) error {
	return each(inv.enis, func(id string) error {
		_, err := c.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: aws.String(id)})
		return err
	})
}

// deleteSecurityGroups removes the rules that reference the security groups to delete, from every
// group of the VPC, then deletes the groups: two groups that reference each other cannot be
// deleted otherwise.
func deleteSecurityGroups(ctx context.Context, c ec2API, inv *inventory) error {
	targets := inv.customSecurityGroups()
	for i := range inv.securityGroups {
		if err := revokeReferences(ctx, c, &inv.securityGroups[i], targets); err != nil {
			return err
		}
	}
	return each(targets, func(id string) error {
		_, err := c.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{GroupId: aws.String(id)})
		return err
	})
}

// revokeReferences revokes the inbound and outbound rules of sg that reference one of targets.
func revokeReferences(ctx context.Context, c ec2API, sg *types.SecurityGroup, targets []string) error {
	id := aws.ToString(sg.GroupId)
	if in := referencing(sg.IpPermissions, targets); len(in) > 0 {
		_, err := c.RevokeSecurityGroupIngress(ctx,
			&ec2.RevokeSecurityGroupIngressInput{GroupId: aws.String(id), IpPermissions: in})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("revoking the inbound rules of %s: %w", id, err)
		}
	}
	if out := referencing(sg.IpPermissionsEgress, targets); len(out) > 0 {
		_, err := c.RevokeSecurityGroupEgress(ctx,
			&ec2.RevokeSecurityGroupEgressInput{GroupId: aws.String(id), IpPermissions: out})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("revoking the outbound rules of %s: %w", id, err)
		}
	}
	return nil
}

// referencing returns the parts of perms that reference one of targets, as rules to revoke.
func referencing(perms []types.IpPermission, targets []string) []types.IpPermission {
	var out []types.IpPermission
	for i := range perms {
		p := &perms[i]
		pairs := slices.DeleteFunc(slices.Clone(p.UserIdGroupPairs), func(pair types.UserIdGroupPair) bool {
			return !slices.Contains(targets, aws.ToString(pair.GroupId))
		})
		if len(pairs) > 0 {
			out = append(out, types.IpPermission{
				IpProtocol: p.IpProtocol, FromPort: p.FromPort, ToPort: p.ToPort, UserIdGroupPairs: pairs,
			})
		}
	}
	return out
}

// deleteSubnets deletes the subnets. Deleting a subnet also removes its route table and network
// ACL associations.
func deleteSubnets(ctx context.Context, c ec2API, inv *inventory) error {
	return each(inv.subnets, func(id string) error {
		_, err := c.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: aws.String(id)})
		return err
	})
}

// deleteRouteTables removes the associations left, such as a gateway association, then deletes
// the route tables.
func deleteRouteTables(ctx context.Context, c ec2API, inv *inventory) error {
	for _, rt := range inv.routeTables {
		err := each(rt.associations, func(id string) error {
			_, err := c.DisassociateRouteTable(ctx, &ec2.DisassociateRouteTableInput{AssociationId: aws.String(id)})
			return err
		})
		if err != nil {
			return err
		}
	}
	return each(routeTableIDs(inv.routeTables), func(id string) error {
		_, err := c.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{RouteTableId: aws.String(id)})
		return err
	})
}

// deleteNetworkACLs deletes the network ACLs, once their subnets are gone.
func deleteNetworkACLs(ctx context.Context, c ec2API, inv *inventory) error {
	return each(inv.networkACLs, func(id string) error {
		_, err := c.DeleteNetworkAcl(ctx, &ec2.DeleteNetworkAclInput{NetworkAclId: aws.String(id)})
		return err
	})
}

// deleteInternetGateways detaches the internet gateways from the VPC and deletes them.
func deleteInternetGateways(ctx context.Context, c ec2API, inv *inventory) error {
	return each(inv.internetGateways, func(id string) error {
		_, err := c.DetachInternetGateway(ctx,
			&ec2.DetachInternetGatewayInput{InternetGatewayId: aws.String(id), VpcId: aws.String(inv.vpcID)})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("detaching: %w", err)
		}
		_, err = c.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{InternetGatewayId: aws.String(id)})
		return err
	})
}

// deleteEgressOnly deletes the egress-only internet gateways.
func deleteEgressOnly(ctx context.Context, c ec2API, inv *inventory) error {
	return each(inv.egressOnly, func(id string) error {
		_, err := c.DeleteEgressOnlyInternetGateway(ctx,
			&ec2.DeleteEgressOnlyInternetGatewayInput{EgressOnlyInternetGatewayId: aws.String(id)})
		return err
	})
}

// deleteFlowLogs deletes the flow logs of the VPC. The logs they stored stay.
func deleteFlowLogs(ctx context.Context, c ec2API, inv *inventory) error {
	if len(inv.flowLogs) == 0 {
		return nil
	}
	out, err := c.DeleteFlowLogs(ctx, &ec2.DeleteFlowLogsInput{FlowLogIds: inv.flowLogs})
	if err := ignoreNotFound(err); err != nil {
		return fmt.Errorf("deleting %s: %w", strings.Join(inv.flowLogs, ", "), err)
	}
	if out == nil {
		return nil
	}
	return unsuccessful(out.Unsuccessful)
}

// deleteVPC deletes the VPC, the last step.
func deleteVPC(ctx context.Context, c ec2API, inv *inventory) error {
	_, err := c.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String(inv.vpcID)})
	if err := ignoreNotFound(err); err != nil {
		return fmt.Errorf("%s: %w", inv.vpcID, err)
	}
	return nil
}

// waitUntil calls done until it reports true, every pollInterval, for at most waitLimit or until
// ctx ends.
func (r *Results) waitUntil(ctx context.Context, what string, done func() (bool, error)) error {
	deadline := time.Now().Add(waitLimit)
	for {
		ok, err := done()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s still there after %s; run again later", what, waitLimit)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for %s: %w", what, ctx.Err())
		case <-time.After(r.pollInterval):
		}
	}
}

// waitEndpoints waits until the VPC has no endpoint left.
func (r *Results) waitEndpoints(ctx context.Context, c ec2API, inv *inventory) error {
	if len(inv.endpoints) == 0 {
		return nil
	}
	probe := &inventory{vpcID: inv.vpcID}
	return r.waitUntil(ctx, "VPC endpoints", func() (bool, error) {
		err := listEndpoints(ctx, c, probe)
		return len(probe.endpoints) == 0, err
	})
}

// waitNATs waits until the VPC has no NAT gateway left.
func (r *Results) waitNATs(ctx context.Context, c ec2API, inv *inventory) error {
	if len(inv.nats) == 0 {
		return nil
	}
	probe := &inventory{vpcID: inv.vpcID}
	return r.waitUntil(ctx, "NAT gateways", func() (bool, error) {
		err := listNATs(ctx, c, probe)
		return !slices.ContainsFunc(probe.nats, func(n natGateway) bool { return !n.deleted }), err
	})
}

// waitTGWAttachments waits until the VPC has no Transit Gateway attachment left.
func (r *Results) waitTGWAttachments(ctx context.Context, c ec2API, inv *inventory) error {
	if len(inv.tgwAttachments) == 0 {
		return nil
	}
	probe := &inventory{vpcID: inv.vpcID}
	return r.waitUntil(ctx, "Transit Gateway attachments", func() (bool, error) {
		err := listTGWAttachments(ctx, c, probe)
		return len(probe.tgwAttachments) == 0, err
	})
}

// waitVPNGateways waits until no VPN gateway is attached to the VPC, or detaching from it.
func (r *Results) waitVPNGateways(ctx context.Context, c ec2API, inv *inventory) error {
	if len(inv.vpnGateways) == 0 {
		return nil
	}
	probe := &inventory{vpcID: inv.vpcID}
	return r.waitUntil(ctx, "VPN gateway attachments", func() (bool, error) {
		err := listVPNGateways(ctx, c, probe)
		return len(probe.vpnGateways) == 0, err
	})
}

// waitENIs waits until the VPC has no ENI left: AWS removes the ENIs of the endpoints, NAT
// gateways and Transit Gateway attachments a little after them, and a subnet with an ENI cannot be
// deleted. A new blocker, such as an instance launched during the run, fails the wait at once.
func (r *Results) waitENIs(ctx context.Context, c ec2API, inv *inventory) error {
	return r.waitUntil(ctx, "ENIs", func() (bool, error) {
		enis, err := all(ctx, ec2.NewDescribeNetworkInterfacesPaginator(c, &ec2.DescribeNetworkInterfacesInput{
			Filters: filter("vpc-id", inv.vpcID), MaxResults: aws.Int32(maxResults),
		}), func(o *ec2.DescribeNetworkInterfacesOutput) []types.NetworkInterface { return o.NetworkInterfaces })
		if err != nil {
			return false, fmt.Errorf("describing the network interfaces: %w", err)
		}
		for i := range enis {
			if blocker, _ := classifyENI(&enis[i]); blocker != "" {
				return false, fmt.Errorf("new blocker: %s", blocker)
			}
		}
		return len(enis) == 0, nil
	})
}
