// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type ec2InstanceStateAction struct {
	clientProvider func(account string, region string, role *string) (ec2InstanceStateChangeApi, error)
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[InstanceStateChangeState] = (*ec2InstanceStateAction)(nil)

type InstanceStateChangeState struct {
	Account          string
	Region           string
	DiscoveredByRole *string
	InstanceId       string
	Action           string
}

type ec2InstanceStateChangeApi interface {
	StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error)
	StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
}

func NewEc2InstanceStateAction() action_kit_sdk.Action[InstanceStateChangeState] {
	return &ec2InstanceStateAction{defaultClientProviderInstanceState}
}

func (e *ec2InstanceStateAction) NewEmptyState() InstanceStateChangeState {
	return InstanceStateChangeState{}
}

func (e *ec2InstanceStateAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          ec2InstanceStateActionId,
		Label:       "Change Instance State",
		Description: "Reboot, terminate, stop or hibernate EC2 instances",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(ec2Icon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: ec2TargetType,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "instance-id",
					Description: extutil.Ptr("Find ec2-instance by instance-id"),
					Query:       "aws-ec2.instance.id=\"\"",
				},
				{
					Label:       "instance-name",
					Description: extutil.Ptr("Find ec2-instance by instance-name"),
					Query:       "aws-ec2.instance.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("EC2"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:        "action",
				Label:       "Action",
				Description: extutil.Ptr("The kind of state change operation to execute for the EC2 instances"),
				Required:    extutil.Ptr(true),
				Type:        action_kit_api.String,
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Reboot",
						Value: "reboot",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Stop",
						Value: "stop",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Hibernate",
						Value: "hibernate",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Terminate",
						Value: "terminate",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Start",
						Value: "start",
					},
				}),
			},
		},
	}
}

func (e *ec2InstanceStateAction) Prepare(_ context.Context, state *InstanceStateChangeState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	action := request.Config["action"]
	if action == nil {
		return nil, extension_kit.ToError("Missing attack action parameter.", nil)
	}

	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	state.InstanceId = extutil.MustHaveValue(request.Target.Attributes, "aws-ec2.instance.id")[0]
	state.Action = action.(string)
	return nil, nil
}

func (e *ec2InstanceStateAction) Start(ctx context.Context, state *InstanceStateChangeState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.Account), err)
	}

	instanceIds := []string{state.InstanceId}

	if state.Action == "reboot" {
		in := ec2.RebootInstancesInput{
			InstanceIds: instanceIds,
		}
		_, err = client.RebootInstances(ctx, &in)
	} else if state.Action == "stop" {
		in := ec2.StopInstancesInput{
			InstanceIds: instanceIds,
			Hibernate:   extutil.Ptr(false),
		}
		_, err = client.StopInstances(ctx, &in)
	} else if state.Action == "hibernate" {
		in := ec2.StopInstancesInput{
			InstanceIds: instanceIds,
			Hibernate:   extutil.Ptr(true),
		}
		_, err = client.StopInstances(ctx, &in)
	} else if state.Action == "terminate" {
		in := ec2.TerminateInstancesInput{
			InstanceIds: instanceIds,
		}
		_, err = client.TerminateInstances(ctx, &in)
	} else if state.Action == "start" {
		in := ec2.StartInstancesInput{
			InstanceIds: instanceIds,
		}
		_, err = client.StartInstances(ctx, &in)
	}

	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to execute state change attack '%s' on instance '%s'", state.Action, state.InstanceId), err)
	}

	return nil, nil
}

func defaultClientProviderInstanceState(account string, region string, role *string) (ec2InstanceStateChangeApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return ec2.NewFromConfig(awsAccess.AwsConfig), nil
}
