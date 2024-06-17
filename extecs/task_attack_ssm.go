/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extecs

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"time"
)

type ecsTaskSsmAction struct {
	clientProvider       func(account string) (ecsTaskSsmApi, error)
	description          action_kit_api.ActionDescription
	ssmCommandInvocation ssmCommandInvocation
}

// Make sure lambdaAction implements all required interfaces
var (
	_ action_kit_sdk.ActionWithStop[TaskSsmActionState]   = (*ecsTaskSsmAction)(nil)
	_ action_kit_sdk.ActionWithStatus[TaskSsmActionState] = (*ecsTaskSsmAction)(nil)

	errorManagedInstanceNotFound  = errors.New("managed instance for ECS Task not found")
	errorManagedInstanceAmbiguous = errors.New("found multiple managed instances for ECS Task")
)

type TaskSsmActionState struct {
	Account           string
	TaskArn           string
	ManagedInstanceId string
	CommandId         string
	DocumentVersion   string
	DocumentName      string
	Parameters        map[string][]string
	StepNameForStatus string
	Comment           string
}

type ecsTaskSsmApi interface {
	SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	CancelCommand(ctx context.Context, params *ssm.CancelCommandInput, optFns ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error)
	DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
	ssm.GetCommandInvocationAPIClient
}

type ssmCommandInvocation struct {
	documentVersion   string
	documentName      string
	getParameters     func(action_kit_api.PrepareActionRequestBody) (map[string][]string, error)
	stepNameForStatus string
}

func newEcsTaskSsmAction(description func() action_kit_api.ActionDescription, invocation ssmCommandInvocation) action_kit_sdk.ActionWithStop[TaskSsmActionState] {
	return &ecsTaskSsmAction{
		clientProvider:       defaultTaskSsmClientProvider,
		description:          description(),
		ssmCommandInvocation: invocation,
	}
}

func (e *ecsTaskSsmAction) NewEmptyState() TaskSsmActionState {
	return TaskSsmActionState{}
}

func (e *ecsTaskSsmAction) Describe() action_kit_api.ActionDescription {
	return e.description
}

func (e *ecsTaskSsmAction) Prepare(ctx context.Context, state *TaskSsmActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	if account := request.Target.Attributes["aws.account"]; len(account) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'aws.account' attribute.", nil)
	} else {
		state.Account = account[0]
	}

	if taskArn := request.Target.Attributes["aws-ecs.task.arn"]; len(taskArn) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'aws-ecs.task.arn' attribute.", nil)
	} else {
		state.TaskArn = taskArn[0]
	}

	if managedInstanceId, err := e.findManagedInstance(ctx, state.Account, state.TaskArn); err != nil {
		prepareErr := extension_kit.ToError(fmt.Sprintf("Failed to find managed instance for ECS Task %s", state.TaskArn), err)
		if errors.Is(err, errorManagedInstanceNotFound) {
			prepareErr.Detail = extutil.Ptr("Please make sure that the 'amazon-ssm-agent' is added to the task definition and running.")
		}
		return nil, prepareErr
	} else {
		state.ManagedInstanceId = managedInstanceId
	}

	if parameters, err := e.ssmCommandInvocation.getParameters(request); err != nil {
		return nil, err
	} else {
		state.DocumentName = e.ssmCommandInvocation.documentName
		state.DocumentVersion = e.ssmCommandInvocation.documentVersion
		state.Parameters = parameters
		state.StepNameForStatus = e.ssmCommandInvocation.stepNameForStatus
		if request.ExecutionContext != nil && request.ExecutionContext.ExecutionId != nil && request.ExecutionContext.ExperimentKey != nil && request.ExecutionContext.ExecutionUri != nil {
			state.Comment = fmt.Sprintf("Run by Steadybit for experiment %s #%d %s", *request.ExecutionContext.ExperimentKey, *request.ExecutionContext.ExecutionId, *request.ExecutionContext.ExecutionUri)
		} else {
			state.Comment = "Run by Steadybit"
		}
	}

	return nil, nil
}

func (e *ecsTaskSsmAction) Start(ctx context.Context, state *TaskSsmActionState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, err
	}

	output, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName:    &state.DocumentName,
		DocumentVersion: &state.DocumentVersion,
		InstanceIds:     []string{state.ManagedInstanceId},
		Parameters:      state.Parameters,
		Comment:         extutil.Ptr(state.Comment),
		TimeoutSeconds:  extutil.Ptr(int32(30)),
	})

	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed start %s on ECS Task %s", e.description.Label, state.TaskArn), err)
	} else {
		state.CommandId = *output.Command.CommandId
	}

	return nil, nil
}

func (e *ecsTaskSsmAction) Status(ctx context.Context, state *TaskSsmActionState) (*action_kit_api.StatusResult, error) {
	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, err
	}

	output, err := client.GetCommandInvocation(ctx, e.createCommandInvocationInput(state))
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed get status for %s on ECS Task %s", e.description.Label, state.TaskArn), err)
	}

	if output.Status == types.CommandInvocationStatusSuccess ||
		output.Status == types.CommandInvocationStatusFailed ||
		output.Status == types.CommandInvocationStatusTimedOut ||
		output.Status == types.CommandInvocationStatusCancelled {
		return &action_kit_api.StatusResult{Completed: true}, nil
	}

	return nil, nil
}

func (e *ecsTaskSsmAction) createCommandInvocationInput(state *TaskSsmActionState) *ssm.GetCommandInvocationInput {
	input := &ssm.GetCommandInvocationInput{
		CommandId:  &state.CommandId,
		InstanceId: &state.ManagedInstanceId,
	}
	if state.StepNameForStatus != "" {
		input.PluginName = &state.StepNameForStatus
	}
	return input
}

func (e *ecsTaskSsmAction) Stop(ctx context.Context, state *TaskSsmActionState) (*action_kit_api.StopResult, error) {
	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, err
	}

	_, err = client.CancelCommand(ctx, &ssm.CancelCommandInput{
		CommandId:   &state.CommandId,
		InstanceIds: []string{state.ManagedInstanceId},
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to cancel %s on ECS Task %s", e.description.Label, state.TaskArn), err)
	}

	output, err := ssm.NewCommandExecutedWaiter(client).WaitForOutput(ctx, e.createCommandInvocationInput(state), 10*time.Second)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to await end of %s on ECS Task %s", e.description.Label, state.TaskArn), err)
	}

	result := action_kit_api.StopResult{Messages: &[]action_kit_api.Message{}}
	if output.Status != types.CommandInvocationStatusSuccess {
		result.Error = &action_kit_api.ActionKitError{
			Title: fmt.Sprintf("Ended %s on ECS Task %s with %s", e.description.Label, state.TaskArn, *output.StatusDetails),
		}
	}

	result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
		Level:   extutil.Ptr(action_kit_api.Info),
		Message: fmt.Sprintf("%s(%s) RC=%d", *output.DocumentName, *output.DocumentVersion, output.ResponseCode),
	}))
	if output.StandardOutputContent != nil {
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("stdout:\n%s", *output.StandardOutputContent),
		}))
	}
	if output.StandardErrorContent != nil {
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Level:   extutil.Ptr(action_kit_api.Error),
			Message: fmt.Sprintf("stderr:\n%s", *output.StandardErrorContent),
		}))
	}

	return &result, nil
}

func (e *ecsTaskSsmAction) findManagedInstance(ctx context.Context, account, taskArn string) (string, error) {
	client, err := e.clientProvider(account)
	if err != nil {
		return "", extension_kit.ToError(fmt.Sprintf("Failed to initialize ECS client for AWS account %s", account), err)
	}

	output, err := client.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []types.InstanceInformationStringFilter{
			{Key: extutil.Ptr("tag:ECS_TASK_ARN"), Values: []string{taskArn}},
		},
	})
	if err != nil {
		return "", err
	}

	if len(output.InstanceInformationList) == 1 && output.InstanceInformationList[0].InstanceId != nil {
		return *output.InstanceInformationList[0].InstanceId, nil
	} else if len(output.InstanceInformationList) > 1 {
		return "", errorManagedInstanceAmbiguous
	} else {
		return "", errorManagedInstanceNotFound
	}
}

func defaultTaskSsmClientProvider(account string) (ecsTaskSsmApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return ssm.NewFromConfig(awsAccount.AwsConfig), nil
}
