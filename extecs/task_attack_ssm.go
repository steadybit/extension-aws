// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

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
	"github.com/steadybit/extension-kit/extbuild"
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
	Parameters        map[string][]string
	Comment           string
	CommandEnded      bool
}

type ecsTaskSsmApi interface {
	SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	CancelCommand(ctx context.Context, params *ssm.CancelCommandInput, optFns ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error)
	DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
	ssm.GetCommandInvocationAPIClient
}

type ssmCommandInvocation struct {
	documentVersion  string
	documentName     string
	getParameters    func(action_kit_api.PrepareActionRequestBody) (map[string][]string, error)
	stepNameToOutput string
}

func newEcsTaskSsmAction(makeDescription func() action_kit_api.ActionDescription, invocation ssmCommandInvocation) action_kit_sdk.ActionWithStop[TaskSsmActionState] {
	description := makeDescription()
	description.Version = extbuild.GetSemverVersionStringOrUnknown()
	description.Icon = extutil.Ptr(ecsTaskIcon)
	description.TargetSelection = &action_kit_api.TargetSelection{
		TargetType: ecsTaskTargetId,
		SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
			{
				Label:       "by service and cluster",
				Description: extutil.Ptr("Find ecs task by cluster and service name"),
				Query:       "aws-ecs.cluster.name=\"\" and aws-ecs.service.name=\"\" and aws-ecs.task.amazon-ssm-agent=\"true\"",
			},
		}),
	}
	description.Category = extutil.Ptr("resource")
	description.Kind = action_kit_api.Attack
	description.TimeControl = action_kit_api.TimeControlInternal
	description.Status = &action_kit_api.MutatingEndpointReferenceWithCallInterval{
		CallInterval: extutil.Ptr("5s"),
	}
	return &ecsTaskSsmAction{
		clientProvider:       defaultTaskSsmClientProvider,
		description:          description,
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
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.TaskArn = extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.task.arn")[0]

	if parameters, err := e.ssmCommandInvocation.getParameters(request); err == nil {
		state.Parameters = parameters
	} else {
		return nil, err
	}

	if request.ExecutionContext != nil && request.ExecutionContext.ExecutionId != nil && request.ExecutionContext.ExperimentKey != nil {
		state.Comment = fmt.Sprintf("Steadybit Experiment %s #%d", *request.ExecutionContext.ExperimentKey, *request.ExecutionContext.ExecutionId)
	} else {
		state.Comment = "Steadybit Experiment"
	}

	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, err
	}

	if managedInstanceId, err := e.findManagedInstance(ctx, client, state.TaskArn); err == nil {
		state.ManagedInstanceId = managedInstanceId
	} else {
		prepareErr := extension_kit.ToError(fmt.Sprintf("Failed to find managed instance for ECS Task %s", state.TaskArn), err)
		if errors.Is(err, errorManagedInstanceNotFound) {
			prepareErr.Detail = extutil.Ptr("Please make sure that the 'amazon-ssm-agent' is added to the task definition and running.")
		}
		return nil, prepareErr
	}

	return nil, nil
}

func (e *ecsTaskSsmAction) Start(ctx context.Context, state *TaskSsmActionState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, err
	}

	output, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName:    &e.ssmCommandInvocation.documentName,
		DocumentVersion: &e.ssmCommandInvocation.documentVersion,
		InstanceIds:     []string{state.ManagedInstanceId},
		Parameters:      state.Parameters,
		Comment:         extutil.Ptr(shorten(state.Comment, 100)),
		TimeoutSeconds:  extutil.Ptr(int32(30)),
	})

	result := &action_kit_api.StartResult{Messages: &[]action_kit_api.Message{}}
	if err == nil {
		state.CommandId = *output.Command.CommandId
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Message: fmt.Sprintf("Sent SSM command (%s) on ECS Task %s using document %s(%s) parameters %+v", state.CommandId, state.TaskArn, e.ssmCommandInvocation.documentName, e.ssmCommandInvocation.documentVersion, state.Parameters),
		}))
	} else {
		result.Error = &action_kit_api.ActionKitError{
			Title:  fmt.Sprintf("Failed to start %s on ECS Task %s", e.description.Label, state.TaskArn),
			Detail: extutil.Ptr(fmt.Sprintf("Sending SSM command on ECS Task %s failed. Using document %s(%s) and parameters %+v: %s", state.TaskArn, e.ssmCommandInvocation.documentName, e.ssmCommandInvocation.documentVersion, state.Parameters, err.Error())),
		}
	}

	return result, nil
}

func shorten(s string, i int) string {
	if len(s) > i {
		return s[:i]
	}
	return s
}

func (e *ecsTaskSsmAction) Status(ctx context.Context, state *TaskSsmActionState) (*action_kit_api.StatusResult, error) {
	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, err
	}

	ciOutput, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{CommandId: &state.CommandId, InstanceId: &state.ManagedInstanceId})
	if err != nil {
		if isErrInvocationDoesNotExist(err) {
			return nil, nil
		} else {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed get status for %s on ECS Task %s", e.description.Label, state.TaskArn), err)
		}
	}

	if hasEnded(ciOutput) {
		state.CommandEnded = true
		return &action_kit_api.StatusResult{Completed: true}, nil
	}

	//As the command will be stuck "InProgress" if the executing managed instance has vanished, we need to check if it still there, so we don't wait on the command timeout.
	if _, err := e.findManagedInstance(ctx, client, state.TaskArn); err != nil {
		if errors.Is(err, errorManagedInstanceNotFound) {
			return &action_kit_api.StatusResult{Completed: true}, nil
		} else {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to find managed instance for %s on ECS Task %s", e.description.Label, state.TaskArn), err)
		}
	}

	return nil, nil
}

func (e *ecsTaskSsmAction) Stop(ctx context.Context, state *TaskSsmActionState) (*action_kit_api.StopResult, error) {
	client, err := e.clientProvider(state.Account)
	if err != nil {
		return nil, err
	}

	result := &action_kit_api.StopResult{Messages: &[]action_kit_api.Message{}}
	if state.CommandId == "" {
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Message: fmt.Sprintf("No SSM command to cancel for %s on ECS Task %s", e.description.Label, state.TaskArn),
		}))
		return result, nil
	}

	if !state.CommandEnded {
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Message: fmt.Sprintf("Cancelling SSM command (%s) for %s on ECS Task %s", state.CommandId, e.description.Label, state.TaskArn),
		}))

		if _, err := client.CancelCommand(ctx, &ssm.CancelCommandInput{CommandId: &state.CommandId, InstanceIds: []string{state.ManagedInstanceId}}); err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to cancel SSM command (%s) for %s on ECS Task %s", state.CommandId, e.description.Label, state.TaskArn), err)
		}
	}

	output, err := ssm.NewCommandExecutedWaiter(client, withCommandStatusRetryable()).WaitForOutput(ctx, &ssm.GetCommandInvocationInput{CommandId: &state.CommandId, InstanceId: &state.ManagedInstanceId}, 10*time.Second)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to await end of %s on ECS Task %s", e.description.Label, state.TaskArn), err)
	}
	if output.Status != types.CommandInvocationStatusSuccess {
		result.Error = &action_kit_api.ActionKitError{
			Title: fmt.Sprintf("Ended SSM command %s on ECS Task %s with status %s", e.description.Label, state.TaskArn, *output.StatusDetails),
		}
	}
	result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
		Message: fmt.Sprintf("SSM command (%s) using document %s(%s) ended with rc=%d and status %s", state.CommandId, *output.DocumentName, *output.DocumentVersion, output.ResponseCode, *output.StatusDetails),
	}))

	//when the step hasn't run yet, an error status is returned, hence we wait the command status and the step output separately
	output, err = client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{CommandId: &state.CommandId, InstanceId: &state.ManagedInstanceId, PluginName: &e.ssmCommandInvocation.stepNameToOutput})
	if err != nil {
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Level:   extutil.Ptr(action_kit_api.Warn),
			Message: fmt.Sprintf("Failed to read output for step %s: %v", e.ssmCommandInvocation.stepNameToOutput, err),
		}))
	}
	if output.StandardOutputContent != nil {
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Message: fmt.Sprintf("%s stdout:\n%s", state.CommandId, *output.StandardOutputContent),
		}))
	}
	if output.StandardErrorContent != nil {
		result.Messages = extutil.Ptr(append(*result.Messages, action_kit_api.Message{
			Message: fmt.Sprintf("%s stderr:\n%s", state.CommandId, *output.StandardErrorContent),
		}))
	}

	return result, nil
}

func hasEnded(ciOutput *ssm.GetCommandInvocationOutput) bool {
	return ciOutput != nil && (ciOutput.Status == types.CommandInvocationStatusSuccess || ciOutput.Status == types.CommandInvocationStatusFailed || ciOutput.Status == types.CommandInvocationStatusTimedOut || ciOutput.Status == types.CommandInvocationStatusCancelled)
}

func withCommandStatusRetryable() func(options *ssm.CommandExecutedWaiterOptions) {
	return func(options *ssm.CommandExecutedWaiterOptions) {
		options.Retryable = func(ctx context.Context, input *ssm.GetCommandInvocationInput, output *ssm.GetCommandInvocationOutput, err error) (bool, error) {
			if err != nil {
				if isErrInvocationDoesNotExist(err) {
					return true, nil
				} else if isErrInvalidPluginName(err) {
					return false, nil
				}
			}
			return !hasEnded(output), nil
		}
	}
}

func isErrInvocationDoesNotExist(err error) bool {
	var errorType *types.InvocationDoesNotExist
	return errors.As(err, &errorType)
}

func isErrInvalidPluginName(err error) bool {
	var errorType *types.InvalidPluginName
	return errors.As(err, &errorType)
}

func (e *ecsTaskSsmAction) findManagedInstance(ctx context.Context, client ecsTaskSsmApi, taskArn string) (string, error) {
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
