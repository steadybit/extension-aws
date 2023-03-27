// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extfis

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPrepareInstanceReboot(t *testing.T) {
	// Given
	executionId, _ := uuid.NewRandom()
	requestBody := action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.fis.experiment.template.id": {"template-123"},
				"aws.account":                    {"42"},
			},
		}),
		ExecutionId: executionId,
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareExperiment(requestBodyJson)

	// Then
	assert.Nil(t, attackErr)
	assert.Equal(t, "42", state.Account)
	assert.Equal(t, "template-123", state.TemplateId)
	assert.Equal(t, executionId, state.ExecutionId)
}

type fisApiMock struct {
	mock.Mock
}

func (m fisApiMock) StartExperiment(ctx context.Context, params *fis.StartExperimentInput, optFns ...func(*fis.Options)) (*fis.StartExperimentOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*fis.StartExperimentOutput), args.Error(1)
}

func (m fisApiMock) GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*fis.GetExperimentOutput), args.Error(1)
}

func (m fisApiMock) StopExperiment(ctx context.Context, params *fis.StopExperimentInput, optFns ...func(*fis.Options)) (*fis.StopExperimentOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*fis.StopExperimentOutput), args.Error(1)
}

func TestStartExperiment(t *testing.T) {
	// Given
	mockedApi := new(fisApiMock)
	mockedApi.On("StartExperiment", mock.Anything, mock.MatchedBy(func(params *fis.StartExperimentInput) bool {
		require.Equal(t, "template-123", *params.ExperimentTemplateId)
		return true
	})).Return(&fis.StartExperimentOutput{
		Experiment: &types.Experiment{
			Id: extutil.Ptr("EXP-123"),
		},
	}, nil)

	executionId, _ := uuid.NewRandom()
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"TemplateId":  "template-123",
			"Account":     "42",
			"ExecutionId": executionId.String(),
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, extKitErr := StartExperiment(context.Background(), requestBodyJson, func(account string) (FisStartExperimentClient, error) {
		assert.Equal(t, "42", account)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, extKitErr)
	assert.Equal(t, "EXP-123", state.ExperimentId)
}

func TestStatusExperiment(t *testing.T) {
	// Given
	mockedApi := new(fisApiMock)
	mockedApi.On("GetExperiment", mock.Anything, mock.MatchedBy(func(params *fis.GetExperimentInput) bool {
		require.Equal(t, "EXP-123", *params.Id)
		return true
	})).Return(&fis.GetExperimentOutput{
		Experiment: &types.Experiment{
			Id: extutil.Ptr("EXP-123"),
			Actions: map[string]types.ExperimentAction{
				"stepC": {
					State: &types.ExperimentActionState{
						Status: types.ExperimentActionStatusCancelled,
					},
				},
				"stepA": {
					State: &types.ExperimentActionState{
						Status: types.ExperimentActionStatusCompleted,
					},
				},
				"stepB": {
					State: &types.ExperimentActionState{
						Status: types.ExperimentActionStatusFailed,
						Reason: extutil.Ptr("Internal error."),
					},
				},
			},
			State: &types.ExperimentState{
				Status: types.ExperimentStatusFailed,
				Reason: extutil.Ptr("stepB failed"),
			},
		},
	}, nil)

	executionId, _ := uuid.NewRandom()
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"TemplateId":   "template-123",
			"Account":      "42",
			"ExecutionId":  executionId.String(),
			"ExperimentId": "EXP-123",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, extKitErr := StatusExperiment(context.Background(), requestBodyJson, func(account string) (FisStatusExperimentClient, error) {
		assert.Equal(t, "42", account)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, extKitErr)
	assert.Equal(t, (*state.Messages)[0].Message, "stepA: completed\nstepB: failed (Internal error.)\nstepC: cancelled\n")
	assert.True(t, state.Completed)
	assert.Equal(t, state.Error.Status, extutil.Ptr(action_kit_api.Failed))
	assert.Equal(t, state.Error.Title, "FIS Experiment failed")
	assert.Equal(t, state.Error.Detail, extutil.Ptr("stepB failed"))
}

func TestStopExperiment(t *testing.T) {
	// Given
	mockedApi := new(fisApiMock)
	mockedApi.On("GetExperiment", mock.Anything, mock.MatchedBy(func(params *fis.GetExperimentInput) bool {
		require.Equal(t, "EXP-123", *params.Id)
		return true
	})).Return(&fis.GetExperimentOutput{
		Experiment: &types.Experiment{
			Id: extutil.Ptr("EXP-123"),
			State: &types.ExperimentState{
				Status: types.ExperimentStatusRunning,
			},
		},
	}, nil)

	stopCalled := false
	mockedApi.On("StopExperiment", mock.Anything, mock.MatchedBy(func(params *fis.StopExperimentInput) bool {
		require.Equal(t, "EXP-123", *params.Id)
		stopCalled = true
		return true
	})).Return(&fis.StopExperimentOutput{}, nil)

	executionId, _ := uuid.NewRandom()
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"TemplateId":   "template-123",
			"Account":      "42",
			"ExecutionId":  executionId.String(),
			"ExperimentId": "EXP-123",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	_, extKitErr := StopExperiment(context.Background(), requestBodyJson, func(account string) (FisStopExperimentClient, error) {
		assert.Equal(t, "42", account)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, extKitErr)
	assert.True(t, stopCalled)
}
