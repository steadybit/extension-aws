// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"time"
)

const LogType = "ECS_SERVICE_EVENTS"

type EcsServiceEventLogAction struct {
	poller ServiceDescriptionPoller
}

type EcsServiceEventLogState struct {
	LatestEventTimestamp time.Time
	ServiceArn           string
	ClusterArn           string
	AwsAccount           string
}

func NewEcsServiceEventLogAction(poller ServiceDescriptionPoller) action_kit_sdk.Action[EcsServiceEventLogState] {
	return EcsServiceEventLogAction{
		poller: poller,
	}
}

var _ action_kit_sdk.Action[EcsServiceEventLogState] = (*EcsServiceEventLogAction)(nil)
var _ action_kit_sdk.ActionWithStatus[EcsServiceEventLogState] = (*EcsServiceEventLogAction)(nil)
var _ action_kit_sdk.ActionWithStop[EcsServiceEventLogState] = (*EcsServiceEventLogAction)(nil)

func (f EcsServiceEventLogAction) NewEmptyState() EcsServiceEventLogState {
	return EcsServiceEventLogState{}
}

func (f EcsServiceEventLogAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          ecsServiceEventLogActionId,
		Label:       "Service Event Log",
		Description: "Collect service events from ECS",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(ecsServiceIcon),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("ECS"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Other,
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
				Label:        "Duration",
				Description:  extutil.Ptr(""),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Widgets: extutil.Ptr([]action_kit_api.Widget{
			action_kit_api.LogWidget{
				Type:    action_kit_api.ComSteadybitWidgetLog,
				Title:   "Service Events",
				LogType: LogType,
			},
		}),
		Prepare: action_kit_api.MutatingEndpointReference{},
		Start:   action_kit_api.MutatingEndpointReference{},
		Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("5s"),
		}),
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (f EcsServiceEventLogAction) Prepare(_ context.Context, state *EcsServiceEventLogState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	awsAccount := extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	clusterArn := extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.cluster.arn")[0]
	serviceArn := extutil.MustHaveValue(request.Target.Attributes, "aws-ecs.service.arn")[0]

	f.poller.Register(awsAccount, clusterArn, serviceArn)

	state.LatestEventTimestamp = time.Now().In(time.UTC)
	state.AwsAccount = awsAccount
	state.ClusterArn = clusterArn
	state.ServiceArn = serviceArn
	return nil, nil
}

func (f EcsServiceEventLogAction) Start(_ context.Context, state *EcsServiceEventLogState) (*action_kit_api.StartResult, error) {
	return &action_kit_api.StartResult{
		Messages: f.newMessages(state),
	}, nil
}

func (f EcsServiceEventLogAction) Status(_ context.Context, state *EcsServiceEventLogState) (*action_kit_api.StatusResult, error) {
	return &action_kit_api.StatusResult{
		Messages: f.newMessages(state),
	}, nil
}

func (f EcsServiceEventLogAction) Stop(_ context.Context, state *EcsServiceEventLogState) (*action_kit_api.StopResult, error) {
	defer f.poller.Unregister(state.AwsAccount, state.ClusterArn, state.ServiceArn)
	return &action_kit_api.StopResult{
		Messages: f.newMessages(state),
	}, nil
}

func (f EcsServiceEventLogAction) newMessages(state *EcsServiceEventLogState) *action_kit_api.Messages {
	latest := f.poller.Latest(state.AwsAccount, state.ClusterArn, state.ServiceArn)
	newEvents, newLatestEventTimestamp := filterEventsAfter(latest, state.LatestEventTimestamp)
	state.LatestEventTimestamp = newLatestEventTimestamp
	if len(newEvents) > 0 {
		log.Debug().Msgf("found %d new event(s) in service %s", len(newEvents), state.ServiceArn)
	}
	return eventsToMessages(newEvents)
}

func filterEventsAfter(poll *PollService, timestamp time.Time) ([]types.ServiceEvent, time.Time) {
	var newEvents []types.ServiceEvent
	latestTimestamp := timestamp
	if poll != nil && poll.service != nil {
		for _, event := range poll.service.Events {
			if event.CreatedAt != nil && event.CreatedAt.After(timestamp) {
				newEvents = append(newEvents, event)
				if latestTimestamp.Before(*event.CreatedAt) {
					latestTimestamp = *event.CreatedAt
				}
			}
		}
	}
	return newEvents, latestTimestamp
}

func eventsToMessages(events []types.ServiceEvent) *action_kit_api.Messages {
	var messages action_kit_api.Messages
	for _, event := range events {
		messages = append(messages, action_kit_api.Message{
			Message:         aws.ToString(event.Message),
			Type:            extutil.Ptr(LogType),
			Timestamp:       event.CreatedAt,
			TimestampSource: extutil.Ptr(action_kit_api.TimestampSourceExternal),
			Fields: extutil.Ptr(action_kit_api.MessageFields{
				"Id": aws.ToString(event.Id),
			}),
		})
	}
	return extutil.Ptr(messages)
}
