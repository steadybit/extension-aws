// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extmq

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type brokerRebootAttack struct {
	clientProvider func(account string, region string, role *string) (MqApi, error)
}

var _ action_kit_sdk.Action[BrokerAttackState] = (*brokerRebootAttack)(nil)

func NewBrokerRebootAttack() action_kit_sdk.Action[BrokerAttackState] {
	return brokerRebootAttack{defaultMqClientProvider}
}

func (a brokerRebootAttack) NewEmptyState() BrokerAttackState {
	return BrokerAttackState{}
}

func (a brokerRebootAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.reboot", brokerTargetId),
		Label:       "Trigger MQ Broker Reboot",
		Description: "Reboots an Amazon MQ broker. For SINGLE_INSTANCE brokers this causes downtime; for ACTIVE_STANDBY_MULTI_AZ or CLUSTER_MULTI_AZ deployments this validates failover.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(mqIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: brokerTargetId,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by broker id",
					Description: new("Find Amazon MQ broker by id"),
					Query:       "aws.mq.broker.id=\"\"",
				},
				{
					Label:       "by broker name",
					Description: new("Find Amazon MQ broker by name"),
					Query:       "aws.mq.broker.name=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("MQ"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (a brokerRebootAttack) Prepare(_ context.Context, state *BrokerAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.BrokerID = extutil.MustHaveValue(request.Target.Attributes, "aws.mq.broker.id")[0]
	state.BrokerName = extutil.MustHaveValue(request.Target.Attributes, "aws.mq.broker.name")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	return nil, nil
}

func (a brokerRebootAttack) Start(ctx context.Context, state *BrokerAttackState) (*action_kit_api.StartResult, error) {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Amazon MQ client for AWS account %s", state.Account), err)
	}
	_, err = client.RebootBroker(ctx, &mq.RebootBrokerInput{BrokerId: &state.BrokerID})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to reboot Amazon MQ broker %s (%s)", state.BrokerName, state.BrokerID), err)
	}
	return &action_kit_api.StartResult{
		Messages: new([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Reboot triggered for Amazon MQ broker %s (%s)", state.BrokerName, state.BrokerID),
		}}),
	}, nil
}
