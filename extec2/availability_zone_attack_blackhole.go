// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type azBlackholeAction struct {
	clientProvider             func(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error)
	extensionRootAccountNumber string
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[BlackholeState] = (*azBlackholeAction)(nil)
var _ action_kit_sdk.ActionWithStop[BlackholeState] = (*azBlackholeAction)(nil)

func NewAzBlackholeAction() action_kit_sdk.Action[BlackholeState] {
	return &azBlackholeAction{
		clientProvider:             defaultClientProviderAzBlackhole,
		extensionRootAccountNumber: utils.GetRootAccountNumber(),
	}
}

func (e *azBlackholeAction) NewEmptyState() BlackholeState {
	return BlackholeState{}
}

func (e *azBlackholeAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          azBlackholeActionId,
		Label:       "Blackhole Availability Zone",
		Description: "Simulates an outage of an entire availability zone.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(azIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: azTargetType,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "zone",
					Description: extutil.Ptr("Find zone by name"),
					Query:       "aws.zone=\"\"",
				},
				{
					Label:       "zone-id",
					Description: extutil.Ptr("Find zone by zone id"),
					Query:       "aws.zone.id=\"\"",
				},
			})}),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("Network"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr(""),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (e *azBlackholeAction) Prepare(ctx context.Context, state *BlackholeState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return prepareBlackhole(ctx, state, request, e.extensionRootAccountNumber, e.clientProvider, getTargetSubnetsForBlackholeZone)
}

func getTargetSubnetsForBlackholeZone(clientEc2 blackholeEC2Api, ctx context.Context, target *action_kit_api.Target) (map[string][]string, error) {
	targetZone := extutil.MustHaveValue(target.Attributes, "aws.zone")[0]

	subnetResults := make(map[string][]string)

	paginator := ec2.NewDescribeSubnetsPaginator(clientEc2,
		&ec2.DescribeSubnetsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("availabilityZone"),
					Values: []string{targetZone},
				},
			},
		})

	for paginator.HasMorePages() {
		subnets, err := paginator.NextPage(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get subnets")
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to get subnets for zone %s", targetZone), err)
		}
		for _, subnet := range subnets.Subnets {
			subnetResults[*subnet.VpcId] = append(subnetResults[*subnet.VpcId], *subnet.SubnetId)
		}
		log.Debug().Msgf("Found %d subnets in AZ %s for creating temporary ACL to block traffic: %+v", len(subnets.Subnets), targetZone, subnets.Subnets)
	}
	return subnetResults, nil
}

func (e *azBlackholeAction) Start(ctx context.Context, state *BlackholeState) (*action_kit_api.StartResult, error) {
	return startBlackhole(ctx, state, e.clientProvider)
}

func (e *azBlackholeAction) Stop(ctx context.Context, state *BlackholeState) (*action_kit_api.StopResult, error) {
	return stopBlackhole(ctx, state, e.clientProvider)
}

func defaultClientProviderAzBlackhole(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, nil, err
	}
	clientEc2 := ec2.NewFromConfig(awsAccess.AwsConfig)
	clientImds := imds.NewFromConfig(awsAccess.AwsConfig)
	if err != nil {
		return nil, nil, err
	}
	return clientEc2, clientImds, nil
}
