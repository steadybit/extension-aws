// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extelb

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strconv"
)

type albStaticResponseAction struct {
	clientProvider func(account string, region string, role *string) (albStaticResponseApi, error)
}

// Make sure action implements all required interfaces
var _ action_kit_sdk.Action[AlbStaticResponseState] = (*albStaticResponseAction)(nil)
var _ action_kit_sdk.ActionWithStop[AlbStaticResponseState] = (*albStaticResponseAction)(nil)

type AlbStaticResponseState struct {
	Account              string
	Region               string
	DiscoveredByRole     *string
	LoadbalancerArn      string
	ListenerArn          string
	ResponseStatusCode   int
	ResponseBody         string
	ResponseContentType  string
	ConditionHostHeader  []string
	ConditionPathPattern []string
	ConditionHttpMethod  []string
	ConditionSourceIp    []string
	ConditionQueryString map[string]string
	ConditionHttpHeader  map[string]string
	TargetExecutionId    uuid.UUID
	ExecutionId          int
	ExperimentKey        string
}

type albStaticResponseApi interface {
	elasticloadbalancingv2.DescribeListenersAPIClient
	elasticloadbalancingv2.DescribeRulesAPIClient
	SetRulePriorities(ctx context.Context, params *elasticloadbalancingv2.SetRulePrioritiesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.SetRulePrioritiesOutput, error)
	CreateRule(ctx context.Context, params *elasticloadbalancingv2.CreateRuleInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateRuleOutput, error)
	DeleteRule(ctx context.Context, params *elasticloadbalancingv2.DeleteRuleInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteRuleOutput, error)
	DescribeTags(ctx context.Context, params *elasticloadbalancingv2.DescribeTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTagsOutput, error)
	AddTags(ctx context.Context, params *elasticloadbalancingv2.AddTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.AddTagsOutput, error)
	RemoveTags(ctx context.Context, params *elasticloadbalancingv2.RemoveTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.RemoveTagsOutput, error)
}

func NewAlbStaticResponseAction() action_kit_sdk.Action[AlbStaticResponseState] {
	return &albStaticResponseAction{defaultClientProviderService}
}

func (e *albStaticResponseAction) NewEmptyState() AlbStaticResponseState {
	return AlbStaticResponseState{}
}

func (e *albStaticResponseAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.static_response", albTargetId),
		Label:       "Return static response",
		Description: "Define a static Response for a given Listener of an Application Load Balancer.",
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("Load Balancer"),
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(albIcon),
		Kind:        action_kit_api.Attack,
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: albTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "name",
					Description: extutil.Ptr("Find load balancer by name"),
					Query:       "aws-elb.alb.name=\"\"",
				},
			}),
		}),
		TimeControl: action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("The duration of the action."),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("180s"),
				Required:     extutil.Ptr(true),
			},
			{
				Name:        "listenerPort",
				Label:       "Listener Port",
				Description: extutil.Ptr("The port of the listener."),
				Type:        action_kit_api.String,
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ParameterOptionsFromTargetAttribute{
						Attribute: "aws-elb.alb.listener.port",
					},
				}),
				Required: extutil.Ptr(true),
			},
			{
				Name:  "-response-separator-",
				Label: "-",
				Type:  action_kit_api.Separator,
			},
			{
				Name:  "-response-header-",
				Type:  action_kit_api.Header,
				Label: "Response",
			},
			{
				Name:         "responseStatusCode",
				Label:        "Status Code",
				Description:  extutil.Ptr("The status code which should get returned."),
				Type:         action_kit_api.Integer,
				MinValue:     extutil.Ptr(100),
				MaxValue:     extutil.Ptr(999),
				Required:     extutil.Ptr(true),
				DefaultValue: extutil.Ptr("503"),
			},
			{
				Name:        "responseContentType",
				Label:       "Content Type",
				Description: extutil.Ptr("The content type of the response."),
				Type:        action_kit_api.String,
				Required:    extutil.Ptr(false),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{Label: "text/plain", Value: "text/plain"},
					action_kit_api.ExplicitParameterOption{Label: "text/html", Value: "text/html"},
					action_kit_api.ExplicitParameterOption{Label: "text/css", Value: "text/css"},
					action_kit_api.ExplicitParameterOption{Label: "application/json", Value: "application/json"},
					action_kit_api.ExplicitParameterOption{Label: "application/javascript", Value: "application/javascript"},
				}),
				DefaultValue: extutil.Ptr("text/plain"),
			},
			{
				Name:        "responseBody",
				Label:       "Body",
				Description: extutil.Ptr("The body of the response."),
				Type:        action_kit_api.String,
				Required:    extutil.Ptr(false),
			},
			{
				Name:  "-conditions-separator-",
				Label: "-",
				Type:  action_kit_api.Separator,
			},
			{
				Name:  "-conditions-header-",
				Type:  action_kit_api.Header,
				Label: "Conditions",
			},
			{
				Name:        "conditionHostHeader",
				Label:       "Host Header",
				Description: extutil.Ptr("The host names. The maximum size of each name is 128 characters. The comparison is case insensitive. The following wildcard characters are supported: * (matches 0 or more characters) and ? (matches exactly 1 character). If you specify multiple strings, the condition is satisfied if one of the strings matches the host name. Max 5 values allowed."),
				Type:        action_kit_api.StringArray,
				Required:    extutil.Ptr(false),
			},
			{
				Name:        "conditionPathPattern",
				Label:       "Path Pattern",
				Description: extutil.Ptr("The path patterns to compare against the request URL. The maximum size of each string is 128 characters. The comparison is case sensitive. The following wildcard characters are supported: * (matches 0 or more characters) and ? (matches exactly 1 character). If you specify multiple strings, the condition is satisfied if one of them matches the request URL. The path pattern is compared only to the path of the URL, not to its query string. Max 5 values allowed."),
				Type:        action_kit_api.StringArray,
				Required:    extutil.Ptr(false),
			},
			{
				Name:        "conditionHttpMethod",
				Label:       "Http Method",
				Description: extutil.Ptr("The name of the request method. The maximum size is 40 characters. The allowed characters are A-Z, hyphen (-), and underscore (_). The comparison is case sensitive. Wildcards are not supported; therefore, the method name must be an exact match. If you specify multiple strings, the condition is satisfied if one of the strings matches the HTTP request method. We recommend that you route GET and HEAD requests in the same way, because the response to a HEAD request may be cached. Max 5 values allowed."),
				Type:        action_kit_api.StringArray,
				Required:    extutil.Ptr(false),
			},
			{
				Name:        "conditionSourceIp",
				Label:       "Source IP",
				Description: extutil.Ptr("The source IP addresses, in CIDR format. You can use both IPv4 and IPv6 addresses. Wildcards are not supported. If you specify multiple addresses, the condition is satisfied if the source IP address of the request matches one of the CIDR blocks. This condition is not satisfied by the addresses in the X-Forwarded-For header. Max 5 values allowed."),
				Type:        action_kit_api.StringArray,
				Required:    extutil.Ptr(false),
			},
			{
				Name:        "conditionQueryString",
				Label:       "Query String",
				Description: extutil.Ptr("The key/value pairs or values to find in the query string. The maximum size of each string is 128 characters. The comparison is case insensitive. The following wildcard characters are supported: * (matches 0 or more characters) and ? (matches exactly 1 character). To search for a literal '*' or '?' character in a query string, you must escape these characters in Values using a '\\' character. If you specify multiple key/value pairs or values, the condition is satisfied if one of them is found in the query string. Max 5 values allowed."),
				Type:        action_kit_api.KeyValue,
				Required:    extutil.Ptr(false),
			},
			{
				Name:        "conditionHttpHeader",
				Label:       "HTTP Header",
				Description: extutil.Ptr("The name of the HTTP header field with a maximum size of 40 characters. And a value to compare against the value of the HTTP header. The maximum size of each string is 128 characters. The comparison strings are case insensitive. The following wildcard characters are supported: * (matches 0 or more characters) and ? (matches exactly 1 character). Currently only a single header name with a single value is allowed."),
				Type:        action_kit_api.KeyValue,
				Required:    extutil.Ptr(false),
			},
		},
	}
}

func (e *albStaticResponseAction) Prepare(ctx context.Context, state *AlbStaticResponseState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	state.LoadbalancerArn = extutil.MustHaveValue(request.Target.Attributes, "aws-elb.alb.arn")[0]
	state.TargetExecutionId = request.ExecutionId
	if request.ExecutionContext != nil {
		state.ExecutionId = *request.ExecutionContext.ExecutionId
		state.ExperimentKey = *request.ExecutionContext.ExperimentKey
	}

	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize elb client for AWS account %s", state.Account), err)
	}

	listenerPort := extutil.ToInt32(request.Config["listenerPort"])
	describeListenersResult, err := client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: &state.LoadbalancerArn,
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to fetch listeners for alb '%s'", state.LoadbalancerArn), err)
	}
	for _, listener := range describeListenersResult.Listeners {
		if *listener.Port == listenerPort {
			state.ListenerArn = *listener.ListenerArn
			break
		}
	}
	if state.ListenerArn == "" {
		return nil, extension_kit.ToError(fmt.Sprintf("Listener with port %d not found for alb '%s'", listenerPort, state.LoadbalancerArn), nil)
	}

	state.ResponseStatusCode = extutil.ToInt(request.Config["responseStatusCode"])
	state.ResponseBody = extutil.ToString(request.Config["responseBody"])
	state.ResponseContentType = extutil.ToString(request.Config["responseContentType"])

	state.ConditionHostHeader = extutil.ToStringArray(request.Config["conditionHostHeader"])
	state.ConditionPathPattern = extutil.ToStringArray(request.Config["conditionPathPattern"])
	state.ConditionHttpMethod = extutil.ToStringArray(request.Config["conditionHttpMethod"])
	state.ConditionSourceIp = extutil.ToStringArray(request.Config["conditionSourceIp"])
	if (request.Config["conditionQueryString"]) != nil {
		state.ConditionQueryString, err = extutil.ToKeyValue(request.Config, "conditionQueryString")
		if err != nil {
			return nil, err
		}
	}
	if (request.Config["conditionHttpHeader"]) != nil {
		state.ConditionHttpHeader, err = extutil.ToKeyValue(request.Config, "conditionHttpHeader")
		if err != nil {
			return nil, err
		}
	}

	if len(state.ConditionHostHeader) > 5 {
		return nil, extension_kit.ToError("Max 5 values allowed for conditionHostHeader", nil)
	}
	if len(state.ConditionPathPattern) > 5 {
		return nil, extension_kit.ToError("Max 5 values allowed for conditionPathPattern", nil)
	}
	if len(state.ConditionHttpMethod) > 5 {
		return nil, extension_kit.ToError("Max 5 values allowed for conditionHttpMethod", nil)
	}
	if len(state.ConditionSourceIp) > 5 {
		return nil, extension_kit.ToError("Max 5 values allowed for conditionSourceIp", nil)
	}
	if len(state.ConditionQueryString) > 5 {
		return nil, extension_kit.ToError("Max 5 values allowed for conditionQueryString", nil)
	}
	if len(state.ConditionHttpHeader) > 1 {
		return nil, extension_kit.ToError("Only a single header name with a single value is supported", nil)
	}
	return nil, nil
}

func (e *albStaticResponseAction) Start(ctx context.Context, state *AlbStaticResponseState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize elb client for AWS account %s", state.Account), err)
	}

	err = reprioritizeIfNecessary(ctx, &client, state)
	if err != nil {
		return nil, err
	}

	conditions := make([]types.RuleCondition, 0)
	if len(state.ConditionHostHeader) > 0 {
		conditions = append(conditions, types.RuleCondition{
			Field: extutil.Ptr("host-header"),
			HostHeaderConfig: &types.HostHeaderConditionConfig{
				Values: state.ConditionHostHeader,
			},
		})
	}
	if len(state.ConditionPathPattern) > 0 {
		conditions = append(conditions, types.RuleCondition{
			Field: extutil.Ptr("path-pattern"),
			PathPatternConfig: &types.PathPatternConditionConfig{
				Values: state.ConditionPathPattern,
			},
		})
	}
	if len(state.ConditionHttpMethod) > 0 {
		conditions = append(conditions, types.RuleCondition{
			Field: extutil.Ptr("http-request-method"),
			HttpRequestMethodConfig: &types.HttpRequestMethodConditionConfig{
				Values: state.ConditionHttpMethod,
			},
		})
	}
	if len(state.ConditionSourceIp) > 0 {
		conditions = append(conditions, types.RuleCondition{
			Field: extutil.Ptr("source-ip"),
			SourceIpConfig: &types.SourceIpConditionConfig{
				Values: state.ConditionSourceIp,
			},
		})
	}
	if len(state.ConditionHttpHeader) > 0 {
		for key, value := range state.ConditionHttpHeader {
			conditions = append(conditions, types.RuleCondition{
				Field: extutil.Ptr("http-header"),
				HttpHeaderConfig: &types.HttpHeaderConditionConfig{
					HttpHeaderName: extutil.Ptr(key),
					Values:         []string{value},
				},
			})
		}
	}
	if len(state.ConditionQueryString) > 0 {
		queryValues := make([]types.QueryStringKeyValuePair, 0)
		for key, value := range state.ConditionQueryString {
			queryValues = append(queryValues, types.QueryStringKeyValuePair{
				Key:   extutil.Ptr(key),
				Value: extutil.Ptr(value),
			})
		}
		conditions = append(conditions, types.RuleCondition{
			Field: extutil.Ptr("query-string"),
			QueryStringConfig: &types.QueryStringConditionConfig{
				Values: queryValues,
			},
		})
	}
	if len(conditions) == 0 {
		//Add default condition
		conditions = append(conditions, types.RuleCondition{
			Field: extutil.Ptr("path-pattern"),
			PathPatternConfig: &types.PathPatternConditionConfig{
				Values: []string{"*"},
			},
		})
	}

	fixedResponseConfig := &types.FixedResponseActionConfig{
		StatusCode: extutil.Ptr(strconv.Itoa(state.ResponseStatusCode)),
	}
	if state.ResponseBody != "" {
		fixedResponseConfig.MessageBody = extutil.Ptr(state.ResponseBody)
	}
	if state.ResponseContentType != "" {
		fixedResponseConfig.ContentType = extutil.Ptr(state.ResponseContentType)
	}

	createRuleResponse, err := client.CreateRule(ctx, &elasticloadbalancingv2.CreateRuleInput{
		Priority:    extutil.Ptr(int32(1)),
		ListenerArn: &state.ListenerArn,
		Conditions:  conditions,
		Actions: []types.Action{
			{
				Type:                types.ActionTypeEnumFixedResponse,
				FixedResponseConfig: fixedResponseConfig,
			},
		},
		Tags: []types.Tag{
			{
				Key:   extutil.Ptr("steadybit-target-execution-id"),
				Value: extutil.Ptr(state.TargetExecutionId.String()),
			},
			{
				Key:   extutil.Ptr("steadybit-execution-id"),
				Value: extutil.Ptr(strconv.Itoa(state.ExecutionId)),
			},
			{
				Key:   extutil.Ptr("steadybit-experiment-key"),
				Value: extutil.Ptr(state.ExperimentKey),
			},
		},
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to add rule to listener '%s'.", state.ListenerArn), err)
	}
	log.Info().Msgf("Created rule '%s'.", *createRuleResponse.Rules[0].RuleArn)

	return nil, nil
}

const steadybitReprioritized = "steadybit-reprioritized"

func reprioritizeIfNecessary(ctx context.Context, client *albStaticResponseApi, state *AlbStaticResponseState) error {
	describeRulesResult, err := (*client).DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: &state.ListenerArn,
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to fetch listener rules for '%s'", state.ListenerArn), err)
	}

	repriorityPairs := getNewPriorityPairs(describeRulesResult)
	if len(repriorityPairs) != 0 {
		ruleArns := make([]string, 0)
		for _, rule := range repriorityPairs {
			ruleArns = append(ruleArns, *rule.RuleArn)
		}
		_, err = (*client).AddTags(ctx, &elasticloadbalancingv2.AddTagsInput{
			ResourceArns: ruleArns,
			Tags: []types.Tag{
				{
					Key:   extutil.Ptr(steadybitReprioritized),
					Value: extutil.Ptr(state.TargetExecutionId.String()),
				},
			},
		})
		log.Info().Msgf("Add steadybit tags for %d rules of listener '%s'.", len(ruleArns), state.ListenerArn)
		if err != nil {
			return extension_kit.ToError(fmt.Sprintf("Failed to add tags to %d rules.", len(repriorityPairs)), err)
		}

		_, err = (*client).SetRulePriorities(ctx, &elasticloadbalancingv2.SetRulePrioritiesInput{
			RulePriorities: repriorityPairs,
		})
		if err != nil {
			return extension_kit.ToError(fmt.Sprintf("Failed to reprioritize %d rules.", len(repriorityPairs)), err)
		}
		log.Info().Msgf("Reprioritized %d rules for listener '%s'", len(repriorityPairs), state.ListenerArn)
	}
	return nil
}

func getNewPriorityPairs(rules *elasticloadbalancingv2.DescribeRulesOutput) []types.RulePriorityPair {
	priorityWithRule := make(map[int]string)
	for _, rule := range rules.Rules {
		if rule.Priority != nil && !*rule.IsDefault {
			priorityInt, err := strconv.Atoi(*rule.Priority)
			if err == nil {
				priorityWithRule[priorityInt] = *rule.RuleArn
			}
		}
	}
	priorityWithRuleNew := make([]types.RulePriorityPair, 0)
	i := 1
	for {
		if _, ok := priorityWithRule[i]; !ok {
			break
		}
		priorityWithRuleNew = append(priorityWithRuleNew, types.RulePriorityPair{
			Priority: extutil.Ptr(int32(i + 1)),
			RuleArn:  extutil.Ptr(priorityWithRule[i]),
		})
		i++
	}
	return priorityWithRuleNew
}

func (e *albStaticResponseAction) Stop(ctx context.Context, state *AlbStaticResponseState) (*action_kit_api.StopResult, error) {
	client, err := e.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize ECS client for AWS account %s", state.Account), err)
	}

	err = deleteRuleCreatedBySteadybit(ctx, &client, state)
	if err != nil {
		return nil, err
	}
	err = restoreOldPriorities(ctx, &client, state)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func deleteRuleCreatedBySteadybit(ctx context.Context, client *albStaticResponseApi, state *AlbStaticResponseState) error {
	describeRulesResult, err := (*client).DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: &state.ListenerArn,
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to fetch rules for listener %s", state.ListenerArn), err)
	}

	for _, rule := range describeRulesResult.Rules {
		if rule.Priority != nil && *rule.Priority == "1" {
			//Check if this is really the rule created by steadybit (idempotent stop method)
			describeTagsResult, err := (*client).DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
				ResourceArns: []string{*rule.RuleArn},
			})
			if err != nil {
				return extension_kit.ToError(fmt.Sprintf("Failed to fetch tags for rule %s", *rule.RuleArn), err)
			}
			isSteadybitRule := false
			for _, tagDescription := range describeTagsResult.TagDescriptions {
				if tagDescription.Tags != nil {
					for _, tag := range tagDescription.Tags {
						if *tag.Key == "steadybit-target-execution-id" {
							isSteadybitRule = true
							break
						}
					}
				}
			}
			if isSteadybitRule {
				_, err = (*client).DeleteRule(ctx, &elasticloadbalancingv2.DeleteRuleInput{
					RuleArn: rule.RuleArn,
				})
				if err != nil {
					return extension_kit.ToError(fmt.Sprintf("Failed to delete rule '%s'", *rule.RuleArn), err)
				}
				log.Info().Msgf("Deleted rule '%s'.", *rule.RuleArn)
			}
		}
	}
	return nil
}

func restoreOldPriorities(ctx context.Context, client *albStaticResponseApi, state *AlbStaticResponseState) error {
	describeRulesResult, err := (*client).DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: &state.ListenerArn,
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to fetch rules for listener %s", state.ListenerArn), err)
	}

	restorePriorities := make([]types.RulePriorityPair, 0)
	for _, page := range utils.SplitIntoPages(describeRulesResult.Rules, 20) {
		ruleArnsInPage := make([]string, 0, len(page))
		for _, rule := range page {
			ruleArnsInPage = append(ruleArnsInPage, *rule.RuleArn)
		}
		describeTagsResult, err := (*client).DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
			ResourceArns: ruleArnsInPage,
		})
		if err != nil {
			return extension_kit.ToError(fmt.Sprintf("Failed to fetch tags for %d rules", len(page)), err)
		}
		for _, rule := range page {
			if rule.Priority == nil || *rule.IsDefault {
				continue
			}
			for _, tagDescription := range describeTagsResult.TagDescriptions {
				if *tagDescription.ResourceArn == *rule.RuleArn && tagDescription.Tags != nil {
					for _, tag := range tagDescription.Tags {
						if *tag.Key == steadybitReprioritized && *tag.Value == state.TargetExecutionId.String() {
							currentPrio, err := strconv.Atoi(*rule.Priority)
							if err != nil {
								return extension_kit.ToError(fmt.Sprintf("Failed to parse priority '%s'", *rule.Priority), err)
							}
							restorePriorities = append(restorePriorities, types.RulePriorityPair{
								Priority: extutil.Ptr(int32(currentPrio - 1)),
								RuleArn:  rule.RuleArn,
							})
						}
					}
				}
			}
		}
	}

	if len(restorePriorities) > 0 {
		restoreRuleArn := make([]string, 0)
		for _, rule := range restorePriorities {
			restoreRuleArn = append(restoreRuleArn, *rule.RuleArn)
		}
		_, err := (*client).RemoveTags(ctx, &elasticloadbalancingv2.RemoveTagsInput{
			ResourceArns: restoreRuleArn,
			TagKeys:      []string{steadybitReprioritized},
		})
		if err != nil {
			return extension_kit.ToError(fmt.Sprintf("Failed to remove tags for %d rules", len(restoreRuleArn)), err)
		}
		log.Info().Msgf("Deleted steadybit tags from %d rules of listener '%s'.", len(restorePriorities), state.ListenerArn)
		_, err = (*client).SetRulePriorities(ctx, &elasticloadbalancingv2.SetRulePrioritiesInput{
			RulePriorities: restorePriorities,
		})
		if err != nil {
			return extension_kit.ToError("Failed to restore old rule priorities.", err)
		}
		log.Info().Msgf("Restored priority for %d rules for listener '%s'", len(restorePriorities), state.ListenerArn)
	}
	return nil
}

func defaultClientProviderService(account string, region string, role *string) (albStaticResponseApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return elasticloadbalancingv2.NewFromConfig(awsAccess.AwsConfig), nil
}
