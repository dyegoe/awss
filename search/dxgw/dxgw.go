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

// Package dxgw contains the search for the AWS Direct Connect gateways, what they are associated
// with and the prefixes they advertise to the data centres.
//
// It implements the common.Results interface. A Direct Connect gateway is global: the search runs
// once per profile, in the region of the profile, and every region answers with every gateway.
package dxgw

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/directconnect/types"
)

// Results describes the Direct Connect gateways found with one profile.
type Results struct {
	common.BaseResults

	// Data contains the gateways found: one row per allowed prefix of each association.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// AccountNames maps an account ID to its name, for the Owner column. It is built once per run
	// and shared by every search of the run, so it is only read.
	AccountNames map[string]string `json:"-"`
}

// dataRow represents one allowed prefix of one association of one Direct Connect gateway. A
// gateway without associations, or an association without allowed prefixes, is one row with the
// missing columns empty.
type dataRow struct {
	// Query is the searched CIDR the allowed prefix matched; empty when no CIDR was searched.
	Query string `json:"query,omitempty" header:"Query" sort:"query"`

	// GatewayID is the ID of the Direct Connect gateway.
	GatewayID string `json:"gateway_id,omitempty" header:"Gateway ID" sort:"gateway-id"`

	// GatewayName is the name of the Direct Connect gateway.
	GatewayName string `json:"gateway_name,omitempty" header:"Name" stack:"" sort:"gateway"`

	// AmazonASN is the AS number of the Amazon side of the BGP sessions.
	AmazonASN string `json:"amazon_asn,omitempty" header:"Amazon ASN" sort:"amazon-asn"`

	// GatewayState is the state of the gateway, such as available.
	GatewayState string `json:"gateway_state,omitempty"`

	// GatewayOwnerID is the account that owns the gateway.
	GatewayOwnerID string `json:"gateway_owner_id,omitempty"`

	// AssociatedType is transit-gateway, virtual-private-gateway or core-network.
	AssociatedType string `json:"associated_type,omitempty" header:"Associated Type" sort:"associated-type"`

	// AssociatedID is the ID of the associated transit gateway, virtual private gateway or core
	// network.
	AssociatedID string `json:"associated_id,omitempty" header:"Associated ID" sort:"associated-id"`

	// AssociatedRegion is the region of the associated gateway; empty for a core network.
	AssociatedRegion string `json:"associated_region,omitempty" header:"Region" sort:"region"`

	// OwnerID is the account that owns the associated gateway or core network.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// OwnerName is the name of the owner account, from AccountNames; empty when it is unknown.
	OwnerName string `json:"owner_name,omitempty" header:"Name" stack:"" sort:"owner-name"`

	// State is the state of the association, such as associated.
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// AllowedPrefix is a prefix the gateway advertises to the data centres for this association.
	AllowedPrefix common.CIDR `json:"allowed_prefix,omitempty" header:"Allowed Prefix" sort:"prefix"`

	// AssociationID is the ID of the association.
	AssociationID string `json:"association_id,omitempty"`

	// CoreNetworkAttachmentID is the Cloud WAN attachment of the gateway, for a core network.
	CoreNetworkAttachmentID string `json:"core_network_attachment_id,omitempty"`

	// StateChangeError is why the association could not change state, when it could not.
	StateChangeError string `json:"state_change_error,omitempty"`

	// VIFs are the virtual interfaces attached to the gateway: the BGP sessions to the data centres.
	VIFs []string `json:"vifs,omitempty" header:"VIFs"`

	// VIFAttachments are the virtual interfaces attached to the gateway, with their region, owner
	// and state.
	VIFAttachments []vifAttachment `json:"vif_attachments,omitempty"`
}

// vifAttachment is a virtual interface attached to a Direct Connect gateway.
type vifAttachment struct {
	ID      string `json:"id,omitempty"`
	Region  string `json:"region,omitempty"`
	OwnerID string `json:"owner_id,omitempty"`
	State   string `json:"state,omitempty"`
}

// New initiates and returns a new instance of Direct Connect gateway results. region is kept for
// the output only: the search calls the region of the profile.
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

// newClient builds the Direct Connect client of a profile. It is a variable so tests can replace
// the AWS config with a fake client.
var newClient = func(profile string) (gatewaysAPI, error) { return NewClient(profile) }

// Search lists the Direct Connect gateways of the profile, their associations and the virtual
// interfaces attached to them. A missing permission is an error in the result set, never an empty
// list.
func (r *Results) Search(ctx context.Context) {
	q, err := newQuery(r.Filters)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}
	client, err := newClient(r.Profile)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	r.collect(ctx, client, q)
}

// collect lists the gateways with client, reads the associations and virtual interfaces of each
// one the filters keep, then sorts the rows. A gateway that fails is recorded and the next one is
// read.
func (r *Results) collect(ctx context.Context, client gatewaysAPI, q *query) {
	gws, err := gateways(ctx, client)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}
	for i := range gws {
		gw := r.parseGateway(&gws[i])
		if !q.wantGateway(&gw) {
			continue
		}
		rows, err := r.gatewayRows(ctx, client, q, &gw)
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
			continue
		}
		r.Data = append(r.Data, rows...)
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// gatewayRows returns the rows of one gateway, gw being its gateway columns: one per allowed
// prefix of each association that passes the filters.
func (r *Results) gatewayRows(ctx context.Context, client gatewaysAPI, q *query, gw *dataRow) ([]dataRow, error) {
	vifs, err := attachments(ctx, client, gw.GatewayID)
	if err != nil {
		return nil, err
	}
	for i := range vifs {
		v := parseVIF(&vifs[i])
		gw.VIFs, gw.VIFAttachments = append(gw.VIFs, v.ID), append(gw.VIFAttachments, v)
	}
	as, err := associations(ctx, client, gw.GatewayID)
	if err != nil {
		return nil, err
	}
	if len(as) == 0 {
		if q.wantUnassociated() {
			return []dataRow{*gw}, nil
		}
		return nil, nil
	}
	var rows []dataRow
	for i := range as {
		a := r.parseAssociation(gw, &as[i])
		if q.wantAssociation(&a) {
			rows = append(rows, q.prefixRows(&a, as[i].AllowedPrefixesToDirectConnectGateway)...)
		}
	}
	return rows, nil
}

// parseGateway returns the gateway columns of a gateway's rows.
func (r *Results) parseGateway(g *types.DirectConnectGateway) dataRow {
	row := dataRow{
		GatewayID:      aws.ToString(g.DirectConnectGatewayId),
		GatewayName:    aws.ToString(g.DirectConnectGatewayName),
		GatewayState:   string(g.DirectConnectGatewayState),
		GatewayOwnerID: aws.ToString(g.OwnerAccount),
	}
	if g.AmazonSideAsn != nil {
		row.AmazonASN = strconv.FormatInt(*g.AmazonSideAsn, 10)
	}
	return row
}

// parseAssociation returns the row of an association, on top of the gateway columns gw. The
// association is to a transit gateway or a virtual private gateway (AssociatedGateway), or to a
// Cloud WAN core network (AssociatedCoreNetwork).
func (r *Results) parseAssociation(gw *dataRow, a *types.DirectConnectGatewayAssociation) dataRow {
	row := *gw
	row.State = string(a.AssociationState)
	row.AssociationID = aws.ToString(a.AssociationId)
	row.StateChangeError = aws.ToString(a.StateChangeError)
	switch {
	case a.AssociatedCoreNetwork != nil:
		row.AssociatedType = AssociatedCoreNetwork
		row.AssociatedID = aws.ToString(a.AssociatedCoreNetwork.Id)
		row.OwnerID = aws.ToString(a.AssociatedCoreNetwork.OwnerAccount)
		row.CoreNetworkAttachmentID = aws.ToString(a.AssociatedCoreNetwork.AttachmentId)
	case a.AssociatedGateway != nil:
		row.AssociatedType = gatewayType(a.AssociatedGateway.Type)
		row.AssociatedID = aws.ToString(a.AssociatedGateway.Id)
		row.OwnerID = aws.ToString(a.AssociatedGateway.OwnerAccount)
		row.AssociatedRegion = aws.ToString(a.AssociatedGateway.Region)
	default:
		row.AssociatedID = aws.ToString(a.VirtualGatewayId)
		row.OwnerID = aws.ToString(a.VirtualGatewayOwnerAccount)
		if row.AssociatedID != "" {
			row.AssociatedType = AssociatedVirtualPrivateGateway
		}
	}
	row.OwnerName = r.AccountNames[row.OwnerID]
	return row
}

// parseVIF returns a virtual interface attached to a gateway.
func parseVIF(a *types.DirectConnectGatewayAttachment) vifAttachment {
	return vifAttachment{
		ID:      aws.ToString(a.VirtualInterfaceId),
		Region:  aws.ToString(a.VirtualInterfaceRegion),
		OwnerID: aws.ToString(a.VirtualInterfaceOwnerAccount),
		State:   string(a.AttachmentState),
	}
}

// sortResults sorts the results by the given field. Rows equal on it stay grouped by gateway and
// association, with the allowed prefixes in address order.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}
	slices.SortStableFunc(r.Data, func(a, b dataRow) int {
		if c := cmp.Or(
			cmp.Compare(a.GatewayName, b.GatewayName), cmp.Compare(a.GatewayID, b.GatewayID),
			cmp.Compare(a.AssociatedID, b.AssociatedID),
		); c != 0 {
			return c
		}
		switch {
		case common.CIDRLess(string(a.AllowedPrefix), string(b.AllowedPrefix)):
			return -1
		case common.CIDRLess(string(b.AllowedPrefix), string(a.AllowedPrefix)):
			return 1
		}
		return 0
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
