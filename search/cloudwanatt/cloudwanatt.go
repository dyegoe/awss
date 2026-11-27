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

// Package cloudwanatt contains the search for the attachments of AWS Cloud WAN core networks.
//
// It implements the common.Results interface. Network Manager is global: the search runs once per
// profile, for no region. ListAttachments filters on one type, one edge and one state at most, so
// awss lists every attachment and matches the filters on the results.
package cloudwanatt

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dyegoe/awss/common"
	"github.com/dyegoe/awss/search/cloudwan"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
)

// Filter keys of the attachments search, all matched on the results.
const (
	FilterKeyID          = "id"
	FilterKeyName        = "name"
	FilterKeyType        = "type"
	FilterKeySegment     = "segment"
	FilterKeyEdge        = "edge"
	FilterKeyState       = "state"
	FilterKeyCoreNetwork = "core-network-id"
)

// TypeNames returns the attachment types, as awss writes them: vpc, direct-connect-gateway...
func TypeNames() []string { return cloudwan.KebabNames(types.AttachmentType("").Values()) }

// StateNames returns the attachment states, as awss writes them: available, pending-...
func StateNames() []string { return cloudwan.KebabNames(types.AttachmentState("").Values()) }

// CheckFlags validates the attachment types and states, and the name, segment and edge globs,
// before any AWS call.
func CheckFlags(filters map[string][]string) error {
	_, err := newLocalFilters(filters)
	return err
}

// Results describes the Cloud WAN attachments found with one profile.
type Results struct {
	common.RowSet[dataRow]

	// Filters is a map of strings used to search, keyed by the FilterKey constants.
	Filters map[string][]string `json:"-"`

	// AccountNames maps an account ID to its name, for the Owner column. It is built once per run
	// and shared by every search of the run, so it is only read.
	AccountNames map[string]string `json:"-"`
}

// dataRow represents one attachment.
type dataRow struct {
	// CoreNetworkID is the ID of the core network.
	CoreNetworkID string `json:"core_network_id,omitempty" sort:"core-network"`

	// ID is the attachment ID.
	ID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Name is the Name tag of the attachment.
	Name string `json:"name,omitempty" header:"Name" stack:"" sort:"name"`

	// Type is vpc, direct-connect-gateway, site-to-site-vpn, connect or
	// transit-gateway-route-table.
	Type string `json:"type,omitempty" header:"Type" sort:"type"`

	// Segment is the segment of the attachment, or nfg:<name> for a network function group.
	Segment string `json:"segment,omitempty" header:"Segment" sort:"segment"`

	// Edges are the edge locations of the attachment: one, or several for a Direct Connect gateway.
	Edges []string `json:"edges,omitempty" header:"Edges" sort:"edge"`

	// State is the attachment state, such as available.
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// Resource is the ID of the attached resource, from its ARN: a VPC, a Direct Connect gateway,
	// a VPN, a Transit Gateway route table...
	Resource string `json:"resource,omitempty" header:"Resource" sort:"resource"`

	// ResourceARN is the ARN of the attached resource.
	ResourceARN string `json:"resource_arn,omitempty"`

	// OwnerID is the account that owns the attachment.
	OwnerID string `json:"owner_id,omitempty" header:"Owner ID" sort:"owner"`

	// OwnerName is the name of the owner account, from AccountNames; empty when it is unknown.
	OwnerName string `json:"owner_name,omitempty" header:"Name" stack:"" sort:"owner-name"`

	// PolicyRule is the attachment policy rule number that placed it in its segment.
	PolicyRule string `json:"policy_rule,omitempty"`

	// Created is when the attachment was created, in RFC 3339 UTC so it sorts chronologically.
	Created string `json:"created,omitempty" sort:"created"`

	// Tags are the tags of the attachment.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of Cloud WAN attachments results. region is kept for
// the output only: the search always calls cloudwan.HomeRegion.
func New(profile, region string, filters map[string][]string, sortField string) *Results {
	return &Results{
		RowSet:  common.NewRowSet[dataRow](profile, region, sortField),
		Filters: filters,
	}
}

// attachmentsAPI is the part of the Network Manager client used by the attachments search.
type attachmentsAPI interface {
	networkmanager.ListCoreNetworksAPIClient
	networkmanager.ListAttachmentsAPIClient
}

// newClient builds the Network Manager client of a profile. It is a variable so tests can replace
// the AWS config with a fake client.
var newClient = func(profile string) (attachmentsAPI, error) { return cloudwan.NewClient(profile) }

// localFilters are the filters awss matches on the attachments.
type localFilters struct {
	ids, types, states, coreNetworks []string
	names, segments, edges           *common.Matcher
}

// newLocalFilters builds the local filters from a filters map, which it only reads.
func newLocalFilters(filters map[string][]string) (*localFilters, error) {
	f := &localFilters{ids: filters[FilterKeyID], coreNetworks: filters[FilterKeyCoreNetwork]}
	var err error
	if f.types, err = kebabValues(filters[FilterKeyType], TypeNames(), "attachment type"); err != nil {
		return nil, err
	}
	if f.states, err = kebabValues(filters[FilterKeyState], StateNames(), "attachment state"); err != nil {
		return nil, err
	}
	if f.names, err = common.NewMatcher(filters[FilterKeyName], false); err != nil {
		return nil, fmt.Errorf("names: %w", err)
	}
	if f.segments, err = common.NewMatcher(filters[FilterKeySegment], false); err != nil {
		return nil, fmt.Errorf("segments: %w", err)
	}
	if f.edges, err = common.NewMatcher(filters[FilterKeyEdge], false); err != nil {
		return nil, fmt.Errorf("edges: %w", err)
	}
	return f, nil
}

// kebabValues returns the values in kebab case, checking each against valid.
func kebabValues(values, valid []string, what string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = cloudwan.Kebab(v)
		if !slices.Contains(valid, v) {
			return nil, fmt.Errorf("invalid %s %q; valid: %s", what, v, strings.Join(valid, ", "))
		}
		out = append(out, v)
	}
	return out, nil
}

// match reports whether a row passes every filter. A segment glob also matches nfg:<name>.
func (f *localFilters) match(row *dataRow) bool {
	return (len(f.ids) == 0 || slices.Contains(f.ids, row.ID)) &&
		(len(f.types) == 0 || slices.Contains(f.types, row.Type)) &&
		(len(f.states) == 0 || slices.Contains(f.states, row.State)) &&
		f.names.Match(row.Name) && f.segments.Match(row.Segment) && f.edges.MatchAny(row.Edges)
}

// Search lists the attachments of every core network the profile sees. A missing permission is an
// error in the result set, never an empty list.
func (r *Results) Search(ctx context.Context) {
	local, err := newLocalFilters(r.Filters)
	if err != nil {
		r.AddError(fmt.Sprintf("error building filters: %v", err))
		return
	}
	client, err := newClient(r.Profile)
	if err != nil {
		r.AddError(fmt.Sprintf("error getting aws config: %v", err))
		return
	}
	r.collect(ctx, client, local)
}

// collect lists the attachments of every core network with client, keeps those that pass the
// filters, then sorts the rows. A core network that fails is recorded and the next one is listed.
func (r *Results) collect(ctx context.Context, client attachmentsAPI, local *localFilters) {
	nets, err := cloudwan.CoreNetworks(ctx, client, local.coreNetworks)
	if err != nil {
		r.AddError(err.Error())
		return
	}
	for i := range nets {
		atts, err := cloudwan.Attachments(ctx, client, aws.ToString(nets[i].CoreNetworkId))
		if err != nil {
			r.AddError(err.Error())
			continue
		}
		for j := range atts {
			if row := r.parseAttachment(&atts[j]); local.match(&row) {
				r.Data = append(r.Data, row)
			}
		}
	}

	r.SortRows()
}

// parseAttachment converts an attachment into a dataRow.
func (r *Results) parseAttachment(a *types.Attachment) dataRow {
	row := dataRow{
		CoreNetworkID: aws.ToString(a.CoreNetworkId),
		ID:            aws.ToString(a.AttachmentId),
		Name:          cloudwan.TagName(a.Tags),
		Type:          cloudwan.Kebab(string(a.AttachmentType)),
		Segment:       aws.ToString(a.SegmentName),
		State:         cloudwan.Kebab(string(a.State)),
		ResourceARN:   aws.ToString(a.ResourceArn),
		OwnerID:       aws.ToString(a.OwnerAccountId),
		Tags:          cloudwan.TagsToMap(a.Tags),
	}
	if g := aws.ToString(a.NetworkFunctionGroupName); g != "" && row.Segment == "" {
		row.Segment = cloudwan.FunctionGroupPrefix + g
	}
	row.Edges = attachmentEdges(a)
	row.Resource = resourceID(row.ResourceARN)
	row.OwnerName = r.AccountNames[row.OwnerID]
	if a.AttachmentPolicyRuleNumber != nil {
		row.PolicyRule = fmt.Sprint(*a.AttachmentPolicyRuleNumber)
	}
	if a.CreatedAt != nil {
		row.Created = a.CreatedAt.UTC().Format(time.RFC3339)
	}
	return row
}

// attachmentEdges returns the edge locations of an attachment: EdgeLocations for a Direct Connect
// gateway, which spans several, and EdgeLocation for the others.
func attachmentEdges(a *types.Attachment) []string {
	if len(a.EdgeLocations) > 0 {
		return slices.Clone(a.EdgeLocations)
	}
	if e := aws.ToString(a.EdgeLocation); e != "" {
		return []string{e}
	}
	return nil
}

// resourceID returns the resource ID of an ARN: what follows its last slash, or its last colon
// when it has no slash, such as vpc-0abc for arn:aws:ec2:eu-west-1:111111111111:vpc/vpc-0abc.
func resourceID(arn string) string {
	if i := strings.LastIndex(arn, "/"); i >= 0 {
		return arn[i+1:]
	}
	if i := strings.LastIndex(arn, ":"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}

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
