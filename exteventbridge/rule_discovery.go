// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteventbridge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type ruleDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*ruleDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*ruleDiscovery)(nil)
)

func NewRuleDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&ruleDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalEventbridge)*time.Second),
	)
}

func (d *ruleDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: ruleTargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEventbridge)),
		},
	}
}

func (d *ruleDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       ruleTargetType,
		Label:    discovery_kit_api.PluralLabel{One: "EventBridge rule", Other: "EventBridge rules"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(eventBridgeIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.eventbridge.rule.bus.name"},
				{Attribute: "aws.eventbridge.rule.state"},
				{Attribute: "aws.eventbridge.rule.dlq.configured"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *ruleDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.eventbridge.rule.name", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule name", Other: "EventBridge rule names"}},
		{Attribute: "aws.eventbridge.rule.bus.name", Label: discovery_kit_api.PluralLabel{One: "EventBridge bus name", Other: "EventBridge bus names"}},
		{Attribute: "aws.eventbridge.rule.state", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule state", Other: "EventBridge rule states"}},
		{Attribute: "aws.eventbridge.rule.schedule-expression", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule schedule expression", Other: "EventBridge rule schedule expressions"}},
		{Attribute: "aws.eventbridge.rule.event-pattern.present", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule event-pattern present", Other: "EventBridge rule event-pattern present"}},
		{Attribute: "aws.eventbridge.rule.target-count", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule target count", Other: "EventBridge rule target counts"}},
		{Attribute: "aws.eventbridge.rule.target.arns", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule target ARN", Other: "EventBridge rule target ARNs"}},
		{Attribute: "aws.eventbridge.rule.dlq.configured", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule DLQ configured", Other: "EventBridge rule DLQ configured"}},
		{Attribute: "aws.eventbridge.rule.role-arn", Label: discovery_kit_api.PluralLabel{One: "EventBridge rule role ARN", Other: "EventBridge rule role ARNs"}},
	}
}

func (d *ruleDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getRuleTargets, ctx, "eventbridge-rule")
}

func getRuleTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := eventbridge.NewFromConfig(account.AwsConfig)
	result, err := getAllRules(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover EventBridge rules for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EVENTBRIDGE=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllRules(ctx context.Context, client EventBridgeApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	busNames, err := listAllBusNames(ctx, client)
	if err != nil {
		return nil, err
	}
	result := make([]discovery_kit_api.Target, 0, 20)
	for _, bus := range busNames {
		rules, err := listAllRulesOnBus(ctx, client, bus)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to list EventBridge rules on bus %s", bus)
			continue
		}
		for _, rule := range rules {
			if rule.Name == nil {
				continue
			}
			targets, err := listAllTargetsByRule(ctx, client, bus, *rule.Name)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to list EventBridge rule targets for %s/%s", bus, *rule.Name)
			}
			tags := map[string]string{}
			if rule.Arn != nil {
				tagsOut, err := client.ListTagsForResource(ctx, &eventbridge.ListTagsForResourceInput{ResourceARN: rule.Arn})
				if err != nil {
					log.Debug().Err(err).Msgf("Failed to list tags for EventBridge rule %s", *rule.Arn)
				} else {
					for _, t := range tagsOut.Tags {
						if t.Key != nil {
							tags[*t.Key] = aws.ToString(t.Value)
						}
					}
				}
			}
			if !matchesEventbridgeTagFilter(tags, account.TagFilters) {
				continue
			}
			result = append(result, toRuleTarget(rule, bus, targets, tags, account.AccountNumber, account.Region, account.AssumeRole))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesEventbridge), nil
}

func listAllBusNames(ctx context.Context, client EventBridgeApi) ([]string, error) {
	names := make([]string, 0)
	var nextToken *string
	for {
		out, err := client.ListEventBuses(ctx, &eventbridge.ListEventBusesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, b := range out.EventBuses {
			if b.Name != nil {
				names = append(names, *b.Name)
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return names, nil
}

func listAllRulesOnBus(ctx context.Context, client EventBridgeApi, busName string) ([]types.Rule, error) {
	rules := make([]types.Rule, 0)
	var nextToken *string
	for {
		out, err := client.ListRules(ctx, &eventbridge.ListRulesInput{EventBusName: aws.String(busName), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		rules = append(rules, out.Rules...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return rules, nil
}

func listAllTargetsByRule(ctx context.Context, client EventBridgeApi, busName string, ruleName string) ([]types.Target, error) {
	targets := make([]types.Target, 0)
	var nextToken *string
	for {
		out, err := client.ListTargetsByRule(ctx, &eventbridge.ListTargetsByRuleInput{
			EventBusName: aws.String(busName),
			Rule:         aws.String(ruleName),
			NextToken:    nextToken,
		})
		if err != nil {
			return nil, err
		}
		targets = append(targets, out.Targets...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return targets, nil
}

func matchesEventbridgeTagFilter(tags map[string]string, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		if v, ok := tags[filter.Key]; ok {
			if slices.Contains(filter.Values, v) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func toRuleTarget(rule types.Rule, busName string, targets []types.Target, tags map[string]string, account string, region string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(rule.Arn)
	name := aws.ToString(rule.Name)
	label := fmt.Sprintf("%s/%s", busName, name)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.eventbridge.rule.name"] = []string{name}
	attributes["aws.eventbridge.rule.bus.name"] = []string{busName}
	if rule.State != "" {
		attributes["aws.eventbridge.rule.state"] = []string{string(rule.State)}
	}
	if rule.ScheduleExpression != nil && *rule.ScheduleExpression != "" {
		attributes["aws.eventbridge.rule.schedule-expression"] = []string{*rule.ScheduleExpression}
	}
	attributes["aws.eventbridge.rule.event-pattern.present"] = []string{strconv.FormatBool(rule.EventPattern != nil && *rule.EventPattern != "")}
	if rule.RoleArn != nil && *rule.RoleArn != "" {
		attributes["aws.eventbridge.rule.role-arn"] = []string{*rule.RoleArn}
	}

	attributes["aws.eventbridge.rule.target-count"] = []string{strconv.Itoa(len(targets))}
	dlqConfigured := false
	if len(targets) > 0 {
		arns := make([]string, 0, len(targets))
		for _, t := range targets {
			if t.Arn != nil {
				arns = append(arns, *t.Arn)
			}
			if t.DeadLetterConfig != nil && t.DeadLetterConfig.Arn != nil && *t.DeadLetterConfig.Arn != "" {
				dlqConfigured = true
			}
		}
		if len(arns) > 0 {
			attributes["aws.eventbridge.rule.target.arns"] = arns
		}
	}
	attributes["aws.eventbridge.rule.dlq.configured"] = []string{strconv.FormatBool(dlqConfigured)}

	for k, v := range tags {
		attributes[fmt.Sprintf("aws.eventbridge.rule.label.%s", strings.ToLower(k))] = []string{v}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: ruleTargetType,
		Attributes: attributes,
	}
}
