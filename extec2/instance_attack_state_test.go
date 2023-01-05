// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPrepareInstanceStateChange(t *testing.T) {
	// Given
	requestBody := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws-ec2.instance.id": {"my-instance"},
			},
		}),
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareInstanceStateChange(requestBodyJson)

	// Then
	assert.Nil(t, attackErr)
	assert.Equal(t, "my-instance", state.InstanceId)
	assert.Equal(t, "stop", state.Action)
}

func TestPrepareInstanceStateChangeMustRequireAnInstanceId(t *testing.T) {
	// Given
	requestBody := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{},
		}),
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareInstanceStateChange(requestBodyJson)

	// Then
	assert.Nil(t, state)
	assert.Contains(t, attackErr.Title, "aws-ec2.instance.id")
}

func TestPrepareInstanceStateChangeMustRequireAnAction(t *testing.T) {
	// Given
	requestBody := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws-ec2.instance.id": {"my-instance"},
			},
		}),
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareInstanceStateChange(requestBodyJson)

	// Then
	assert.Nil(t, state)
	assert.Contains(t, attackErr.Title, "action")
}

func TestPrepareInstanceStateChangeMustFailOnInvalidBody(t *testing.T) {
	// When
	state, attackErr := PrepareInstanceStateChange([]byte{})

	// Then
	assert.Nil(t, state)
	assert.Contains(t, attackErr.Title, "Failed to parse request body")
}

type ec2ClientApiMock struct {
	mock.Mock
}

func (m ec2ClientApiMock) StopInstances(ctx context.Context, params *ec2.StopInstancesInput, _ ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func (m ec2ClientApiMock) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, _ ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func (m ec2ClientApiMock) RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, _ ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func TestStartInstanceStop(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientApiMock)
	mockedApi.On("StopInstances", mock.Anything, mock.MatchedBy(func(params *ec2.StopInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		require.Equal(t, false, *params.Hibernate)
		return true
	})).Return(nil, nil)
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"InstanceId": "dev-worker-1",
			"Action":     "stop",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	attackError := StartInstanceStateChange(context.Background(), requestBodyJson, mockedApi)

	// Then
	assert.Nil(t, attackError)
}

func TestStartInstanceHibernate(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientApiMock)
	mockedApi.On("StopInstances", mock.Anything, mock.MatchedBy(func(params *ec2.StopInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		require.Equal(t, true, *params.Hibernate)
		return true
	})).Return(nil, nil)
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"InstanceId": "dev-worker-1",
			"Action":     "hibernate",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	attackError := StartInstanceStateChange(context.Background(), requestBodyJson, mockedApi)

	// Then
	assert.Nil(t, attackError)
}

func TestStartInstanceTerminate(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientApiMock)
	mockedApi.On("TerminateInstances", mock.Anything, mock.MatchedBy(func(params *ec2.TerminateInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		return true
	})).Return(nil, nil)
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"InstanceId": "dev-worker-1",
			"Action":     "terminate",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	attackError := StartInstanceStateChange(context.Background(), requestBodyJson, mockedApi)

	// Then
	assert.Nil(t, attackError)
}

func TestStartInstanceReboot(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientApiMock)
	mockedApi.On("RebootInstances", mock.Anything, mock.MatchedBy(func(params *ec2.RebootInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		return true
	})).Return(nil, nil)
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"InstanceId": "dev-worker-1",
			"Action":     "reboot",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	attackError := StartInstanceStateChange(context.Background(), requestBodyJson, mockedApi)

	// Then
	assert.Nil(t, attackError)
}

func TestStartInstanceStateChangeForwardsError(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientApiMock)
	mockedApi.On("RebootInstances", mock.Anything, mock.MatchedBy(func(params *ec2.RebootInstancesInput) bool {
		require.Equal(t, "dev-worker-1", params.InstanceIds[0])
		return true
	})).Return(nil, errors.New("expected"))
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"InstanceId": "dev-worker-1",
			"Action":     "reboot",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	attackError := StartInstanceStateChange(context.Background(), requestBodyJson, mockedApi)

	// Then
	assert.Contains(t, attackError.Title, "Failed to execute state change attack")
}
