// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extecs

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEcsTaskStopAction_Prepare(t *testing.T) {
	action := ecsTaskStopAction{}

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError error
		wantedState *TaskStopState
	}{
		{
			name: "Should return config",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-ecs.cluster.arn": {"my-cluster-arn"},
						"aws-ecs.task.arn":    {"my-task-arn"},
						"aws.account":         {"42"},
					},
				}),
			}),

			wantedState: &TaskStopState{
				Account:    "42",
				ClusterArn: "my-cluster-arn",
				TaskArn:    "my-task-arn",
			},
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
				assert.Equal(t, tt.wantedState.ClusterArn, state.ClusterArn)
				assert.EqualValues(t, tt.wantedState.TaskArn, state.TaskArn)
			}
		})
	}
}

type ecsClientApiMock struct {
	mock.Mock
}

func (m *ecsClientApiMock) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeTasksOutput), nil
}

func (m *ecsClientApiMock) StopTask(ctx context.Context, params *ecs.StopTaskInput, optFns ...func(*ecs.Options)) (*ecs.StopTaskOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func TestEcsTaskStopAction_Start(t *testing.T) {
	// Given
	api := new(ecsClientApiMock)
	api.On("StopTask", mock.Anything, mock.MatchedBy(func(params *ecs.StopTaskInput) bool {
		require.Equal(t, "my-task-arn", *params.Task)
		require.Equal(t, "my-cluster-arn", *params.Cluster)
		return true
	})).Return(nil, nil)
	api.On("DescribeTasks", mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				LastStatus: extutil.Ptr("RUNNING"),
			},
		},
	})

	action := ecsTaskStopAction{clientProvider: func(account string) (ecsTaskStopApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &TaskStopState{
		Account:    "42",
		ClusterArn: "my-cluster-arn",
		TaskArn:    "my-task-arn",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestEcsTaskStopAction_Start_already_stopped_task(t *testing.T) {
	// Given
	api := new(ecsClientApiMock)
	api.On("DescribeTasks", mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				LastStatus: extutil.Ptr("STOPPED"),
			},
		},
	})

	action := ecsTaskStopAction{clientProvider: func(account string) (ecsTaskStopApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &TaskStopState{
		Account:    "42",
		ClusterArn: "my-cluster-arn",
		TaskArn:    "my-task-arn",
	})

	// Then
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Task not running", result.Error.Title)

	api.AssertExpectations(t)
}

func TestEcsTaskStopActionForwardsError(t *testing.T) {
	// Given
	api := new(ecsClientApiMock)
	api.On("DescribeTasks", mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				LastStatus: extutil.Ptr("RUNNING"),
			},
		},
	})
	api.On("StopTask", mock.Anything, mock.MatchedBy(func(params *ecs.StopTaskInput) bool {
		require.Equal(t, "my-task-arn", *params.Task)
		require.Equal(t, "my-cluster-arn", *params.Cluster)
		return true
	})).Return(&ecs.StopTaskOutput{}, errors.New("expected"))
	action := ecsTaskStopAction{clientProvider: func(account string) (ecsTaskStopApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &TaskStopState{
		Account:    "42",
		ClusterArn: "my-cluster-arn",
		TaskArn:    "my-task-arn",
	})

	// Then
	assert.ErrorContains(t, err, "Failed to stop ecs task 'my-task-arn'.")
	assert.Nil(t, result)

	api.AssertExpectations(t)
}
