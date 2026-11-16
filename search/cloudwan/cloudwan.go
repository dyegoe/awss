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

// Package cloudwan contains the search for the routes of AWS Cloud WAN core networks, and what the
// Cloud WAN searches share.
//
// It implements the common.Results interface. Network Manager is global: the search runs once
// per profile, for no region, and reads every segment and network function group route table of
// every edge location of the core networks the profile sees.
package cloudwan

import (
	"context"
	"fmt"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// Results describes the Cloud WAN routes found with one profile.
type Results struct {
	common.BaseResults

	// Data contains the routes found: one row per route destination.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// AccountNames maps an account ID to its name, for the Owner column. It is built once per run
	// and shared by every search of the run, so it is only read.
	AccountNames map[string]string `json:"-"`
}

// dataRow represents one destination of one route of one route table. dataRow holds only strings,
// so it is comparable and rows are deduplicated as map keys.
type dataRow struct {
	// Query is the searched CIDR the route matched; empty when no CIDR was searched.
	Query string `json:"query,omitempty" header:"Query" sort:"query"`

	// CoreNetworkID is the ID of the core network.
	CoreNetworkID string `json:"core_network_id,omitempty" sort:"core-network"`

	// Segment is the segment of the route table, or nfg:<name> for a network function group.
	Segment string `json:"segment,omitempty" header:"Segment" sort:"segment"`

	// Edge is the edge location (region) of the route table.
	Edge string `json:"edge,omitempty" header:"Edge" sort:"edge"`

	// CIDR is the destination CIDR block of the route.
	CIDR string `json:"cidr,omitempty" header:"CIDR" sort:"cidr"`

	// PrefixListID is the prefix list of the route, when the route comes from one.
	PrefixListID string `json:"prefix_list_id,omitempty"`

	// Type is propagated, static or system.
	Type string `json:"type,omitempty" header:"Type" sort:"type"`

	// State is active or blackhole.
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// TargetType is the type of the next hop: vpc, direct-connect-gateway, vpn, connect-peer,
	// transit-gateway-route-table...
	TargetType string `json:"target_type,omitempty" header:"Target Type" sort:"target-type"`

	// TargetID is the ID of the next hop resource, such as a VPC or a Direct Connect gateway.
	TargetID string `json:"target_id,omitempty" header:"Target ID" sort:"target-id"`

	// AttachmentID is the core network attachment of the next hop.
	AttachmentID string `json:"attachment_id,omitempty" sort:"attachment"`

	// AttachmentName is the Name tag of the attachment.
	AttachmentName string `json:"attachment_name,omitempty" header:"Attachment" sort:"attachment-name"`

	// TargetSegment is the segment of the next hop (nfg:<name> for a function group): another
	// segment for a route shared between segments.
	TargetSegment string `json:"target_segment,omitempty" header:"Target Segment" sort:"target-segment"`

	// TargetEdge is the edge location of the next hop.
	TargetEdge string `json:"target_edge,omitempty"`

	// OwnerID is the account that owns the attachment of the next hop, such as the account of a
	// VPC; empty when the attachments could not be listed.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// OwnerName is the name of the owner account, from AccountNames; empty when it is unknown.
	OwnerName string `json:"owner_name,omitempty" header:"Owner" sort:"owner-name"`

	// TransitGatewayAttachmentID is the Transit Gateway attachment of the next hop, for a route
	// through a Transit Gateway route table attachment.
	TransitGatewayAttachmentID string `json:"transit_gateway_attachment_id,omitempty"`
}

// New initiates and returns a new instance of Cloud WAN routes results. region is kept for the
// output only: the search always calls HomeRegion.
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

// getCoreNetworkAPI is the part of the Network Manager client that reads one core network. The
// SDK has no interface for it, since the call is not paginated.
type getCoreNetworkAPI interface {
	GetCoreNetwork(
		ctx context.Context, params *networkmanager.GetCoreNetworkInput, optFns ...func(*networkmanager.Options),
	) (*networkmanager.GetCoreNetworkOutput, error)
}

// routesAPI is the part of the Network Manager client used by the routes search.
type routesAPI interface {
	networkmanager.ListCoreNetworksAPIClient
	networkmanager.ListAttachmentsAPIClient
	networkmanager.GetNetworkRoutesAPIClient
	getCoreNetworkAPI
}

// newClient builds the Network Manager client of a profile. It is a variable so tests can replace
// the AWS config with a fake client.
var newClient = func(profile string) (routesAPI, error) { return NewClient(profile) }

// Search searches the routes of every core network the profile sees. A missing permission is an
// error in the result set, never an empty list.
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

// collect searches every core network of the query with client, then sorts the rows.
func (r *Results) collect(ctx context.Context, client routesAPI, q *query) {
	nets, err := CoreNetworks(ctx, client, q.coreNetworks)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return
	}
	for i := range nets {
		r.searchCoreNetwork(ctx, client, &nets[i], q)
	}

	if r.SortField == "" {
		return
	}
	if err := r.sortResults(r.SortField); err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// coreNetwork is what a route call needs to know of its core network.
type coreNetwork struct {
	id, globalNetworkID string

	// attachments maps an attachment ID to its Name tag and owner account.
	attachments map[string]attachmentInfo

	// accountNames maps an account ID to its name, for the Owner column.
	accountNames map[string]string
}

// attachmentInfo is what a route row shows of its attachment.
type attachmentInfo struct{ name, owner string }

// searchCoreNetwork reads the route tables of one core network and adds the routes that match q.
// A failure is recorded in Errors and the run goes on with the next core network.
func (r *Results) searchCoreNetwork(ctx context.Context, client routesAPI, s *types.CoreNetworkSummary, q *query) {
	cn := coreNetwork{id: aws.ToString(s.CoreNetworkId), globalNetworkID: aws.ToString(s.GlobalNetworkId)}

	out, err := client.GetCoreNetwork(ctx, &networkmanager.GetCoreNetworkInput{CoreNetworkId: aws.String(cn.id)})
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error reading core network %s: %v", cn.id, err))
		return
	}
	if out == nil || out.CoreNetwork == nil {
		return
	}
	if gid := aws.ToString(out.CoreNetwork.GlobalNetworkId); gid != "" {
		cn.globalNetworkID = gid
	}

	tables := routeTables(cn.id, out.CoreNetwork, q)
	if len(tables) == 0 {
		return
	}

	// The names are a convenience: without them the rows still show the attachment IDs.
	cn.attachments, cn.accountNames = map[string]attachmentInfo{}, r.AccountNames
	atts, err := Attachments(ctx, client, cn.id)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("no attachment names: %v", err))
	}
	for i := range atts {
		cn.attachments[aws.ToString(atts[i].AttachmentId)] = attachmentInfo{
			name: TagName(atts[i].Tags), owner: aws.ToString(atts[i].OwnerAccountId),
		}
	}

	r.runCalls(ctx, client, &cn, calls(tables, q), q)
}

// sortResults sorts the results by the given field.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}
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
