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

package dxgw

import (
	"context"
	"fmt"

	"github.com/dyegoe/awss/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/directconnect"
	"github.com/aws/aws-sdk-go-v2/service/directconnect/types"
)

// Region is the region shown for the Direct Connect gateway results: a gateway is global, so the
// search runs once per profile.
const Region = "global"

// fallbackRegion is the region of the Direct Connect calls when the profile names none. Any region
// answers with every gateway of the account.
const fallbackRegion = "us-east-1"

// pageSize is the page size of the Direct Connect calls: AWS returns 100 at most.
const pageSize int32 = 100

// NewClient returns the Direct Connect client of a profile, in the region of the profile, or
// us-east-1 when it names none.
func NewClient(profile string) (*directconnect.Client, error) {
	cfg, err := common.AwsConfig(profile, "")
	if err != nil {
		return nil, fmt.Errorf("loading the aws config: %w", err)
	}
	if cfg.Region == "" {
		cfg.Region = fallbackRegion
	}
	return directconnect.NewFromConfig(cfg), nil
}

// gatewaysAPI is the part of the Direct Connect client used by the search. The SDK has no
// paginators for these calls.
type gatewaysAPI interface {
	DescribeDirectConnectGateways(
		ctx context.Context, params *directconnect.DescribeDirectConnectGatewaysInput,
		optFns ...func(*directconnect.Options),
	) (*directconnect.DescribeDirectConnectGatewaysOutput, error)
	DescribeDirectConnectGatewayAssociations(
		ctx context.Context, params *directconnect.DescribeDirectConnectGatewayAssociationsInput,
		optFns ...func(*directconnect.Options),
	) (*directconnect.DescribeDirectConnectGatewayAssociationsOutput, error)
	DescribeDirectConnectGatewayAttachments(
		ctx context.Context, params *directconnect.DescribeDirectConnectGatewayAttachmentsInput,
		optFns ...func(*directconnect.Options),
	) (*directconnect.DescribeDirectConnectGatewayAttachmentsOutput, error)
}

// allPages calls page with each NextToken until AWS returns none, and returns every item. It stops
// when AWS returns the token it was given, so a faulty answer cannot loop forever. what names the
// list in the error.
func allPages[T any](what string, page func(*string) ([]T, *string, error)) ([]T, error) {
	var all []T
	var token *string
	for {
		items, next, err := page(token)
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", what, err)
		}
		all = append(all, items...)
		if aws.ToString(next) == "" || aws.ToString(next) == aws.ToString(token) {
			return all, nil
		}
		token = next
	}
}

// gateways lists every Direct Connect gateway of the account.
func gateways(ctx context.Context, client gatewaysAPI) ([]types.DirectConnectGateway, error) {
	return allPages("the Direct Connect gateways", func(token *string) ([]types.DirectConnectGateway, *string, error) {
		out, err := client.DescribeDirectConnectGateways(ctx, &directconnect.DescribeDirectConnectGatewaysInput{
			MaxResults: aws.Int32(pageSize), NextToken: token,
		})
		if err != nil || out == nil {
			return nil, nil, err
		}
		return out.DirectConnectGateways, out.NextToken, nil
	})
}

// associations lists the associations of one Direct Connect gateway. AWS needs a gateway, a
// virtual private gateway or a transit gateway, so no call returns them all.
//
//nolint:dupl // the same paging as attachments over other SDK types; Go cannot be generic over them
func associations(
	ctx context.Context, client gatewaysAPI, gatewayID string,
) ([]types.DirectConnectGatewayAssociation, error) {
	in := &directconnect.DescribeDirectConnectGatewayAssociationsInput{
		DirectConnectGatewayId: aws.String(gatewayID), MaxResults: aws.Int32(pageSize),
	}
	what := "the associations of Direct Connect gateway " + gatewayID
	return allPages(what, func(token *string) ([]types.DirectConnectGatewayAssociation, *string, error) {
		in.NextToken = token
		out, err := client.DescribeDirectConnectGatewayAssociations(ctx, in)
		if err != nil || out == nil {
			return nil, nil, err
		}
		return out.DirectConnectGatewayAssociations, out.NextToken, nil
	})
}

// attachments lists the virtual interfaces attached to one Direct Connect gateway.
//
//nolint:dupl // the same paging as associations over other SDK types; Go cannot be generic over them
func attachments(
	ctx context.Context, client gatewaysAPI, gatewayID string,
) ([]types.DirectConnectGatewayAttachment, error) {
	in := &directconnect.DescribeDirectConnectGatewayAttachmentsInput{
		DirectConnectGatewayId: aws.String(gatewayID), MaxResults: aws.Int32(pageSize),
	}
	what := "the virtual interfaces of Direct Connect gateway " + gatewayID
	return allPages(what, func(token *string) ([]types.DirectConnectGatewayAttachment, *string, error) {
		in.NextToken = token
		out, err := client.DescribeDirectConnectGatewayAttachments(ctx, in)
		if err != nil || out == nil {
			return nil, nil, err
		}
		return out.DirectConnectGatewayAttachments, out.NextToken, nil
	})
}
