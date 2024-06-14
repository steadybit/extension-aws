// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type ecsTaskStopAction struct {
	clientProvider func(account string) (ecsTaskStopApi, error)
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[TaskStopState] = (*ecsTaskStopAction)(nil)

type TaskStopState struct {
	Account    string
	TaskArn    string
	ClusterArn string
}

type ecsTaskStopApi interface {
	StopTask(ctx context.Context, params *ecs.StopTaskInput, optFns ...func(*ecs.Options)) (*ecs.StopTaskOutput, error)
}

func NewEcsTaskStopAction() action_kit_sdk.Action[TaskStopState] {
	return &ecsTaskStopAction{defaultClientProvider}
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
		Icon:        extutil.Ptr(ecsTaskIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: ecsTaskTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by service and cluster",
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
	account := request.Target.Attributes["aws.account"]
	clusterArn := request.Target.Attributes["aws-ecs.cluster.arn"]
	taskArn := request.Target.Attributes["aws-ecs.task.arn"]

	state.Account = account[0]
	state.ClusterArn = clusterArn[0]
	state.TaskArn = taskArn[0]
	return nil, nil
}

func (e *ecsTaskStopAction) Start(ctx context.Context, state *TaskStopState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize ECS client for AWS account %s", state.Account), err)
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

func defaultClientProvider(account string) (ecsTaskStopApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return ecs.NewFromConfig(awsAccount.AwsConfig), nil
}
