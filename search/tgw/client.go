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
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

// pageSize is the page size of every call: AWS returns 1000 at most.
const pageSize int32 = 1000

// errCodeUnauthorized is the error code of an EC2 call the profile may not make.
const errCodeUnauthorized = "UnauthorizedOperation"

// isUnauthorized reports whether err says the profile may not make the call.
func isUnauthorized(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == errCodeUnauthorized
}

// ec2API is the part of the EC2 client used by the search.
type ec2API interface {
	ec2.DescribeTransitGatewayRouteTablesAPIClient
	ec2.DescribeTransitGatewayAttachmentsAPIClient
	ec2.SearchTransitGatewayRoutesAPIClient
}

// routeTables lists the Transit Gateway route tables of the region that --route-table-ids and
// --transit-gateway-ids ask for. The IDs are AWS filters, so an ID of another region or account is
// left out, not an error.
func routeTables(ctx context.Context, client ec2API, q *query) ([]types.TransitGatewayRouteTable, error) {
	in := &ec2.DescribeTransitGatewayRouteTablesInput{MaxResults: aws.Int32(pageSize)}
	in.Filters = appendFilter(in.Filters, "transit-gateway-route-table-id", q.tableIDs)
	in.Filters = appendFilter(in.Filters, "transit-gateway-id", q.tgwID)

	var out []types.TransitGatewayRouteTable
	paginator := ec2.NewDescribeTransitGatewayRouteTablesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing the transit gateway route tables: %w", err)
		}
		out = append(out, page.TransitGatewayRouteTables...)
	}
	return out, nil
}

// attachments lists the attachments of the transit gateways tgwIDs.
func attachments(ctx context.Context, client ec2API, tgwIDs []string) ([]types.TransitGatewayAttachment, error) {
	in := &ec2.DescribeTransitGatewayAttachmentsInput{MaxResults: aws.Int32(pageSize)}
	in.Filters = appendFilter(in.Filters, "transit-gateway-id", tgwIDs)

	var out []types.TransitGatewayAttachment
	paginator := ec2.NewDescribeTransitGatewayAttachmentsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing the transit gateway attachments: %w", err)
		}
		out = append(out, page.TransitGatewayAttachments...)
	}
	return out, nil
}

// searchRoutes returns the routes of one route table that the filters keep, following every page,
// and reports whether AWS said more routes exist than it returned.
func searchRoutes(
	ctx context.Context, client ec2API, tableID string, filters []types.Filter,
) (routes []types.TransitGatewayRoute, truncated bool, err error) {
	in := &ec2.SearchTransitGatewayRoutesInput{
		TransitGatewayRouteTableId: aws.String(tableID), Filters: filters, MaxResults: aws.Int32(pageSize),
	}
	paginator := ec2.NewSearchTransitGatewayRoutesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("searching the routes of %s: %w", tableID, err)
		}
		routes = append(routes, page.Routes...)
		truncated = aws.ToBool(page.AdditionalRoutesAvailable) && aws.ToString(page.NextToken) == ""
	}
	return routes, truncated, nil
}
