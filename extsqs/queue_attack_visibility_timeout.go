// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extsqs

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type queueVisibilityTimeoutAttack struct {
	clientProvider func(account string, region string, role *string) (SqsApi, error)
}

var (
	_ action_kit_sdk.Action[QueueVisibilityTimeoutAttackState]         = (*queueVisibilityTimeoutAttack)(nil)
	_ action_kit_sdk.ActionWithStop[QueueVisibilityTimeoutAttackState] = (*queueVisibilityTimeoutAttack)(nil)
)

func NewQueueVisibilityTimeoutAttack() action_kit_sdk.ActionWithStop[QueueVisibilityTimeoutAttackState] {
	return &queueVisibilityTimeoutAttack{clientProvider: defaultSqsClientProvider}
}

func (a *queueVisibilityTimeoutAttack) NewEmptyState() QueueVisibilityTimeoutAttackState {
	return QueueVisibilityTimeoutAttackState{}
}

func (a *queueVisibilityTimeoutAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    fmt.Sprintf("%s.visibility-timeout", queueTargetType),
		Label: "Change Queue Visibility Timeout",
		Description: "Temporarily changes the SQS queue's visibility timeout. Set it very low (0-5s) to force premature redelivery and stress consumer idempotency, " +
			"or very high (close to the 12-hour max) to stall redelivery and reveal stuck-message handling. Original timeout is restored on stop.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    new(sqsIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: queueTargetType,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by queue name",
					Description: new("Find SQS queue by name"),
					Query:       "aws.sqs.queue.name=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("SQS"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("How long the modified visibility timeout stays in effect. Original value is restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("60s"),
				Order:        new(1),
				Required:     new(true),
			},
			{
				Name:         "visibilityTimeoutSeconds",
				Label:        "New visibility timeout (seconds)",
				Description:  new("New visibility timeout in seconds. Valid range: 0-43200 (12 hours). Use 0-5 to force premature redelivery; use a high value to stall redelivery."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("0"),
				Order:        new(2),
				Required:     new(true),
				MinValue:     new(0),
				MaxValue:     new(43200),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *queueVisibilityTimeoutAttack) Prepare(ctx context.Context, state *QueueVisibilityTimeoutAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.QueueUrl = extutil.MustHaveValue(request.Target.Attributes, "aws.sqs.queue.url")[0]
	state.QueueName = extutil.MustHaveValue(request.Target.Attributes, "aws.sqs.queue.name")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")

	target := extutil.ToInt(request.Config["visibilityTimeoutSeconds"])
	if target < 0 || target > 43200 {
		return nil, extension_kit.ToError("visibilityTimeoutSeconds must be between 0 and 43200 (12 hours).", nil)
	}
	state.TargetVisibilityTimeout = int32(target)

	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize SQS client for AWS account %s", state.Account), err)
	}
	out, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       &state.QueueUrl,
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameVisibilityTimeout},
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe SQS queue %s", state.QueueName), err)
	}
	if v, ok := out.Attributes[string(sqstypes.QueueAttributeNameVisibilityTimeout)]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to parse original VisibilityTimeout %q for queue %s", v, state.QueueName), err)
		}
		state.OriginalVisibilityTimeout = int32(n)
	} else {
		// SQS default when unset is 30s.
		state.OriginalVisibilityTimeout = 30
	}
	return &action_kit_api.PrepareResult{
		Messages: new([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Will change VisibilityTimeout on queue %s from %d to %d for the attack duration", state.QueueName, state.OriginalVisibilityTimeout, state.TargetVisibilityTimeout),
		}}),
	}, nil
}

func (a *queueVisibilityTimeoutAttack) Start(ctx context.Context, state *QueueVisibilityTimeoutAttackState) (*action_kit_api.StartResult, error) {
	if err := a.setVisibilityTimeout(ctx, state, state.TargetVisibilityTimeout); err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to change VisibilityTimeout on SQS queue %s", state.QueueName), err)
	}
	return &action_kit_api.StartResult{
		Messages: new([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Set VisibilityTimeout on queue %s to %d (was %d)", state.QueueName, state.TargetVisibilityTimeout, state.OriginalVisibilityTimeout),
		}}),
	}, nil
}

func (a *queueVisibilityTimeoutAttack) Stop(ctx context.Context, state *QueueVisibilityTimeoutAttackState) (*action_kit_api.StopResult, error) {
	if err := a.setVisibilityTimeout(ctx, state, state.OriginalVisibilityTimeout); err != nil {
		log.Error().Err(err).Msgf("Failed to restore VisibilityTimeout on SQS queue %s", state.QueueName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore VisibilityTimeout on SQS queue %s", state.QueueName), err)
	}
	return &action_kit_api.StopResult{
		Messages: new([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Restored VisibilityTimeout on queue %s to %d", state.QueueName, state.OriginalVisibilityTimeout),
		}}),
	}, nil
}

func (a *queueVisibilityTimeoutAttack) setVisibilityTimeout(ctx context.Context, state *QueueVisibilityTimeoutAttackState, value int32) error {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return err
	}
	_, err = client.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl: &state.QueueUrl,
		Attributes: map[string]string{
			string(sqstypes.QueueAttributeNameVisibilityTimeout): strconv.Itoa(int(value)),
		},
	})
	return err
}

func defaultSqsClientProvider(account string, region string, role *string) (SqsApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return sqs.NewFromConfig(awsAccess.AwsConfig), nil
}
