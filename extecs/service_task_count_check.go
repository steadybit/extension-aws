// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extensionkit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/extutil"
	"time"
)

const (
	runningCountMin1                 = "runningCountMin1"
	runningCountEqualsDesiredCount   = "runningCountEqualsDesiredCount"
	runningCountLessThanDesiredCount = "runningCountLessThanDesiredCount"
	runningCountDecreased            = "runningCountDecreased"
	runningCountIncreased            = "runningCountIncreased"
)

type EcsServiceTaskCountCheckState struct {
	Timeout               time.Time
	RunningCountCheckMode string
	ServiceArn            string
	ClusterArn            string
	AwsAccount            string
	InitialRunningCount   int
}

type EcsServiceTaskCountCheckConfig struct {
	Duration              int
	RunningCountCheckMode string
}

type escServiceTaskCounts struct {
	running int
	desired int
}

type EcsServiceTaskCountCheckAction struct {
	poller ServiceDescriptionPoller
}

var _ action_kit_sdk.Action[EcsServiceTaskCountCheckState] = (*EcsServiceTaskCountCheckAction)(nil)
var _ action_kit_sdk.ActionWithStatus[EcsServiceTaskCountCheckState] = (*EcsServiceTaskCountCheckAction)(nil)
var _ action_kit_sdk.ActionWithStop[EcsServiceTaskCountCheckState] = (*EcsServiceTaskCountCheckAction)(nil)

func NewEcsServiceTaskCountCheckAction(poller ServiceDescriptionPoller) action_kit_sdk.Action[EcsServiceTaskCountCheckState] {
	return EcsServiceTaskCountCheckAction{
		poller: poller,
	}
}

func (f EcsServiceTaskCountCheckAction) NewEmptyState() EcsServiceTaskCountCheckState {
	return EcsServiceTaskCountCheckState{}
}

func (f EcsServiceTaskCountCheckAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          ecsServiceTaskCountCheckActionId,
		Label:       "Service Task Count",
		Description: "Verify service task counts.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(ecsServiceIcon),
		Category:    extutil.Ptr("AWS ECS"),
		Kind:        action_kit_api.Check,
		TimeControl: action_kit_api.TimeControlInternal,
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType:          ecsServiceTargetId,
			QuantityRestriction: extutil.Ptr(action_kit_api.All),
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "default",
					Description: extutil.Ptr("Find service by cluster and service name"),
					Query:       "aws-ecs.cluster.name=\"\" AND aws-ecs.service.name=\"\"",
				},
			}),
		}),
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Timeout",
				Description:  extutil.Ptr("How long the check should wait for the specified service task count."),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("10s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
			{
				Name:         "runningCountCheckMode",
				Label:        "Service task count",
				Description:  extutil.Ptr("How many running tasks are required to let the check pass."),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(runningCountEqualsDesiredCount),
				Order:        extutil.Ptr(2),
				Required:     extutil.Ptr(true),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "running count > 0",
						Value: runningCountMin1,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "running count = desired count",
						Value: runningCountEqualsDesiredCount,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "running count < desired count",
						Value: runningCountLessThanDesiredCount,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "running count increases",
						Value: runningCountIncreased,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "running count decreases",
						Value: runningCountDecreased,
					},
				}),
			},
		},
		Prepare: action_kit_api.MutatingEndpointReference{},
		Start:   action_kit_api.MutatingEndpointReference{},
		Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("5s"),
		}),
	}
}

func (f EcsServiceTaskCountCheckAction) Prepare(_ context.Context, state *EcsServiceTaskCountCheckState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	var config EcsServiceTaskCountCheckConfig
	if err := extconversion.Convert(request.Config, &config); err != nil {
		return nil, extensionkit.ToError("Failed to unmarshal the config.", err)
	}

	awsAccount := extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	clusterArn := extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.cluster.arn")[0]
	serviceArn := extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.service.arn")[0]

	f.poller.Register(awsAccount, clusterArn, serviceArn)
	counts, err := f.initialRunningCount(awsAccount, clusterArn, serviceArn)
	if err != nil {
		return nil, err
	}

	state.Timeout = time.Now().Add(time.Millisecond * time.Duration(config.Duration))
	state.RunningCountCheckMode = config.RunningCountCheckMode
	state.AwsAccount = awsAccount
	state.ClusterArn = clusterArn
	state.ServiceArn = serviceArn
	state.InitialRunningCount = counts.running

	return nil, nil
}

func (f EcsServiceTaskCountCheckAction) initialRunningCount(awsAccount string, clusterArn string, serviceArn string) (*escServiceTaskCounts, error) {
	latest := f.poller.AwaitLatest(awsAccount, clusterArn, serviceArn)
	if latest != nil {
		if latest.service != nil {
			return toServiceTaskCounts(latest.service), nil
		} else if latest.failure != nil {
			message := fmt.Sprintf("error accessing service %q in cluster %q: %s", serviceArn, clusterArn, *latest.failure.Reason)
			return nil, extensionkit.ToError(message, nil)
		}
	}
	message := fmt.Sprintf("service %q in cluster %q not found", serviceArn, clusterArn)
	return nil, extensionkit.ToError(message, nil)
}

func (f EcsServiceTaskCountCheckAction) Start(_ context.Context, _ *EcsServiceTaskCountCheckState) (*action_kit_api.StartResult, error) {
	return nil, nil
}

func (f EcsServiceTaskCountCheckAction) Stop(_ context.Context, state *EcsServiceTaskCountCheckState) (*action_kit_api.StopResult, error) {
	f.poller.Unregister(state.AwsAccount, state.ClusterArn, state.ServiceArn)
	return nil, nil
}

func (f EcsServiceTaskCountCheckAction) Status(_ context.Context, state *EcsServiceTaskCountCheckState) (*action_kit_api.StatusResult, error) {
	latest := f.poller.Latest(state.AwsAccount, state.ClusterArn, state.ServiceArn)

	var checkError *action_kit_api.ActionKitError
	if latest != nil {
		if latest.service != nil {
			counts := toServiceTaskCounts(latest.service)
			checkError = f.checkRunningAndDesiredCount(state, counts)
		} else if latest.failure != nil {
			checkError = &action_kit_api.ActionKitError{
				Title: fmt.Sprintf("error accessing service %q in cluster %q: %s", state.ServiceArn, state.ClusterArn, *latest.failure.Reason),
			}
		}
	}

	if time.Now().After(state.Timeout) {
		return &action_kit_api.StatusResult{
			Completed: true,
			Error:     checkError,
		}, nil
	} else {
		return &action_kit_api.StatusResult{
			Completed: checkError == nil,
		}, nil
	}
}

func (f EcsServiceTaskCountCheckAction) checkRunningAndDesiredCount(state *EcsServiceTaskCountCheckState, counts *escServiceTaskCounts) *action_kit_api.ActionKitError {
	var checkMessage string
	switch state.RunningCountCheckMode {
	case runningCountMin1:
		if counts.running < 1 {
			checkMessage = fmt.Sprintf("Service %q in cluster %q has no running task.", state.ServiceArn, state.ClusterArn)
		}
	case runningCountEqualsDesiredCount:
		if counts.running != counts.desired {
			checkMessage = fmt.Sprintf("Service %q in cluster %q has only %d of desired %d tasks running.", state.ServiceArn, state.ClusterArn, counts.running, counts.desired)
		}
	case runningCountLessThanDesiredCount:
		if counts.running >= counts.desired {
			checkMessage = fmt.Sprintf("Service %q in cluster %q has all %d desired tasks running.", state.ServiceArn, state.ClusterArn, counts.desired)
		}
	case runningCountIncreased:
		if counts.running <= state.InitialRunningCount {
			checkMessage = fmt.Sprintf("service %q in cluster %q running task count didn't increase. Initial count: %d, current count: %d.", state.ServiceArn, state.ClusterArn, state.InitialRunningCount, counts.running)
		}
	case runningCountDecreased:
		if counts.running >= state.InitialRunningCount {
			checkMessage = fmt.Sprintf("service %q in cluster %q running task count didn't decrease. Initial count: %d, current count: %d.", state.ServiceArn, state.ClusterArn, state.InitialRunningCount, counts.running)
		}
	default:
		checkMessage = fmt.Sprintf("unsupported check type %q", state.RunningCountCheckMode)
	}
	if checkMessage != "" {
		return extutil.Ptr(action_kit_api.ActionKitError{
			Title:  checkMessage,
			Status: extutil.Ptr(action_kit_api.Failed),
		})
	}
	return nil
}

func toServiceTaskCounts(service *types.Service) *escServiceTaskCounts {
	return &escServiceTaskCounts{
		running: extutil.ToInt(service.RunningCount),
		desired: extutil.ToInt(service.DesiredCount),
	}
}
