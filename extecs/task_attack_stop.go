// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type ecsTaskStopAction struct {
	clientProvider func(account string, region string, role *string) (ecsTaskStopApi, error)
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[TaskStopState] = (*ecsTaskStopAction)(nil)

type TaskStopState struct {
	Account          string
	Region           string
	DiscoveredByRole *string
	TaskArn          string
	ClusterArn       string
}

type ecsTaskStopApi interface {
	ecs.DescribeTasksAPIClient
	StopTask(ctx context.Context, params *ecs.StopTaskInput, optFns ...func(*ecs.Options)) (*ecs.StopTaskOutput, error)
}

func NewEcsTaskStopAction() action_kit_sdk.Action[TaskStopState] {
	return &ecsTaskStopAction{defaultTaskStopClientProvider}
}

func (e *ecsTaskStopAction) NewEmptyState() TaskStopState {
	return TaskStopState{}
}

func (e *ecsTaskStopAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.stop", ecsTaskTargetId),
		Label:       "Stop Task",
		Description: "Stop an ECS task",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("ECS"),
		Icon:        extutil.Ptr(ecsTaskIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: ecsTaskTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "cluster and service",
					Description: extutil.Ptr("Find ecs task by cluster and service name"),
					Query:       "aws-ecs.cluster.name=\"\" and aws-ecs.service.name=\"\"",
				},
			}),
		}),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (e *ecsTaskStopAction) Prepare(_ context.Context, state *TaskStopState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	state.ClusterArn = extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.cluster.arn")[0]
	state.TaskArn = extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.task.arn")[0]
	return nil, nil
}

func (e *ecsTaskStopAction) Start(ctx context.Context, state *TaskStopState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize ECS client for AWS account %s", state.Account), err)
	}

	describeTaskResult, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &state.ClusterArn,
		Tasks:   []string{state.TaskArn},
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe ecs task '%s'.", state.TaskArn), err)
	}

	if len(describeTaskResult.Tasks) == 1 && aws.ToString(describeTaskResult.Tasks[0].LastStatus) != "RUNNING" {
		return &action_kit_api.StartResult{
			Error: &action_kit_api.ActionKitError{
				Detail: extutil.Ptr(fmt.Sprintf("State of task %s was %s", state.TaskArn, aws.ToString(describeTaskResult.Tasks[0].LastStatus))),
				Status: extutil.Ptr(action_kit_api.Failed),
				Title:  "Task not running",
			},
		}, nil
	}

	_, err = client.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: &state.ClusterArn,
		Task:    &state.TaskArn,
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to stop ecs task '%s'.", state.TaskArn), err)
	}

	return nil, nil
}

func defaultTaskStopClientProvider(account string, region string, role *string) (ecsTaskStopApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return ecs.NewFromConfig(awsAccess.AwsConfig), nil
}
