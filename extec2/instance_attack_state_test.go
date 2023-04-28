// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEc2InstanceStateAction_Prepare(t *testing.T) {
	action := ec2InstanceStateAction{}

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError error
		wantedState *InstanceStateChangeState
	}{
		{
			name: "Should return config",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "stop",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-ec2.instance.id": {"my-instance"},
						"aws.account":         {"42"},
					},
				}),
			}),

			wantedState: &InstanceStateChangeState{
				Account:    "42",
				Action:     "stop",
				InstanceId: "my-instance",
			},
		},
		{
			name: "Should return error if account is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "stop",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-ec2.instance.id": {"my-instance"},
					},
				}),
			}),
			wantedError: extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.account' attribute.", nil)),
		},
		{
			name: "Should return error if instanceId is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "stop",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws.account": {"42"},
					},
				}),
			}),
			wantedError: extutil.Ptr(extension_kit.ToError("Target is missing the 'aws-ec2.instance.id' attribute.", nil)),
		},
		{
			name: "Should return error if action is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-ec2.instance.id": {"my-instance"},
						"aws.account":         {"42"},
					},
				}),
			}),
			wantedError: extutil.Ptr(extension_kit.ToError("Missing attack action parameter.", nil)),
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
				assert.Equal(t, tt.wantedState.Account, state.Account)
				assert.Equal(t, tt.wantedState.InstanceId, state.InstanceId)
				assert.EqualValues(t, tt.wantedState.Action, state.Action)
			}
		})
	}
}

type ec2ClientApiMock struct {
	mock.Mock
}

func (m *ec2ClientApiMock) StopInstances(ctx context.Context, params *ec2.StopInstancesInput, _ ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func (m *ec2ClientApiMock) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, _ ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func (m *ec2ClientApiMock) RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, _ ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func TestEc2InstanceStateAction_Start(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("StopInstances", mock.Anything, mock.MatchedBy(func(params *ec2.StopInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		require.Equal(t, false, *params.Hibernate)
		return true
	})).Return(nil, nil)

	action := ec2InstanceStateAction{clientProvider: func(account string) (ec2InstanceStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &InstanceStateChangeState{
		Account:    "42",
		InstanceId: "dev-worker-1",
		Action:     "stop",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestEc2InstanceStateAction_Hibernate(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("StopInstances", mock.Anything, mock.MatchedBy(func(params *ec2.StopInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		require.Equal(t, true, *params.Hibernate)
		return true
	})).Return(nil, nil)
	action := ec2InstanceStateAction{clientProvider: func(account string) (ec2InstanceStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &InstanceStateChangeState{
		Account:    "42",
		InstanceId: "dev-worker-1",
		Action:     "hibernate",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestEc2InstanceStateAction_Terminate(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("TerminateInstances", mock.Anything, mock.MatchedBy(func(params *ec2.TerminateInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		return true
	})).Return(nil, nil)
	action := ec2InstanceStateAction{clientProvider: func(account string) (ec2InstanceStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &InstanceStateChangeState{
		Account:    "42",
		InstanceId: "dev-worker-1",
		Action:     "terminate",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestEc2InstanceStateAction_Reboot(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("RebootInstances", mock.Anything, mock.MatchedBy(func(params *ec2.RebootInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		return true
	})).Return(nil, nil)
	action := ec2InstanceStateAction{clientProvider: func(account string) (ec2InstanceStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &InstanceStateChangeState{
		Account:    "42",
		InstanceId: "dev-worker-1",
		Action:     "reboot",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestStartInstanceStateChangeForwardsError(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("RebootInstances", mock.Anything, mock.MatchedBy(func(params *ec2.RebootInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		return true
	})).Return(nil, errors.New("expected"))
	action := ec2InstanceStateAction{clientProvider: func(account string) (ec2InstanceStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &InstanceStateChangeState{
		Account:    "42",
		InstanceId: "dev-worker-1",
		Action:     "reboot",
	})

	// Then
	assert.Error(t, err, "Failed to execute state change attack")
	assert.Nil(t, result)

	api.AssertExpectations(t)
}
