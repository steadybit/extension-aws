// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelb

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

type albStaticResponseApiMock struct {
	mock.Mock
}

func (m *albStaticResponseApiMock) DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.DescribeListenersOutput), args.Error(1)
}
func (m *albStaticResponseApiMock) DescribeRules(ctx context.Context, params *elasticloadbalancingv2.DescribeRulesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeRulesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.DescribeRulesOutput), args.Error(1)
}
func (m *albStaticResponseApiMock) SetRulePriorities(ctx context.Context, params *elasticloadbalancingv2.SetRulePrioritiesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.SetRulePrioritiesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.SetRulePrioritiesOutput), args.Error(1)
}
func (m *albStaticResponseApiMock) CreateRule(ctx context.Context, params *elasticloadbalancingv2.CreateRuleInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.CreateRuleOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.CreateRuleOutput), args.Error(1)
}
func (m *albStaticResponseApiMock) DeleteRule(ctx context.Context, params *elasticloadbalancingv2.DeleteRuleInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DeleteRuleOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.DeleteRuleOutput), args.Error(1)
}
func (m *albStaticResponseApiMock) DescribeTags(ctx context.Context, params *elasticloadbalancingv2.DescribeTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTagsOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.DescribeTagsOutput), args.Error(1)
}
func (m *albStaticResponseApiMock) AddTags(ctx context.Context, params *elasticloadbalancingv2.AddTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.AddTagsOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.AddTagsOutput), args.Error(1)
}
func (m *albStaticResponseApiMock) RemoveTags(ctx context.Context, params *elasticloadbalancingv2.RemoveTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.RemoveTagsOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*elasticloadbalancingv2.RemoveTagsOutput), args.Error(1)
}

func TestAlbStaticResponseAction_Prepare(t *testing.T) {
	// Given
	api := new(albStaticResponseApiMock)
	api.On("DescribeListeners", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeListenersOutput{
		Listeners: []types.Listener{
			{
				Port:        extutil.Ptr(int32(443)),
				ListenerArn: extutil.Ptr("my-listener-arn"),
			}},
	}, nil)

	action := albStaticResponseAction{clientProvider: func(account string) (albStaticResponseApi, error) {
		return api, nil
	}}

	targetExecutionId, _ := uuid.NewUUID()

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError error
		wantedState *AlbStaticResponseState
	}{
		{
			name: "Should return config",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":             "180",
					"listenerPort":         "443",
					"responseStatusCode":   "500",
					"responseContentType":  "text/plain",
					"responseBody":         "Steadybit killed your request",
					"conditionHostHeader":  []interface{}{"example.com", "example.org"},
					"conditionHttpMethod":  []interface{}{"GET"},
					"conditionPathPattern": []interface{}{"/test", "/test2"},
					"conditionQueryString": []interface{}{map[string]interface{}{"key": "key-1", "value": "value-1"}, map[string]interface{}{"key": "key-2", "value": "value-2"}},
					"conditionSourceIp":    []interface{}{"0.0.0.0/32", "0.0.0.1/32"},
					"conditionHttpHeader":  []interface{}{map[string]interface{}{"key": "X-HEADER", "value": "value-1"}},
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-elb.alb.arn": {"my-loadbalancer-arn"},
						"aws.account":     {"42"},
					},
				}),
				ExecutionContext: extutil.Ptr(action_kit_api.ExecutionContext{
					ExecutionId:   extutil.Ptr(5),
					ExperimentKey: extutil.Ptr("ADM-1"),
				}),
				ExecutionId: targetExecutionId,
			}),

			wantedState: &AlbStaticResponseState{
				Account:              "42",
				ListenerArn:          "my-listener-arn",
				LoadbalancerArn:      "my-loadbalancer-arn",
				ResponseBody:         "Steadybit killed your request",
				ResponseStatusCode:   500,
				ResponseContentType:  "text/plain",
				ConditionHostHeader:  []string{"example.com", "example.org"},
				ConditionHttpMethod:  []string{"GET"},
				ConditionPathPattern: []string{"/test", "/test2"},
				ConditionQueryString: map[string]string{
					"key-1": "value-1",
					"key-2": "value-2",
				},
				ConditionSourceIp: []string{"0.0.0.0/32", "0.0.0.1/32"},
				ConditionHttpHeader: map[string]string{
					"X-HEADER": "value-1",
				},
				ExecutionId:       5,
				ExperimentKey:     "ADM-1",
				TargetExecutionId: targetExecutionId,
			},
		},
		{
			name: "Should return error if too many host headers",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":            "180",
					"listenerPort":        "443",
					"conditionHostHeader": []interface{}{"example.com", "example.org", "example.net", "example.de", "example.xyz", "example.io"},
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-elb.alb.arn": {"my-loadbalancer-arn"},
						"aws.account":     {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Max 5 values allowed for conditionHostHeader", nil),
		},
		{
			name: "Should return error if too many http methods",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":            "180",
					"listenerPort":        "443",
					"conditionHttpMethod": []interface{}{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-elb.alb.arn": {"my-loadbalancer-arn"},
						"aws.account":     {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Max 5 values allowed for conditionHttpMethod", nil),
		},
		{
			name: "Should return error if too many path patterns",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":             "180",
					"listenerPort":         "443",
					"conditionPathPattern": []interface{}{"/test", "/test2", "/test3", "/test4", "/test5", "/test6"},
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-elb.alb.arn": {"my-loadbalancer-arn"},
						"aws.account":     {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Max 5 values allowed for conditionPathPattern", nil),
		},
		{
			name: "Should return error if too many query string",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":     "180",
					"listenerPort": "443",
					"conditionQueryString": []interface{}{
						map[string]interface{}{"key": "key-1", "value": "value-1"},
						map[string]interface{}{"key": "key-2", "value": "value-2"},
						map[string]interface{}{"key": "key-3", "value": "value-3"},
						map[string]interface{}{"key": "key-4", "value": "value-4"},
						map[string]interface{}{"key": "key-5", "value": "value-5"},
						map[string]interface{}{"key": "key-6", "value": "value-6"},
					},
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-elb.alb.arn": {"my-loadbalancer-arn"},
						"aws.account":     {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Max 5 values allowed for conditionQueryString", nil),
		},
		{
			name: "Should return error if too many source ips",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":          "180",
					"listenerPort":      "443",
					"conditionSourceIp": []interface{}{"0.0.0.0/32", "0.0.0.1/32", "0.0.0.2/32", "0.0.0.3/32", "0.0.0.4/32", "0.0.0.5/32", "0.0.0.6/32"},
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-elb.alb.arn": {"my-loadbalancer-arn"},
						"aws.account":     {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Max 5 values allowed for conditionSourceIp", nil),
		},
		{
			name: "Should return error if too many http header",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":     "180",
					"listenerPort": "443",
					"conditionHttpHeader": []interface{}{
						map[string]interface{}{"key": "X-HEADER", "value": "value-1"},
						map[string]interface{}{"key": "X-HEADER-2", "value": "value-2"},
					},
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-elb.alb.arn": {"my-loadbalancer-arn"},
						"aws.account":     {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Only a single header name with a single value is supported", nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := action.NewEmptyState()
			request := tt.requestBody
			//When
			_, err := action.Prepare(context.Background(), &state, request)

			//Then
			if tt.wantedError != nil {
				assert.EqualError(t, err, tt.wantedError.Error())
			}
			if tt.wantedState != nil {
				assert.NoError(t, err)
				assert.EqualValues(t, *tt.wantedState, state)
			}
		})
	}
}

func TestAlbStaticResponseAction_Start(t *testing.T) {
	// Given
	targetExecutionId, _ := uuid.NewUUID()
	api := new(albStaticResponseApiMock)
	api.On("DescribeRules", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeRulesOutput{
		Rules: []types.Rule{
			{
				Priority:  extutil.Ptr("1"),
				RuleArn:   extutil.Ptr("rule-arn-exisiting-1"),
				IsDefault: extutil.Ptr(false),
			},
			{
				Priority:  extutil.Ptr("default"),
				RuleArn:   extutil.Ptr("rule-arn-existing-default"),
				IsDefault: extutil.Ptr(true),
			},
		},
	}, nil)
	api.On("AddTags", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.AddTagsInput) bool {
		require.Equal(t, 1, len(params.ResourceArns))
		require.Equal(t, "rule-arn-exisiting-1", params.ResourceArns[0])
		require.Equal(t, 1, len(params.Tags))
		require.Equal(t, "steadybit-reprioritized", *params.Tags[0].Key)
		require.Equal(t, targetExecutionId.String(), *params.Tags[0].Value)
		return true
	})).Return(&elasticloadbalancingv2.AddTagsOutput{}, nil)
	api.On("SetRulePriorities", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.SetRulePrioritiesInput) bool {
		require.Equal(t, 1, len(params.RulePriorities))
		require.Equal(t, int32(2), *params.RulePriorities[0].Priority)
		require.Equal(t, "rule-arn-exisiting-1", *params.RulePriorities[0].RuleArn)
		return true
	})).Return(&elasticloadbalancingv2.SetRulePrioritiesOutput{}, nil)
	api.On("CreateRule", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.CreateRuleInput) bool {
		require.Equal(t, "my-listener-arn", *params.ListenerArn)
		require.Equal(t, int32(1), *params.Priority)
		require.Equal(t, 3, len(params.Tags))
		require.Equal(t, 1, len(params.Actions))
		require.Equal(t, types.ActionTypeEnumFixedResponse, params.Actions[0].Type)
		require.Equal(t, "text/plain", *(*params.Actions[0].FixedResponseConfig).ContentType)
		require.Equal(t, "500", *(*params.Actions[0].FixedResponseConfig).StatusCode)
		require.Equal(t, "Steadybit killed your request", *(*params.Actions[0].FixedResponseConfig).MessageBody)
		require.Equal(t, 6, len(params.Conditions))
		return true
	})).Return(&elasticloadbalancingv2.CreateRuleOutput{
		Rules: []types.Rule{
			{
				RuleArn: extutil.Ptr("rule-arn-created-by-steadybit"),
			},
		},
	}, nil)

	action := albStaticResponseAction{clientProvider: func(account string) (albStaticResponseApi, error) {
		return api, nil
	}}

	// When
	state := &AlbStaticResponseState{
		Account:              "42",
		ListenerArn:          "my-listener-arn",
		LoadbalancerArn:      "my-loadbalancer-arn",
		ResponseBody:         "Steadybit killed your request",
		ResponseStatusCode:   500,
		ResponseContentType:  "text/plain",
		ConditionHostHeader:  []string{"example.com"},
		ConditionHttpMethod:  []string{"GET"},
		ConditionPathPattern: []string{"/test"},
		ConditionQueryString: map[string]string{"key-1": "value-1"},
		ConditionSourceIp:    []string{"0.0.0.1/32"},
		ConditionHttpHeader:  map[string]string{"X-HEADER": "value-1"},
		ExecutionId:          5,
		ExperimentKey:        "ADM-1",
		TargetExecutionId:    targetExecutionId,
	}
	result, err := action.Start(context.Background(), state)

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestEcsServiceScaleAction_Stop(t *testing.T) {
	// Given
	targetExecutionId, _ := uuid.NewUUID()
	api := new(albStaticResponseApiMock)
	//First call is returning all rules including the one created by steadybit
	api.On("DescribeRules", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeRulesOutput{
		Rules: []types.Rule{
			{
				Priority:  extutil.Ptr("1"),
				RuleArn:   extutil.Ptr("rule-arn-created-by-steadybit"),
				IsDefault: extutil.Ptr(false),
			},
			{
				Priority:  extutil.Ptr("2"),
				RuleArn:   extutil.Ptr("rule-arn-exisiting-1"),
				IsDefault: extutil.Ptr(false),
			},
			{
				Priority:  extutil.Ptr("default"),
				RuleArn:   extutil.Ptr("rule-arn-existing-default"),
				IsDefault: extutil.Ptr(true),
			},
		},
	}, nil).Once()
	api.On("DescribeTags", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.DescribeTagsInput) bool {
		if len(params.ResourceArns) == 1 {
			require.Equal(t, "rule-arn-created-by-steadybit", params.ResourceArns[0])
			return true
		}
		return false
	})).Return(&elasticloadbalancingv2.DescribeTagsOutput{
		TagDescriptions: []types.TagDescription{
			{
				Tags: []types.Tag{
					{
						Key:   extutil.Ptr("steadybit-target-execution-id"),
						Value: extutil.Ptr(targetExecutionId.String()),
					},
				},
			},
		},
	}, nil).Once()
	//Second call after delete is only returning the previous existing rules
	api.On("DescribeRules", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeRulesOutput{
		Rules: []types.Rule{
			{
				IsDefault: extutil.Ptr(false),
				Priority:  extutil.Ptr("2"),
				RuleArn:   extutil.Ptr("rule-arn-exisiting-1"),
			},
			{
				IsDefault: extutil.Ptr(true),
				Priority:  extutil.Ptr("default"),
				RuleArn:   extutil.Ptr("rule-arn-existing-default"),
			},
		},
	}, nil).Once()
	api.On("DescribeTags", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.DescribeTagsInput) bool {
		if len(params.ResourceArns) == 2 {
			require.Equal(t, "rule-arn-exisiting-1", params.ResourceArns[0])
			require.Equal(t, "rule-arn-existing-default", params.ResourceArns[1])
			return true
		}
		return false
	})).Return(&elasticloadbalancingv2.DescribeTagsOutput{
		TagDescriptions: []types.TagDescription{
			{
				ResourceArn: extutil.Ptr("rule-arn-exisiting-1"),
				Tags: []types.Tag{
					{
						Key:   extutil.Ptr("steadybit-reprioritized"),
						Value: extutil.Ptr(targetExecutionId.String()),
					},
				},
			},
		},
	}, nil).Once()
	api.On("DeleteRule", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.DeleteRuleInput) bool {
		require.Equal(t, "rule-arn-created-by-steadybit", *params.RuleArn)
		return true
	})).Return(&elasticloadbalancingv2.DeleteRuleOutput{}, nil)
	api.On("RemoveTags", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.RemoveTagsInput) bool {
		require.Equal(t, 1, len(params.ResourceArns))
		require.Equal(t, "rule-arn-exisiting-1", params.ResourceArns[0])
		require.Equal(t, 1, len(params.TagKeys))
		require.Equal(t, "steadybit-reprioritized", params.TagKeys[0])
		return true
	})).Return(&elasticloadbalancingv2.RemoveTagsOutput{}, nil)
	api.On("SetRulePriorities", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.SetRulePrioritiesInput) bool {
		require.Equal(t, 1, len(params.RulePriorities))
		require.Equal(t, int32(1), *params.RulePriorities[0].Priority)
		require.Equal(t, "rule-arn-exisiting-1", *params.RulePriorities[0].RuleArn)
		return true
	})).Return(&elasticloadbalancingv2.SetRulePrioritiesOutput{}, nil)

	action := albStaticResponseAction{clientProvider: func(account string) (albStaticResponseApi, error) {
		return api, nil
	}}

	// When
	state := &AlbStaticResponseState{
		Account:           "42",
		ListenerArn:       "my-listener-arn",
		LoadbalancerArn:   "my-loadbalancer-arn",
		TargetExecutionId: targetExecutionId,
	}
	result, err := action.Stop(context.Background(), state)

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func Test_getNewPriorityPairs(t *testing.T) {
	type args struct {
		rules *elasticloadbalancingv2.DescribeRulesOutput
	}
	tests := []struct {
		name string
		args args
		want []types.RulePriorityPair
	}{
		{
			name: "Should return empty list if no rules",
			args: args{
				rules: &elasticloadbalancingv2.DescribeRulesOutput{},
			},
			want: []types.RulePriorityPair{},
		},
		{
			name: "Should not move anything if prio 1 is not used",
			args: args{
				rules: &elasticloadbalancingv2.DescribeRulesOutput{
					Rules: []types.Rule{
						{
							Priority: extutil.Ptr("2"),
							RuleArn:  extutil.Ptr("rule-arn-a"),
						},
						{
							IsDefault: extutil.Ptr(true),
							Priority:  extutil.Ptr("default"),
							RuleArn:   extutil.Ptr("rule-arn-b"),
						},
					},
				},
			},
			want: []types.RulePriorityPair{},
		},
		{
			name: "Should move existing rules",
			args: args{
				rules: &elasticloadbalancingv2.DescribeRulesOutput{
					Rules: []types.Rule{
						{
							IsDefault: extutil.Ptr(false),
							Priority:  extutil.Ptr("1"),
							RuleArn:   extutil.Ptr("rule-arn-a"),
						},
						{
							IsDefault: extutil.Ptr(false),
							Priority:  extutil.Ptr("2"),
							RuleArn:   extutil.Ptr("rule-arn-b"),
						},
						{
							IsDefault: extutil.Ptr(false),
							Priority:  extutil.Ptr("3"),
							RuleArn:   extutil.Ptr("rule-arn-c"),
						},
						{
							IsDefault: extutil.Ptr(false),
							Priority:  extutil.Ptr("5"),
							RuleArn:   extutil.Ptr("rule-arn-d"),
						},
						{
							IsDefault: extutil.Ptr(false),
							Priority:  extutil.Ptr("6"),
							RuleArn:   extutil.Ptr("rule-arn-e"),
						},
						{
							IsDefault: extutil.Ptr(true),
							Priority:  extutil.Ptr("default"),
							RuleArn:   extutil.Ptr("rule-arn-f"),
						},
					},
				},
			},
			want: []types.RulePriorityPair{
				{
					Priority: extutil.Ptr(int32(2)),
					RuleArn:  extutil.Ptr("rule-arn-a"),
				},
				{
					Priority: extutil.Ptr(int32(3)),
					RuleArn:  extutil.Ptr("rule-arn-b"),
				},
				{
					Priority: extutil.Ptr(int32(4)),
					RuleArn:  extutil.Ptr("rule-arn-c"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, getNewPriorityPairs(tt.args.rules), "getNewPriorityPairs(%v)", tt.args.rules)
		})
	}
}
