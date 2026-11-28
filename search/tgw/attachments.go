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
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Filter keys of the Transit Gateway attachments search only. It also takes FilterKeyAttachmentID,
// FilterKeyAttachmentType, FilterKeyResourceID and FilterKeyTransitGatewayID, filtered by AWS.
const (
	// FilterKeyAttachmentName keeps the attachments with these Name tags (globs), matched by awss.
	FilterKeyAttachmentName = "attachment-name"
	// FilterKeyAttachmentState keeps the attachments in these states, matched by awss: AWS writes
	// them in camel case (pendingAcceptance), awss in kebab case (pending-acceptance).
	FilterKeyAttachmentState = "attachment-state"
	// FilterKeyAssociatedRouteTableID keeps the attachments associated with these route tables,
	// filtered by AWS.
	FilterKeyAssociatedRouteTableID = "associated-route-table-id"
	// FilterKeyPropagatedRouteTableID keeps the attachments that propagate to these route tables,
	// in any propagation state, matched by awss. It reads the propagations, as Propagations does.
	FilterKeyPropagatedRouteTableID = "propagated-route-table-id"
)

// The DescribeTransitGatewayAttachments filters.
const (
	awsFilterAttID           = "transit-gateway-attachment-id"
	awsFilterAttResourceType = "resource-type"
	awsFilterAttResourceID   = "resource-id"
	awsFilterAttTGWID        = "transit-gateway-id"
	awsFilterAttAssociatedRT = "association.transit-gateway-route-table-id"
)

// AttachmentStateNames returns the attachment states, as awss writes them: available,
// pending-acceptance...
func AttachmentStateNames() []string {
	values := types.TransitGatewayAttachmentState("").Values()
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, camelKebab(string(v)))
	}
	return out
}

// camelKebab writes a camel case AWS value in kebab case: pendingAcceptance is pending-acceptance.
// A dash goes only where a lower case letter meets an upper case one, so a value already in kebab
// case, or in upper case (AVAILABLE), is kept as one word.
func camelKebab(s string) string {
	var b strings.Builder
	prev := rune(0)
	for _, c := range s {
		if unicode.IsUpper(c) && unicode.IsLower(prev) {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(c))
		prev = c
	}
	return b.String()
}

// CheckAttachmentFlags validates the attachment types and states and the name globs before any
// AWS call.
func CheckAttachmentFlags(filters map[string][]string) error {
	_, err := newAttachmentQuery(filters)
	return err
}

// AttachmentResults describes the Transit Gateway attachments found with one profile in one region.
type AttachmentResults struct {
	common.RowSet[attachmentRow]

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// AccountNames maps an account ID to its name, for the Owner column. It is built once per run
	// and shared by every search of the run, so it is only read.
	AccountNames map[string]string `json:"-"`

	// Propagations reads the route tables each attachment propagates to (--propagations): one
	// call per attachment or per route table, whichever is fewer.
	Propagations bool `json:"-"`
}

// attachmentRow represents one Transit Gateway attachment.
type attachmentRow struct {
	// ID is the attachment ID.
	ID string `json:"id,omitempty" header:"Attachment ID" sort:"id"`

	// Name is the Name tag of the attachment.
	Name string `json:"name,omitempty" header:"Name" stack:"" sort:"name"`

	// TransitGatewayID is the ID of the transit gateway.
	TransitGatewayID string `json:"transit_gateway_id,omitempty" header:"TGW ID" sort:"tgw-id"`

	// Type is the resource type: vpc, vpn, direct-connect-gateway, peering, connect...
	Type string `json:"type,omitempty" header:"Type" sort:"type"`

	// ResourceID is the attached resource, such as the VPC ID.
	ResourceID string `json:"resource_id,omitempty" header:"Resource ID" sort:"resource-id"`

	// State is the attachment state, such as available.
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// AssocRTID is the route table the attachment is associated with: the table that routes the
	// traffic coming from the attachment. Empty when the attachment is not associated.
	AssocRTID string `json:"associated_route_table_id,omitempty" header:"Associated RT" sort:"associated-route-table-id"`

	// AssocRTName is the Name tag of the associated route table.
	AssocRTName string `json:"associated_route_table_name,omitempty" header:"Name" stack:"" sort:"associated-route-table"`

	// AssocState is the state of the association: associated, or associating / disassociating
	// while it changes.
	AssocState string `json:"association_state,omitempty"`

	// PropagatedRTs are the route tables the attachment propagates its routes to, one per line:
	// the ID, then the name and the state when it is not enabled. Filled with --propagations; the
	// column is hidden when no row has any.
	PropagatedRTs []string `json:"-" header:"Propagated RTs" hide:"empty"`

	// Propagations are the route tables the attachment propagates its routes to, with their
	// state (the Propagated RTs column); filled with --propagations.
	Propagations []propagation `json:"propagations,omitempty"`

	// OwnerID is the account that owns the resource of the attachment, such as the VPC.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// OwnerName is the name of the owner account, from the accounts map or --org-profile; empty
	// when it is unknown.
	OwnerName string `json:"owner_name,omitempty" header:"Name" stack:"" sort:"owner-name"`

	// TransitGatewayOwnerID is the account that owns the transit gateway.
	TransitGatewayOwnerID string `json:"transit_gateway_owner_id,omitempty"`

	// Created is when the attachment was created, in RFC 3339 UTC so it sorts chronologically.
	Created string `json:"created,omitempty" sort:"created"`

	// Tags are the tags of the attachment, shown in the table with --show-tags.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// NewAttachments initiates and returns a new instance of Transit Gateway attachment results.
func NewAttachments(profile, region string, filters map[string][]string, sortField string) *AttachmentResults {
	return &AttachmentResults{
		RowSet:  common.NewRowSet[attachmentRow](profile, region, sortField),
		Filters: filters,
	}
}

// attachmentQuery is the attachments search asked by the filters.
type attachmentQuery struct {
	// awsFilters are the DescribeTransitGatewayAttachments filters.
	awsFilters []types.Filter
	states     []string
	names      *common.Matcher
	// propagatedRTs keeps the attachments that propagate to these route tables.
	propagatedRTs []string
}

// newAttachmentQuery builds the query from a filters map, which it only reads.
func newAttachmentQuery(filters map[string][]string) (*attachmentQuery, error) {
	attTypes, err := attachmentTypes(filters[FilterKeyAttachmentType])
	if err != nil {
		return nil, err
	}
	q := &attachmentQuery{propagatedRTs: filters[FilterKeyPropagatedRouteTableID]}
	states := make([]string, 0, len(filters[FilterKeyAttachmentState]))
	for _, v := range filters[FilterKeyAttachmentState] {
		states = append(states, camelKebab(v))
	}
	if q.states, err = oneOf(states, AttachmentStateNames(), "attachment state"); err != nil {
		return nil, err
	}
	if q.names, err = common.NewMatcher(filters[FilterKeyAttachmentName], false); err != nil {
		return nil, fmt.Errorf("attachment names: %w", err)
	}
	q.awsFilters = appendFilter(q.awsFilters, awsFilterAttID, filters[FilterKeyAttachmentID])
	q.awsFilters = appendFilter(q.awsFilters, awsFilterAttResourceType, attTypes)
	q.awsFilters = appendFilter(q.awsFilters, awsFilterAttResourceID, filters[FilterKeyResourceID])
	q.awsFilters = appendFilter(q.awsFilters, awsFilterAttTGWID, filters[FilterKeyTransitGatewayID])
	q.awsFilters = appendFilter(q.awsFilters, awsFilterAttAssociatedRT, filters[FilterKeyAssociatedRouteTableID])
	return q, nil
}

// propagatesTo reports whether the row propagates to one of the route tables of
// --propagated-route-table-ids, or the filter is not set.
func (q *attachmentQuery) propagatesTo(row *attachmentRow) bool {
	if len(q.propagatedRTs) == 0 {
		return true
	}
	return slices.ContainsFunc(row.Propagations, func(p propagation) bool {
		return slices.Contains(q.propagatedRTs, p.RouteTableID)
	})
}

// match reports whether a row passes the filters awss matches.
func (q *attachmentQuery) match(row *attachmentRow) bool {
	return (len(q.states) == 0 || slices.Contains(q.states, row.State)) && q.names.Match(row.Name)
}

// Search lists the Transit Gateway attachments of the profile and region.
func (r *AttachmentResults) Search(ctx context.Context) {
	q, err := newAttachmentQuery(r.Filters)
	if err != nil {
		r.AddError(fmt.Sprintf("error building filters: %v", err))
		return
	}
	client, err := newClient(r.Profile, r.Region)
	if err != nil {
		r.AddError(fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	r.collect(ctx, client, q)
}

// collect lists the attachments with client, keeps those that pass the filters, names their
// associated route tables, reads their propagations when asked and sorts the rows.
func (r *AttachmentResults) collect(ctx context.Context, client ec2API, q *attachmentQuery) {
	atts, err := attachments(ctx, client, q.awsFilters)
	if err != nil {
		r.AddError(err.Error())
		return
	}
	for i := range atts {
		if row := r.parseAttachment(&atts[i]); q.match(&row) {
			r.Data = append(r.Data, row)
		}
	}
	propagations := r.Propagations || len(q.propagatedRTs) > 0
	tables, ok := r.routeTables(ctx, client, propagations)
	names := make(map[string]string, len(tables))
	for i := range tables {
		names[aws.ToString(tables[i].TransitGatewayRouteTableId)] = common.TagName(tables[i].Tags)
	}
	for i := range r.Data {
		r.Data[i].AssocRTName = names[r.Data[i].AssocRTID]
	}
	if propagations {
		if ok {
			r.addPropagations(ctx, client, tables, names)
		}
		r.Data = slices.DeleteFunc(r.Data, func(row attachmentRow) bool { return !q.propagatesTo(&row) })
	}

	r.SortRows()
}

// routeTables lists the route tables of the transit gateways of the rows, for their names and, with
// propagations, to read the propagations: one call, none when no row is associated and propagations
// are not asked. A failed call is recorded and reports false; the rows are kept.
func (r *AttachmentResults) routeTables(
	ctx context.Context, client ec2API, propagations bool,
) ([]types.TransitGatewayRouteTable, bool) {
	var tgwIDs []string
	for i := range r.Data {
		needed := propagations || r.Data[i].AssocRTID != ""
		if needed && !slices.Contains(tgwIDs, r.Data[i].TransitGatewayID) {
			tgwIDs = append(tgwIDs, r.Data[i].TransitGatewayID)
		}
	}
	if len(tgwIDs) == 0 {
		return nil, true
	}
	tables, err := routeTables(ctx, client, &query{tgwID: tgwIDs})
	if err != nil {
		r.AddError(err.Error())
		return nil, false
	}
	return tables, true
}

// parseAttachment converts an attachment into a row. The associated route table is kept in any
// association state, which AssocState tells.
func (r *AttachmentResults) parseAttachment(a *types.TransitGatewayAttachment) attachmentRow {
	row := attachmentRow{
		ID:                    aws.ToString(a.TransitGatewayAttachmentId),
		Name:                  common.TagName(a.Tags),
		TransitGatewayID:      aws.ToString(a.TransitGatewayId),
		Type:                  string(a.ResourceType),
		ResourceID:            aws.ToString(a.ResourceId),
		State:                 camelKebab(string(a.State)),
		OwnerID:               aws.ToString(a.ResourceOwnerId),
		TransitGatewayOwnerID: aws.ToString(a.TransitGatewayOwnerId),
		Tags:                  common.TagsToMap(a.Tags),
	}
	row.OwnerName = r.AccountNames[row.OwnerID]
	if a.Association != nil {
		row.AssocRTID = aws.ToString(a.Association.TransitGatewayRouteTableId)
		row.AssocState = string(a.Association.State)
	}
	if a.CreationTime != nil {
		row.Created = a.CreationTime.UTC().Format(time.RFC3339)
	}
	return row
}

// GetAttachmentSortFields returns a map of the attachment sort fields and their corresponding
// struct field. It returns an error if the given field is not a valid sort field.
func GetAttachmentSortFields(f string) (map[string]string, error) {
	return common.SortFields(attachmentRow{}, f)
}

// AttachmentSortFieldNames returns the valid attachment sort fields, sorted alphabetically.
func AttachmentSortFieldNames() []string {
	return common.SortFieldNames(attachmentRow{})
}
