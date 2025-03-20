// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extmsk

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type mskRebootBrokerAttack struct {
	clientProvider func(account string, region string, role *string) (MskApi, error)
}

var _ action_kit_sdk.Action[KafkaAttackState] = (*mskRebootBrokerAttack)(nil)

func NewMskRebootBrokerAttack() action_kit_sdk.Action[KafkaAttackState] {
	return mskRebootBrokerAttack{defaultMskClientProvider}
}

func (f mskRebootBrokerAttack) NewEmptyState() KafkaAttackState {
	return KafkaAttackState{}
}

func (f mskRebootBrokerAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.reboot-broker", mskBrokerTargetId),
		Label:       "Trigger Broker Reboot",
		Description: "Triggers broker reboot",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(mskIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: mskBrokerTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "msk cluster id and broker id",
					Description: extutil.Ptr("Find broker by id and cluster id"),
					Query:       "aws.msk.cluster.id=\"\" and aws.msk.cluster.broker.id=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("MSK"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (f mskRebootBrokerAttack) Prepare(_ context.Context, state *KafkaAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	state.ClusterARN = extutil.MustHaveValue(request.Target.Attributes, "aws.msk.cluster.arn")[0]
	state.ClusterName = extutil.MustHaveValue(request.Target.Attributes, "aws.msk.cluster.name")[0]
	state.BrokerID = extutil.MustHaveValue(request.Target.Attributes, "aws.msk.cluster.broker.id")[0]
	return nil, nil
}

func (f mskRebootBrokerAttack) Start(ctx context.Context, state *KafkaAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Msk client for AWS account %s", state.Account), err)
	}

	input := kafka.RebootBrokerInput{
		ClusterArn: &state.ClusterARN,
		BrokerIds:  []string{state.BrokerID},
	}
	_, err = client.RebootBroker(ctx, &input)
	if err != nil {
		return nil, extension_kit.ToError("Failed to trigger kafka broker reboot", err)
	}
	return &action_kit_api.StartResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("MSK kafka cluster %s broker %s reboot triggered", state.ClusterName, state.BrokerID),
		}},
	}, nil

}
