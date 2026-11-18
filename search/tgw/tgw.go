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

// Package tgw contains the searches of the AWS Transit Gateways: the routes of their route tables
// (tgw.go) and their attachments (attachments.go).
//
// It implements the common.Results interface. A Transit Gateway is regional: the search runs once
// per profile and region, as the EC2 searches do. It answers where a CIDR goes behind a Transit
// Gateway, such as the legacy part of a network: Cloud WAN, then a Transit Gateway, then VPCs.
package tgw

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search/cloudwan"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Results describes the Transit Gateway routes found with one profile in one region.
type Results struct {
	common.BaseResults

	// Data contains the routes found: one row per attachment of each route.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// AccountNames maps an account ID to its name, for the Owner column. It is built once per run
	// and shared by every search of the run, so it is only read.
	AccountNames map[string]string `json:"-"`
}

// dataRow represents one attachment of one route of one Transit Gateway route table. A route
// without attachments, such as a blackhole, is one row with the attachment columns empty.
type dataRow struct {
	// Query is the searched CIDR the route matched; empty when no CIDR was searched.
	Query string `json:"query,omitempty" header:"Query" sort:"query"`

	// TransitGatewayID is the ID of the transit gateway of the route table.
	TransitGatewayID string `json:"transit_gateway_id,omitempty" header:"TGW ID" sort:"tgw-id"`

	// RouteTableID is the ID of the route table.
	RouteTableID string `json:"route_table_id,omitempty" header:"Route Table ID" sort:"route-table-id"`

	// RouteTableName is the Name tag of the route table.
	RouteTableName string `json:"route_table_name,omitempty" header:"Name" stack:"" sort:"route-table"`

	// CIDR is the destination of the route; empty for a prefix list route.
	CIDR common.CIDR `json:"cidr,omitempty" header:"CIDR" sort:"cidr"`

	// PrefixListID is the destination of a prefix list route.
	PrefixListID string `json:"prefix_list_id,omitempty" header:"Prefix List" stack:"" sort:"prefix-list"`

	// Type is propagated or static.
	Type string `json:"type,omitempty" header:"Type" sort:"type"`

	// State is active or blackhole.
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// ResourceType is the type of the attachment the route points to: vpc, vpn,
	// direct-connect-gateway, peering, connect...
	ResourceType string `json:"resource_type,omitempty" header:"Resource Type" sort:"resource-type"`

	// ResourceID is the resource of the attachment, such as the VPC ID.
	ResourceID string `json:"resource_id,omitempty" header:"Resource ID" stack:"" sort:"resource-id"`

	// AttachmentID is the ID of the attachment the route points to.
	AttachmentID string `json:"attachment_id,omitempty" header:"Attachment ID" sort:"attachment-id"`

	// AttachmentName is the Name tag of the attachment.
	AttachmentName string `json:"attachment_name,omitempty" header:"Name" stack:"" sort:"attachment"`

	// AssocRTID is the route table the attachment is associated with: the table that
	// routes the traffic coming from the attachment, where to look for the return route. Empty
	// when the attachment is not associated.
	AssocRTID string `json:"associated_route_table_id,omitempty" header:"Associated RT" sort:"associated-route-table-id"`

	// AssocRTName is the Name tag of the associated route table.
	AssocRTName string `json:"associated_route_table_name,omitempty" header:"Name" stack:"" sort:"associated-route-table"`

	// OwnerID is the account that owns the resource of the attachment, such as the VPC.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// OwnerName is the name of the owner account, from AccountNames; empty when it is unknown.
	OwnerName string `json:"owner_name,omitempty" header:"Name" stack:"" sort:"owner-name"`

	// AnnouncementID is the route table announcement of the route, for a route from a Cloud WAN
	// peering.
	AnnouncementID string `json:"announcement_id,omitempty"`
}

// New initiates and returns a new instance of Transit Gateway route results.
func New(profile, region string, filters map[string][]string, sortField string) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data:    []dataRow{},
		Filters: filters,
	}
}

// newClient builds the EC2 client of a profile and region. It is a variable so tests can replace
// the AWS config with a fake client.
var newClient = func(profile, region string) (ec2API, error) {
	cfg, err := common.AwsConfig(profile, region)
	if err != nil {
		return nil, fmt.Errorf("loading the aws config: %w", err)
	}
	return ec2.NewFromConfig(cfg), nil
}

// Search finds the routes of the Transit Gateway route tables of the profile and region.
func (r *Results) Search(ctx context.Context) {
	q, err := newQuery(r.Filters)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}
	client, err := newClient(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	r.collect(ctx, client, q)
}

// collect lists the route tables with client, searches the routes of each one --route-table-ids
// and --route-table-names keep, names the attachments and sorts the rows. A route table that fails
// is recorded and the next one is searched, unless the profile may not search routes at all.
func (r *Results) collect(ctx context.Context, client ec2API, q *query) {
	tables, err := routeTables(ctx, client, q)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}
	tableNames := make(map[string]string, len(tables))
	seen := map[dataRow]bool{}
	for i := range tables {
		base := parseTable(&tables[i])
		tableNames[base.RouteTableID] = base.RouteTableName
		if !q.keepsTable(&base) {
			continue
		}
		rows, err := r.tableRows(ctx, client, q, &base)
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
			if isUnauthorized(err) || ctx.Err() != nil {
				break
			}
			continue
		}
		for j := range rows {
			if !seen[rows[j]] {
				seen[rows[j]] = true
				r.Data = append(r.Data, rows[j])
			}
		}
	}
	r.nameAttachments(ctx, client, tableNames)

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// tableRows returns the rows of one route table, base being its route table columns: one call per
// CIDR and match mode. A longest search keeps, per CIDR, the most specific of the routes its exact
// and supernet-of calls found.
func (r *Results) tableRows(ctx context.Context, client ec2API, q *query, base *dataRow) ([]dataRow, error) {
	var rows []dataRow
	covering := map[string][]types.TransitGatewayRoute{}
	for _, s := range q.searches() {
		routes, truncated, err := searchRoutes(ctx, client, base.RouteTableID, s.filters(q))
		if err != nil {
			return nil, err
		}
		if truncated {
			r.Errors = append(r.Errors, fmt.Sprintf(
				"route table %s has more routes than AWS returns in one search; narrow it with --cidrs", base.RouteTableID))
		}
		if q.longest {
			covering[s.cidr] = append(covering[s.cidr], routes...)
			continue
		}
		for i := range routes {
			rows = append(rows, parseRoute(&routes[i], base, s.cidr)...)
		}
	}
	if q.longest {
		rows = longestRows(q.cidrs, covering, base)
	}
	return rows, nil
}

// longestRows returns, for each CIDR, the rows of the most specific of its covering routes: the
// routes the traffic to it takes.
func longestRows(cidrs []string, covering map[string][]types.TransitGatewayRoute, base *dataRow) []dataRow {
	var rows []dataRow
	for _, c := range cidrs {
		best := cloudwan.LongestMatches(covering[c], c,
			func(rt *types.TransitGatewayRoute) *string { return rt.DestinationCidrBlock })
		for i := range best {
			rows = append(rows, parseRoute(&best[i], base, c)...)
		}
	}
	return rows
}

// nameAttachments fills the attachment name, owner and associated route table of every row from
// the attachments of the transit gateways of the rows: one call for them all. tableNames maps every
// route table ID of the region to its name. A failed call is recorded and leaves the columns
// empty; the rows are kept.
func (r *Results) nameAttachments(ctx context.Context, client ec2API, tableNames map[string]string) {
	var tgwIDs []string
	for i := range r.Data {
		if r.Data[i].AttachmentID != "" && !slices.Contains(tgwIDs, r.Data[i].TransitGatewayID) {
			tgwIDs = append(tgwIDs, r.Data[i].TransitGatewayID)
		}
	}
	if len(tgwIDs) == 0 {
		return
	}
	atts, err := attachments(ctx, client, appendFilter(nil, awsFilterAttTGWID, tgwIDs))
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}
	byID := make(map[string]*types.TransitGatewayAttachment, len(atts))
	for i := range atts {
		byID[aws.ToString(atts[i].TransitGatewayAttachmentId)] = &atts[i]
	}
	for i := range r.Data {
		att, ok := byID[r.Data[i].AttachmentID]
		if !ok {
			continue
		}
		r.Data[i].AttachmentName = common.TagName(att.Tags)
		r.Data[i].OwnerID = aws.ToString(att.ResourceOwnerId)
		r.Data[i].OwnerName = r.AccountNames[r.Data[i].OwnerID]
		if a := att.Association; a != nil && a.State == types.TransitGatewayAssociationStateAssociated {
			r.Data[i].AssocRTID = aws.ToString(a.TransitGatewayRouteTableId)
			r.Data[i].AssocRTName = tableNames[r.Data[i].AssocRTID]
		}
	}
}

// parseTable returns the route table columns of a route table's rows.
func parseTable(t *types.TransitGatewayRouteTable) dataRow {
	return dataRow{
		TransitGatewayID: aws.ToString(t.TransitGatewayId),
		RouteTableID:     aws.ToString(t.TransitGatewayRouteTableId),
		RouteTableName:   common.TagName(t.Tags),
	}
}

// parseRoute converts a route into one row per attachment, on top of the route table columns base.
// query is the searched CIDR the route matched.
func parseRoute(route *types.TransitGatewayRoute, base *dataRow, query string) []dataRow {
	row := *base
	row.Query = query
	row.CIDR = common.CIDR(aws.ToString(route.DestinationCidrBlock))
	row.PrefixListID = aws.ToString(route.PrefixListId)
	row.Type = string(route.Type)
	row.State = string(route.State)
	row.AnnouncementID = aws.ToString(route.TransitGatewayRouteTableAnnouncementId)
	if len(route.TransitGatewayAttachments) == 0 {
		return []dataRow{row}
	}
	rows := make([]dataRow, 0, len(route.TransitGatewayAttachments))
	for _, a := range route.TransitGatewayAttachments {
		att := row
		att.AttachmentID = aws.ToString(a.TransitGatewayAttachmentId)
		att.ResourceType = string(a.ResourceType)
		att.ResourceID = aws.ToString(a.ResourceId)
		rows = append(rows, att)
	}
	return rows
}

// sortResults sorts the results by the given field. Rows equal on it stay in route table order:
// route table, then destination by address.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}
	slices.SortStableFunc(r.Data, func(a, b dataRow) int {
		if c := cmp.Or(
			cmp.Compare(a.RouteTableName, b.RouteTableName), cmp.Compare(a.RouteTableID, b.RouteTableID),
		); c != 0 {
			return c
		}
		switch {
		case common.CIDRLess(string(a.CIDR), string(b.CIDR)):
			return -1
		case common.CIDRLess(string(b.CIDR), string(a.CIDR)):
			return 1
		}
		return cmp.Or(cmp.Compare(a.PrefixListID, b.PrefixListID), cmp.Compare(a.AttachmentID, b.AttachmentID))
	})
	common.SortByField(r.Data, sortFields[field])
	return nil
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the `header` tag of the dataRow fields.
func (r *Results) GetHeaders() []interface{} { return common.Headers(dataRow{}) }

// GetRows returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} { return common.Rows(r.Data) }

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The sort fields are defined in the struct tag `sort` on dataRow.
// The function returns an error if the given field is not a valid sort field.
func GetSortFields(f string) (map[string]string, error) {
	return common.SortFields(dataRow{}, f)
}

// SortFieldNames returns the valid sort fields, sorted alphabetically.
func SortFieldNames() []string {
	return common.SortFieldNames(dataRow{})
}
