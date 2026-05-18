// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extasg

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type asgSuspendProcessesAttack struct {
	clientProvider func(account string, region string, role *string) (AsgApi, error)
}

var (
	_ action_kit_sdk.Action[AsgAttackState]         = (*asgSuspendProcessesAttack)(nil)
	_ action_kit_sdk.ActionWithStop[AsgAttackState] = (*asgSuspendProcessesAttack)(nil)
)

func NewAsgSuspendProcessesAttack() action_kit_sdk.ActionWithStop[AsgAttackState] {
	return &asgSuspendProcessesAttack{clientProvider: defaultAsgClientProvider}
}

func (a *asgSuspendProcessesAttack) NewEmptyState() AsgAttackState {
	return AsgAttackState{}
}

func (a *asgSuspendProcessesAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.suspend-processes", asgTargetId),
		Label:       "Suspend Auto Scaling Processes",
		Description: "Suspends one or more Auto Scaling processes (Launch, HealthCheck, ReplaceUnhealthy, ...) for the duration of the experiment. Processes are resumed on stop.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(asgIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: asgTargetId,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by Auto Scaling group name",
					Description: new("Find Auto Scaling group by name"),
					Query:       "aws.asg.name=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("Auto Scaling"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("Duration of the suspension. Processes will be resumed when the experiment stops."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("60s"),
				Order:        new(1),
				Required:     new(true),
			},
			{
				Name:        "processes",
				Label:       "Processes to suspend",
				Description: new("Select the Auto Scaling processes to suspend. See AWS docs for the effect of each process."),
				Type:        action_kit_api.ActionParameterTypeStringArray,
				// JSON-array literal: ActionParameterTypeStringArray ("string_array") expects DefaultValue to
				// be a JSON-array string. A comma-joined default is flagged as "invalid option value" by the UI
				// because the whole string gets matched against the Options list as a single value.
				DefaultValue: new(`["Launch","HealthCheck","ReplaceUnhealthy"]`),
				Order:        new(2),
				Required:     new(true),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{Label: "Launch", Value: "Launch"},
					action_kit_api.ExplicitParameterOption{Label: "Terminate", Value: "Terminate"},
					action_kit_api.ExplicitParameterOption{Label: "HealthCheck", Value: "HealthCheck"},
					action_kit_api.ExplicitParameterOption{Label: "ReplaceUnhealthy", Value: "ReplaceUnhealthy"},
					action_kit_api.ExplicitParameterOption{Label: "AZRebalance", Value: "AZRebalance"},
					action_kit_api.ExplicitParameterOption{Label: "AlarmNotification", Value: "AlarmNotification"},
					action_kit_api.ExplicitParameterOption{Label: "ScheduledActions", Value: "ScheduledActions"},
					action_kit_api.ExplicitParameterOption{Label: "AddToLoadBalancer", Value: "AddToLoadBalancer"},
					action_kit_api.ExplicitParameterOption{Label: "InstanceRefresh", Value: "InstanceRefresh"},
				}),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *asgSuspendProcessesAttack) Prepare(_ context.Context, state *AsgAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.AutoScalingGroupName = extutil.MustHaveValue(request.Target.Attributes, "aws.asg.name")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")

	alreadySuspended := request.Target.Attributes["aws.asg.suspended-processes"]
	suspendedSet := make(map[string]bool, len(alreadySuspended))
	for _, p := range alreadySuspended {
		suspendedSet[p] = true
	}

	requested := extutil.ToStringArray(request.Config["processes"])
	if len(requested) == 0 {
		return nil, extension_kit.ToError("No processes selected to suspend.", nil)
	}

	toSuspend := make([]string, 0, len(requested))
	for _, p := range requested {
		if !suspendedSet[p] {
			toSuspend = append(toSuspend, p)
		}
	}
	state.SuspendedProcesses = toSuspend

	if len(toSuspend) == 0 {
		return &action_kit_api.PrepareResult{
			Messages: extutil.Ptr([]action_kit_api.Message{{
				Level:   extutil.Ptr(action_kit_api.Warn),
				Message: fmt.Sprintf("All requested processes were already suspended on Auto Scaling group %s. Stop will not resume them.", state.AutoScalingGroupName),
			}}),
		}, nil
	}
	return nil, nil
}

func (a *asgSuspendProcessesAttack) Start(ctx context.Context, state *AsgAttackState) (*action_kit_api.StartResult, error) {
	if len(state.SuspendedProcesses) == 0 {
		return nil, nil
	}
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Auto Scaling client for AWS account %s", state.Account), err)
	}
	_, err = client.SuspendProcesses(ctx, &autoscaling.SuspendProcessesInput{
		AutoScalingGroupName: &state.AutoScalingGroupName,
		ScalingProcesses:     state.SuspendedProcesses,
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to suspend processes %v on Auto Scaling group %s", state.SuspendedProcesses, state.AutoScalingGroupName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Suspended processes %v on Auto Scaling group %s", state.SuspendedProcesses, state.AutoScalingGroupName),
		}}),
	}, nil
}

func (a *asgSuspendProcessesAttack) Stop(ctx context.Context, state *AsgAttackState) (*action_kit_api.StopResult, error) {
	if len(state.SuspendedProcesses) == 0 {
		return nil, nil
	}
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Auto Scaling client for AWS account %s", state.Account), err)
	}
	_, err = client.ResumeProcesses(ctx, &autoscaling.ResumeProcessesInput{
		AutoScalingGroupName: &state.AutoScalingGroupName,
		ScalingProcesses:     state.SuspendedProcesses,
	})
	if err != nil {
		log.Error().Err(err).Msgf("Failed to resume processes %v on Auto Scaling group %s", state.SuspendedProcesses, state.AutoScalingGroupName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to resume processes %v on Auto Scaling group %s", state.SuspendedProcesses, state.AutoScalingGroupName), err)
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Resumed processes %v on Auto Scaling group %s", state.SuspendedProcesses, state.AutoScalingGroupName),
		}}),
	}, nil
}
