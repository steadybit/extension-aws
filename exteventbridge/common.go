// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteventbridge

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/steadybit/extension-aws/v2/utils"
)

const (
	eventBridgeIcon = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmw0IDQtNCA0LTQtNCA0LTR6bS00IDhsNCA0IDQtNCA0IDQtNCA0LTQtNC00IDQtNC00IDQtNHptOCA4bDQgNC00IDR2LTh6IiBmaWxsPSJjdXJyZW50Q29sb3IiLz48L3N2Zz4="
	ruleTargetType  = "com.steadybit.extension_aws.eventbridge.rule"
)

type EventBridgeRuleAttackState struct {
	BusName          string
	RuleName         string
	Account          string
	Region           string
	DiscoveredByRole *string
}

type EventBridgeApi interface {
	ListEventBuses(ctx context.Context, params *eventbridge.ListEventBusesInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error)
	ListRules(ctx context.Context, params *eventbridge.ListRulesInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error)
	ListTargetsByRule(ctx context.Context, params *eventbridge.ListTargetsByRuleInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error)
	ListTagsForResource(ctx context.Context, params *eventbridge.ListTagsForResourceInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListTagsForResourceOutput, error)
	EnableRule(ctx context.Context, params *eventbridge.EnableRuleInput, optFns ...func(*eventbridge.Options)) (*eventbridge.EnableRuleOutput, error)
	DisableRule(ctx context.Context, params *eventbridge.DisableRuleInput, optFns ...func(*eventbridge.Options)) (*eventbridge.DisableRuleOutput, error)
}

func defaultEventBridgeClientProvider(account string, region string, role *string) (EventBridgeApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return eventbridge.NewFromConfig(awsAccess.AwsConfig), nil
}
