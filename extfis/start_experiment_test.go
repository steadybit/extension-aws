// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extfis

import (
	"context"
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
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.fis.experiment.template.id": {"template-123"},
				"aws.account":                    {"42"},
				"aws.region":                     {"us-west-1"},
			},
		}),
		ExecutionId: executionId,
	})

	// When
	action := NewFisExperimentAction()
	state := action.NewEmptyState()
	result, err := action.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.Nil(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "42", state.Account)
	assert.Equal(t, "us-west-1", state.Region)
	assert.Equal(t, "template-123", state.TemplateId)
	assert.Equal(t, executionId, state.ExecutionId)
}

type fisApiMock struct {
	mock.Mock
}

func (m *fisApiMock) StartExperiment(ctx context.Context, params *fis.StartExperimentInput, _ ...func(*fis.Options)) (*fis.StartExperimentOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*fis.StartExperimentOutput), args.Error(1)
}

func (m *fisApiMock) GetExperiment(ctx context.Context, params *fis.GetExperimentInput, _ ...func(*fis.Options)) (*fis.GetExperimentOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*fis.GetExperimentOutput), args.Error(1)
}

func (m *fisApiMock) StopExperiment(ctx context.Context, params *fis.StopExperimentInput, _ ...func(*fis.Options)) (*fis.StopExperimentOutput, error) {
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
	action := NewFisExperimentAction()
	state := action.NewEmptyState()
	state.TemplateId = "template-123"
	state.Account = "42"
	state.Region = "us-west-1"
	state.ExecutionId = executionId

	// When
	result, err := startExperiment(context.Background(), &state, func(account string, region string) (FisStartExperimentClient, error) {
		assert.Equal(t, "42", account)
		assert.Equal(t, "us-west-1", region)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, err)
	assert.Nil(t, result)
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
	action := NewFisExperimentAction()
	state := action.NewEmptyState()
	state.TemplateId = "template-123"
	state.Account = "42"
	state.Region = "us-west-1"
	state.ExecutionId = executionId
	state.ExperimentId = "EXP-123"

	// When
	result, err := statusExperiment(context.Background(), &state, func(account string, region string) (FisStatusExperimentClient, error) {
		assert.Equal(t, "42", account)
		assert.Equal(t, "us-west-1", region)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, err)
	assert.Equal(t, (*result.Messages)[0].Message, "FIS experiment summary:\nstepA: completed\nstepB: failed (Internal error.)\nstepC: cancelled\n")
	assert.True(t, result.Completed)
	assert.Equal(t, result.Error.Status, extutil.Ptr(action_kit_api.Failed))
	assert.Equal(t, result.Error.Title, "FIS Experiment failed")
	assert.Equal(t, result.Error.Detail, extutil.Ptr("stepB failed"))
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
	action := NewFisExperimentAction()
	state := action.NewEmptyState()
	state.TemplateId = "template-123"
	state.Account = "42"
	state.Region = "us-west-1"
	state.ExecutionId = executionId
	state.ExperimentId = "EXP-123"

	// When
	_, extKitErr := stopExperiment(context.Background(), &state, func(account string, region string) (FisStopExperimentClient, error) {
		assert.Equal(t, "42", account)
		assert.Equal(t, "us-west-1", region)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, extKitErr)
	assert.True(t, stopCalled)
}
