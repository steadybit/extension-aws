/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extlambda

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestLambdaAction_Prepare(t *testing.T) {
	config := FailureInjectionConfig{}
	action := lambdaAction{configProvider: func(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
		return &config, nil
	}}

	tests := []struct {
		name        string
		attributes  map[string][]string
		wantedError error
		wantedState *LambdaActionState
	}{
		{
			name: "Should return config",
			attributes: map[string][]string{
				"aws.account":                        {"123456789012"},
				"aws.lambda.failure-injection-param": {"PARAM"},
			},
			wantedState: &LambdaActionState{
				Account: "123456789012",
				Param:   "PARAM",
				Config:  &config,
			},
		},
		{
			name: "Should return error if account is missing",
			attributes: map[string][]string{
				"aws.lambda.failure-injection-param": {"PARAM"},
			},
			wantedError: extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.account' attribute.", nil)),
		},
		{
			name: "Should return error if failure-injection-param is missing",
			attributes: map[string][]string{
				"aws.account": {"123456789012"},
			},
			wantedError: extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.lambda.failure-injection-param' attribute. Did you wrap the lambda with https://github.com/gunnargrosch/failure-lambda ?", nil)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := action.NewEmptyState()
			request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Target: &action_kit_api.Target{
					Attributes: tt.attributes,
				},
			})

			//When
			_, err := action.Prepare(context.Background(), &state, request)

			//Then
			if tt.wantedError != nil {
				assert.EqualError(t, err, tt.wantedError.Error())
			}
			if tt.wantedState != nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantedState.Account, state.Account)
				assert.Equal(t, tt.wantedState.Param, state.Param)
				assert.EqualValues(t, tt.wantedState.Config, state.Config)
			}
		})
	}
}

func TestLambdaAction_Start(t *testing.T) {
	api := new(ssmClientMock)
	api.On("PutParameter", mock.Anything, &ssm.PutParameterInput{
		Name:        extutil.Ptr("PARAM"),
		Value:       extutil.Ptr("{\"failureMode\":\"test\",\"rate\":0.5,\"isEnabled\":true}"),
		Type:        types.ParameterTypeString,
		DataType:    extutil.Ptr("text"),
		Description: extutil.Ptr("lambda failure injection config - set by steadybit"),
		Overwrite:   extutil.Ptr(true),
	}, mock.Anything).Return(&ssm.PutParameterOutput{}, nil)
	api.On("AddTagsToResource", mock.Anything, &ssm.AddTagsToResourceInput{
		ResourceId:   extutil.Ptr("PARAM"),
		ResourceType: types.ResourceTypeForTaggingParameter,
		Tags:         []types.Tag{{Key: extutil.Ptr("created-by"), Value: extutil.Ptr("steadybit")}},
	}, mock.Anything).Return(&ssm.AddTagsToResourceOutput{}, nil)

	action := lambdaAction{
		clientProvider: func(account string) (ssmApi, error) {
			return api, nil
		},
	}
	state := action.NewEmptyState()
	state.Account = "123456789012"
	state.Param = "PARAM"
	state.Config = &FailureInjectionConfig{
		IsEnabled:   true,
		FailureMode: "test",
		Rate:        0.5,
	}

	result, err := action.Start(context.Background(), &state)
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestLambdaAction_Stop(t *testing.T) {
	api := new(ssmClientMock)
	api.On("DeleteParameter", mock.Anything, &ssm.DeleteParameterInput{
		Name: extutil.Ptr("PARAM"),
	}, mock.Anything).Return(&ssm.DeleteParameterOutput{}, nil)

	action := lambdaAction{
		clientProvider: func(account string) (ssmApi, error) {
			return api, nil
		},
	}
	state := action.NewEmptyState()
	state.Account = "123456789012"
	state.Param = "PARAM"

	result, err := action.Stop(context.Background(), &state)
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

type ssmClientMock struct {
	mock.Mock
}

func (m *ssmClientMock) PutParameter(ctx context.Context, s *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	args := m.Called(ctx, s, optFns)
	return args.Get(0).(*ssm.PutParameterOutput), args.Error(1)
}

func (m *ssmClientMock) DeleteParameter(ctx context.Context, s *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	args := m.Called(ctx, s, optFns)
	return args.Get(0).(*ssm.DeleteParameterOutput), args.Error(1)
}

func (m *ssmClientMock) AddTagsToResource(ctx context.Context, s *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
	args := m.Called(ctx, s, optFns)
	return args.Get(0).(*ssm.AddTagsToResourceOutput), args.Error(1)
}

func Test_injectStatusCode(t *testing.T) {
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"statuscode": 500.0,
			"rate":       50.0,
		},
	})

	config, err := injectStatusCode(request)
	assert.NoError(t, err)
	assert.EqualValues(t, "statuscode", config.FailureMode)
	assert.EqualValues(t, 500, *config.StatusCode)
	assert.EqualValues(t, 0.5, config.Rate)
	assert.EqualValues(t, true, config.IsEnabled)
}

func Test_injectLatency(t *testing.T) {
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"minLatency": 200.0,
			"maxLatency": 300.0,
			"rate":       50.0,
		},
	})

	config, err := injectLatency(request)
	assert.NoError(t, err)
	assert.EqualValues(t, "latency", config.FailureMode)
	assert.EqualValues(t, 200, *config.MinLatency)
	assert.EqualValues(t, 300, *config.MaxLatency)
	assert.EqualValues(t, 0.5, config.Rate)
	assert.EqualValues(t, true, config.IsEnabled)
}

func Test_fillDiskspace(t *testing.T) {
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"diskSpace": 128.0,
			"rate":      100.0,
		},
	})

	config, err := fillDiskspace(request)
	assert.NoError(t, err)
	assert.EqualValues(t, "diskspace", config.FailureMode)
	assert.EqualValues(t, 128, *config.DiskSpace)
	assert.EqualValues(t, 1.0, config.Rate)
	assert.EqualValues(t, true, config.IsEnabled)
}

func Test_injectException(t *testing.T) {
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"exceptionMsg": "Error",
			"rate":         25.0,
		},
	})

	config, err := injectException(request)
	assert.NoError(t, err)
	assert.EqualValues(t, "exception", config.FailureMode)
	assert.EqualValues(t, "Error", *config.ExceptionMsg)
	assert.EqualValues(t, 0.25, config.Rate)
	assert.EqualValues(t, true, config.IsEnabled)
}

func toGenericArray(arr ...interface{}) []interface{} {
	return arr
}
func Test_denyConnection(t *testing.T) {
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"denylist": toGenericArray(".*.google.com", ".*"),
			"rate":     25.0,
		},
	})

	config, err := denyConnection(request)
	assert.NoError(t, err)
	assert.EqualValues(t, "denylist", config.FailureMode)
	assert.EqualValues(t, []string{".*.google.com", ".*"}, *config.Denylist)
	assert.EqualValues(t, 0.25, config.Rate)
	assert.EqualValues(t, true, config.IsEnabled)
}
