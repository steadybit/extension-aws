// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type ecsServiceScaleAction struct {
	clientProvider func(account string, region string, role *string) (ecsServiceScaleApi, error)
}

// Make sure action implements all required interfaces
var _ action_kit_sdk.Action[ServiceScaleState] = (*ecsServiceScaleAction)(nil)
var _ action_kit_sdk.ActionWithStop[ServiceScaleState] = (*ecsServiceScaleAction)(nil)

type ServiceScaleState struct {
	Account             string
	Region              string
	DiscoveredByRole    *string
	ServiceName         string
	ClusterArn          string
	DesiredCount        int32
	InitialDesiredCount int32
}

type ecsServiceScaleApi interface {
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
	ecs.DescribeServicesAPIClient
}

func NewEcsServiceScaleAction() action_kit_sdk.Action[ServiceScaleState] {
	return &ecsServiceScaleAction{defaultClientProviderService}
}

func (e *ecsServiceScaleAction) NewEmptyState() ServiceScaleState {
	return ServiceScaleState{}
}

func (e *ecsServiceScaleAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.scale", ecsServiceTargetId),
		Label:       "Scale Service",
		Description: "Up-/ or downscale an ECS service",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("ECS"),
		Icon:        extutil.Ptr(ecsServiceIcon),
		Kind:        action_kit_api.Attack,
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: ecsServiceTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "cluster and service",
					Description: extutil.Ptr("Find ecs service by cluster and service name"),
					Query:       "aws-ecs.cluster.name=\"\" and aws-ecs.service.name=\"\"",
				},
			}),
		}),
		TimeControl: action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Description:  extutil.Ptr("The duration of the action. The service will be scaled back to the original value after the action."),
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("180s"),
				Required:     extutil.Ptr(true),
			},
			{
				Name:         "desiredCount",
				Label:        "Desired Count",
				Description:  extutil.Ptr("The new desired count."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: extutil.Ptr("1"),
				Required:     extutil.Ptr(true),
			},
		},
	}
}

func (e *ecsServiceScaleAction) Prepare(ctx context.Context, state *ServiceScaleState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	state.ClusterArn = extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.cluster.arn")[0]
	state.ServiceName = extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.service.name")[0]
	state.DesiredCount = extutil.ToInt32(request.Config["desiredCount"])

	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize ECS client for AWS account %s", state.Account), err)
	}

	serviceFetchResult, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Services: []string{state.ServiceName},
		Cluster:  &state.ClusterArn,
	})
	if err != nil || len(serviceFetchResult.Services) != 1 {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to fetch ecs service '%s'", state.ServiceName), err)
	}
	state.InitialDesiredCount = serviceFetchResult.Services[0].DesiredCount
	return nil, nil
}

func (e *ecsServiceScaleAction) Start(ctx context.Context, state *ServiceScaleState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize ECS client for AWS account %s", state.Account), err)
	}

	_, err = client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &state.ClusterArn,
		Service:      &state.ServiceName,
		DesiredCount: &state.DesiredCount,
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to scale ecs service '%s'.", state.ServiceName), err)
	}

	return nil, nil
}

func (e *ecsServiceScaleAction) Stop(ctx context.Context, state *ServiceScaleState) (*action_kit_api.StopResult, error) {
	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize ECS client for AWS account %s", state.Account), err)
	}
	_, err = client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &state.ClusterArn,
		Service:      &state.ServiceName,
		DesiredCount: &state.InitialDesiredCount,
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to scale ecs service '%s'.", state.ServiceName), err)
	}
	return nil, nil
}

func defaultClientProviderService(account string, region string, role *string) (ecsServiceScaleApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return ecs.NewFromConfig(awsAccess.AwsConfig), nil
}
