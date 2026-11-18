/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

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

package tgw

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// propagation is one route table an attachment propagates its routes to.
type propagation struct {
	// RouteTableID is the ID of the route table.
	RouteTableID string `json:"route_table_id"`

	// RouteTableName is the Name tag of the route table.
	RouteTableName string `json:"route_table_name,omitempty"`

	// State is enabled, or enabling, disabling or disabled.
	State string `json:"state"`
}

// cell returns the propagation as one line of the Propagated RTs column: the route table ID, then
// its name and its state, when it is not enabled, in parentheses.
func (p *propagation) cell() string {
	var extra []string
	if p.RouteTableName != "" {
		extra = append(extra, p.RouteTableName)
	}
	if p.State != string(types.TransitGatewayPropagationStateEnabled) {
		extra = append(extra, p.State)
	}
	if len(extra) == 0 {
		return p.RouteTableID
	}
	return fmt.Sprintf("%s (%s)", p.RouteTableID, strings.Join(extra, ", "))
}

// addPropagations fills the propagations of every row. AWS reads them one attachment or one route
// table at a time, with no batch call, so awss picks the side with fewer calls: one call per
// attachment when there are fewer attachments than route tables (a single VPC is one call),
// otherwise one call per route table. tables are the route tables of the transit gateways of the
// rows, and names maps a route table ID to its name. Only the attachments of a transit gateway
// whose route tables the profile sees are read: an attachment to a transit gateway another account
// shares is not, as AWS would refuse it. A failed call is recorded; the rows are kept.
func (r *AttachmentResults) addPropagations(
	ctx context.Context, client ec2API, tables []types.TransitGatewayRouteTable, names map[string]string,
) {
	owned := map[string]bool{}
	tableIDs := make([]string, 0, len(tables))
	for i := range tables {
		owned[aws.ToString(tables[i].TransitGatewayId)] = true
		tableIDs = append(tableIDs, aws.ToString(tables[i].TransitGatewayRouteTableId))
	}
	var attIDs []string
	for i := range r.Data {
		if owned[r.Data[i].TransitGatewayID] {
			attIDs = append(attIDs, r.Data[i].ID)
		}
	}
	var byAttachment map[string][]propagation
	var msgs []string
	if len(attIDs) < len(tableIDs) {
		byAttachment, msgs = propagationsByAttachment(ctx, client, attIDs)
	} else {
		byAttachment, msgs = propagationsByTable(ctx, client, tableIDs)
	}
	r.Errors = append(r.Errors, msgs...)
	for i := range r.Data {
		props := byAttachment[r.Data[i].ID]
		for j := range props {
			props[j].RouteTableName = names[props[j].RouteTableID]
		}
		slices.SortFunc(props, func(a, b propagation) int {
			return cmp.Or(cmp.Compare(a.RouteTableName, b.RouteTableName), cmp.Compare(a.RouteTableID, b.RouteTableID))
		})
		r.Data[i].Propagations = props
		for j := range props {
			r.Data[i].PropagatedRTs = append(r.Data[i].PropagatedRTs, props[j].cell())
		}
	}
}

// propagationsByAttachment returns the propagations of each attachment of ids, keyed by
// attachment ID, and the error messages.
func propagationsByAttachment(
	ctx context.Context, client ec2API, ids []string,
) (byAttachment map[string][]propagation, msgs []string) {
	return readEach(ctx, ids, func(ctx context.Context, id string, out map[string][]propagation) error {
		in := &ec2.GetTransitGatewayAttachmentPropagationsInput{
			TransitGatewayAttachmentId: aws.String(id), MaxResults: aws.Int32(pageSize),
		}
		paginator := ec2.NewGetTransitGatewayAttachmentPropagationsPaginator(client, in)
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("reading the propagations of %s: %w", id, err)
			}
			for _, p := range page.TransitGatewayAttachmentPropagations {
				out[id] = append(out[id], propagation{
					RouteTableID: aws.ToString(p.TransitGatewayRouteTableId), State: string(p.State),
				})
			}
		}
		return nil
	})
}

// propagationsByTable returns the propagations into each route table of ids, keyed by attachment
// ID, and the error messages.
func propagationsByTable(
	ctx context.Context, client ec2API, ids []string,
) (byAttachment map[string][]propagation, msgs []string) {
	return readEach(ctx, ids, func(ctx context.Context, id string, out map[string][]propagation) error {
		in := &ec2.GetTransitGatewayRouteTablePropagationsInput{
			TransitGatewayRouteTableId: aws.String(id), MaxResults: aws.Int32(pageSize),
		}
		paginator := ec2.NewGetTransitGatewayRouteTablePropagationsPaginator(client, in)
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("reading the propagations into %s: %w", id, err)
			}
			for _, p := range page.TransitGatewayRouteTablePropagations {
				att := aws.ToString(p.TransitGatewayAttachmentId)
				out[att] = append(out[att], propagation{RouteTableID: id, State: string(p.State)})
			}
		}
		return nil
	})
}

// readEach calls read for each ID, which adds the propagations it reads to the map, and returns
// the map and the error messages. A failed ID is recorded and the next one is read, unless the
// profile may not read propagations at all or the run is over.
func readEach(
	ctx context.Context, ids []string, read func(context.Context, string, map[string][]propagation) error,
) (byAttachment map[string][]propagation, msgs []string) {
	byAttachment = map[string][]propagation{}
	for _, id := range ids {
		err := read(ctx, id, byAttachment)
		if err == nil {
			continue
		}
		msgs = append(msgs, err.Error())
		if isUnauthorized(err) || ctx.Err() != nil {
			break
		}
	}
	return byAttachment, msgs
}
