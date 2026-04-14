// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extec2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type subnetBlackholeAction struct {
	clientProvider             func(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error)
	extensionRootAccountNumber string
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[BlackholeState] = (*subnetBlackholeAction)(nil)
var _ action_kit_sdk.ActionWithStop[BlackholeState] = (*subnetBlackholeAction)(nil)

func NewSubnetBlackholeAction() action_kit_sdk.Action[BlackholeState] {
	return &subnetBlackholeAction{
		clientProvider:             defaultClientProviderSubnetBlackhole,
		extensionRootAccountNumber: utils.GetRootAccountNumber(),
	}
}

func (e *subnetBlackholeAction) NewEmptyState() BlackholeState {
	return BlackholeState{}
}

func (e *subnetBlackholeAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          subnetBlackholeActionId,
		Label:       "Blackhole Subnet",
		Description: "Block traffic for a given subnet.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(subnetIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: subnetTargetType,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "vpc",
					Description: new("Find subnets by vpc"),
					Query:       "aws.vpc.name=\"\"",
				},
				{
					Label:       "zone",
					Description: new("Find subnets by zone"),
					Query:       "aws.zone=\"\"",
				},
				{
					Label:       "subnet",
					Description: new("Find subnets by name"),
					Query:       "aws.ec2.subnet.name=\"\"",
				},
			})}),
		Technology:  new("AWS"),
		Category:    new("Network"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new(""),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("60s"),
				Order:        new(1),
				Required:     new(true),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func (e *subnetBlackholeAction) Prepare(ctx context.Context, state *BlackholeState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return prepareBlackhole(ctx, state, request, e.extensionRootAccountNumber, e.clientProvider, getTargetSubnetsForBlackholeSubnet)
}

func getTargetSubnetsForBlackholeSubnet(_ blackholeEC2Api, _ context.Context, target *action_kit_api.Target) (map[string][]string, error) {
	subnetResults := make(map[string][]string)
	subnetResults[extutil.MustHaveValue(target.Attributes, "aws.vpc.id")[0]] = []string{extutil.MustHaveValue(target.Attributes, "aws.ec2.subnet.id")[0]}
	return subnetResults, nil
}

func (e *subnetBlackholeAction) Start(ctx context.Context, state *BlackholeState) (*action_kit_api.StartResult, error) {
	return startBlackhole(ctx, state, e.clientProvider)
}

func (e *subnetBlackholeAction) Stop(ctx context.Context, state *BlackholeState) (*action_kit_api.StopResult, error) {
	return stopBlackhole(ctx, state, e.clientProvider)
}

func defaultClientProviderSubnetBlackhole(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error) {
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
