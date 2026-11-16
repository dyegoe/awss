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

package cloudwan

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager"
	"github.com/aws/aws-sdk-go-v2/service/networkmanager/types"
	"github.com/aws/smithy-go"
)

// Region is the region of the Cloud WAN result sets: Network Manager is global, so --regions does
// not apply.
const Region = "global"

// HomeRegion is the region of the Network Manager API endpoint: Cloud WAN is global, and its
// control plane answers in us-west-2 for every edge location.
const HomeRegion = "us-west-2"

// PageSize is the number of items asked per page of the Network Manager List and Get calls: the
// largest the API allows (1 to 500). The SDK paginators set no page size by themselves.
const PageSize int32 = 500

// maxAttempts is how many times the Network Manager client tries a call: a throttled call waits
// and retries with the SDK's backoff, longer than the SDK's default of 3 attempts.
const maxAttempts = 10

// NewClient builds the Network Manager client of a profile, in HomeRegion.
func NewClient(profile string) (*networkmanager.Client, error) {
	cfg, err := common.AwsConfig(profile, HomeRegion)
	if err != nil {
		return nil, fmt.Errorf("loading the aws config: %w", err)
	}
	return networkmanager.NewFromConfig(cfg, func(o *networkmanager.Options) {
		o.Retryer = retry.AddWithMaxAttempts(o.Retryer, maxAttempts)
	}), nil
}

// CoreNetworks lists the core networks the profile sees, following every page. With ids, it keeps
// only those core networks.
func CoreNetworks(
	ctx context.Context, client networkmanager.ListCoreNetworksAPIClient, ids []string,
) ([]types.CoreNetworkSummary, error) {
	var nets []types.CoreNetworkSummary
	paginator := networkmanager.NewListCoreNetworksPaginator(client, &networkmanager.ListCoreNetworksInput{
		MaxResults: aws.Int32(PageSize),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing the core networks: %w", err)
		}
		for _, n := range page.CoreNetworks {
			if len(ids) == 0 || slices.Contains(ids, aws.ToString(n.CoreNetworkId)) {
				nets = append(nets, n)
			}
		}
	}
	return nets, nil
}

// Attachments lists the attachments of a core network, following every page.
func Attachments(
	ctx context.Context, client networkmanager.ListAttachmentsAPIClient, coreNetworkID string,
) ([]types.Attachment, error) {
	var atts []types.Attachment
	paginator := networkmanager.NewListAttachmentsPaginator(client, &networkmanager.ListAttachmentsInput{
		CoreNetworkId: aws.String(coreNetworkID),
		MaxResults:    aws.Int32(PageSize),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing the attachments of core network %s: %w", coreNetworkID, err)
		}
		atts = append(atts, page.Attachments...)
	}
	return atts, nil
}

// TagName returns the value of the Name tag, or "" when there is none.
func TagName(tags []types.Tag) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Name" {
			return aws.ToString(t.Value)
		}
	}
	return ""
}

// TagsToMap converts Network Manager tags to a map. Tags without a key are dropped.
func TagsToMap(tags []types.Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		if t.Key != nil {
			m[*t.Key] = aws.ToString(t.Value)
		}
	}
	return m
}

// Kebab returns an AWS enum value, such as DIRECT_CONNECT_GATEWAY, as awss writes CLI values:
// direct-connect-gateway.
func Kebab(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "_", "-"))
}

// FunctionGroupPrefix marks a network function group where a segment name is expected, in the
// Segment columns: a route table or attachment belongs to a segment or to a function group.
const FunctionGroupPrefix = "nfg:"

// errCodeAccessDenied is the error code of a call the profile may not make.
const errCodeAccessDenied = "AccessDeniedException"

// isAccessDenied reports whether err says the profile may not make the call.
func isAccessDenied(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == errCodeAccessDenied
}
