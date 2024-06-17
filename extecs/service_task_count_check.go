// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
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

type ServiceTaskCountCheckState struct {
	Timeout               time.Time
	RunningCountCheckMode string
	ServiceArn            string
	ClusterArn            string
	AwsAccount            string
	InitialRunningCount   int
}

type ServiceTaskCountCheckConfig struct {
	Duration              int
	RunningCountCheckMode string
}

type escServiceTaskCounts struct {
	running int
	desired int
}

type ecsServiceTaskCountCheckApi interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

type ServiceTaskCountCheckAction struct {
	getApiClient func(account string) (ecsServiceTaskCountCheckApi, error)
}

var _ action_kit_sdk.Action[ServiceTaskCountCheckState] = (*ServiceTaskCountCheckAction)(nil)
var _ action_kit_sdk.ActionWithStatus[ServiceTaskCountCheckState] = (*ServiceTaskCountCheckAction)(nil)

func NewServiceTaskCountCheckAction() action_kit_sdk.Action[ServiceTaskCountCheckState] {
	return ServiceTaskCountCheckAction{
		getApiClient: defaultServiceTaskCountClientProvider,
	}
}

func (f ServiceTaskCountCheckAction) NewEmptyState() ServiceTaskCountCheckState {
	return ServiceTaskCountCheckState{}
}

func (f ServiceTaskCountCheckAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          ecsServiceTaskCountCheckActionId,
		Label:       "Service Task Count",
		Description: "Verify service task counts.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(ecsServiceIcon),
		Category:    extutil.Ptr("cloud"),
		Kind:        action_kit_api.Check,
		TimeControl: action_kit_api.TimeControlInternal,
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType:          ecsServiceTargetId,
			QuantityRestriction: extutil.Ptr(action_kit_api.All),
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "default",
					Description: extutil.Ptr("Find service by cluster and name"),
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

func (f ServiceTaskCountCheckAction) Prepare(ctx context.Context, state *ServiceTaskCountCheckState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	var config ServiceTaskCountCheckConfig
	if err := extconversion.Convert(request.Config, &config); err != nil {
		return nil, extensionkit.ToError("Failed to unmarshal the config.", err)
	}

	awsAccount := request.Target.Attributes["aws.account"][0]
	clusterArn := request.Target.Attributes["aws-ecs.cluster.arn"][0]
	serviceArn := request.Target.Attributes["aws-ecs.service.arn"][0]

	client, err := f.getApiClient(awsAccount)
	if err != nil {
		return nil, err
	}

	counts, err := f.getRunningAndDesiredTaskCount(serviceArn, clusterArn, client, ctx)
	if err != nil {
		return nil, err
	}

	state.Timeout = time.Now().Add(time.Millisecond * time.Duration(config.Duration))
	state.RunningCountCheckMode = config.RunningCountCheckMode
	state.ServiceArn = serviceArn
	state.ClusterArn = clusterArn
	state.AwsAccount = awsAccount
	state.InitialRunningCount = counts.running

	return nil, nil
}

func (f ServiceTaskCountCheckAction) Start(_ context.Context, _ *ServiceTaskCountCheckState) (*action_kit_api.StartResult, error) {
	return nil, nil
}

func (f ServiceTaskCountCheckAction) Status(ctx context.Context, state *ServiceTaskCountCheckState) (*action_kit_api.StatusResult, error) {
	now := time.Now()
	client, err := f.getApiClient(state.AwsAccount)
	if err != nil {
		return nil, err
	}

	counts, err := f.getRunningAndDesiredTaskCount(state.ServiceArn, state.ClusterArn, client, ctx)
	if err != nil {
		return nil, err
	}
	failedCheck := f.checkRunningAndDesiredCount(state, counts)

	timeIsUp := now.After(state.Timeout)
	return &action_kit_api.StatusResult{
		Completed: timeIsUp || failedCheck != nil,
		Error:     failedCheck,
	}, nil
}

func (f ServiceTaskCountCheckAction) getRunningAndDesiredTaskCount(serviceArn string, clusterArn string, ecsServiceApi ecsServiceTaskCountCheckApi, ctx context.Context) (*escServiceTaskCounts, error) {

	services, err := ecsServiceApi.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Services: []string{serviceArn},
		Cluster:  extutil.Ptr(clusterArn),
	})
	if err != nil {
		return nil, err
	}
	for _, service := range services.Services {
		return &escServiceTaskCounts{
			running: extutil.ToInt(service.RunningCount),
			desired: extutil.ToInt(service.DesiredCount),
		}, nil
	}
	return nil, extensionkit.ToError(fmt.Sprintf("service %q in cluster %q not found", serviceArn, clusterArn), nil)
}

func (f ServiceTaskCountCheckAction) checkRunningAndDesiredCount(state *ServiceTaskCountCheckState, counts *escServiceTaskCounts) *action_kit_api.ActionKitError {
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

func defaultServiceTaskCountClientProvider(account string) (ecsServiceTaskCountCheckApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return ecs.NewFromConfig(awsAccount.AwsConfig), nil
}
