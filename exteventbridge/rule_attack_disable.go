// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteventbridge

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type ruleDisableAttack struct {
	clientProvider func(account string, region string, role *string) (EventBridgeApi, error)
}

var (
	_ action_kit_sdk.Action[EventBridgeRuleAttackState]         = (*ruleDisableAttack)(nil)
	_ action_kit_sdk.ActionWithStop[EventBridgeRuleAttackState] = (*ruleDisableAttack)(nil)
)

func NewRuleDisableAttack() action_kit_sdk.ActionWithStop[EventBridgeRuleAttackState] {
	return &ruleDisableAttack{clientProvider: defaultEventBridgeClientProvider}
}

func (a *ruleDisableAttack) NewEmptyState() EventBridgeRuleAttackState {
	return EventBridgeRuleAttackState{}
}

func (a *ruleDisableAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.disable", ruleTargetType),
		Label:       "Disable EventBridge Rule",
		Description: "Disables an EventBridge rule for the duration of the experiment. The rule is re-enabled on stop. Lets you simulate dropped events without deleting any configuration.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(eventBridgeIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: ruleTargetType,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by rule name and event bus",
					Description: new("Find rule by event bus name and rule name"),
					Query:       "aws.eventbridge.rule.bus.name=\"\" and aws.eventbridge.rule.name=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("EventBridge"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("Duration the rule will be disabled. The rule is re-enabled on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("60s"),
				Order:        new(1),
				Required:     new(true),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *ruleDisableAttack) Prepare(_ context.Context, state *EventBridgeRuleAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.BusName = extutil.MustHaveValue(request.Target.Attributes, "aws.eventbridge.rule.bus.name")[0]
	state.RuleName = extutil.MustHaveValue(request.Target.Attributes, "aws.eventbridge.rule.name")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	return nil, nil
}

func (a *ruleDisableAttack) Start(ctx context.Context, state *EventBridgeRuleAttackState) (*action_kit_api.StartResult, error) {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EventBridge client for AWS account %s", state.Account), err)
	}
	_, err = client.DisableRule(ctx, &eventbridge.DisableRuleInput{
		EventBusName: aws.String(state.BusName),
		Name:         aws.String(state.RuleName),
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to disable EventBridge rule %s/%s", state.BusName, state.RuleName), err)
	}
	return &action_kit_api.StartResult{
		Messages: new([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Disabled EventBridge rule %s/%s", state.BusName, state.RuleName),
		}}),
	}, nil
}

func (a *ruleDisableAttack) Stop(ctx context.Context, state *EventBridgeRuleAttackState) (*action_kit_api.StopResult, error) {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EventBridge client for AWS account %s", state.Account), err)
	}
	_, err = client.EnableRule(ctx, &eventbridge.EnableRuleInput{
		EventBusName: aws.String(state.BusName),
		Name:         aws.String(state.RuleName),
	})
	if err != nil {
		log.Error().Err(err).Msgf("Failed to re-enable EventBridge rule %s/%s", state.BusName, state.RuleName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to re-enable EventBridge rule %s/%s", state.BusName, state.RuleName), err)
	}
	return &action_kit_api.StopResult{
		Messages: new([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Re-enabled EventBridge rule %s/%s", state.BusName, state.RuleName),
		}}),
	}, nil
}
