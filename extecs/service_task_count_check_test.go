// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

type ecsDescribeServicesApiMock struct {
	mock.Mock
}

func (m *ecsDescribeServicesApiMock) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

func TestServiceTaskCountCheck_prepare_saves_initial_state(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	poller := NewServiceDescriptionPoller()
	poller.ticker = time.NewTicker(1 * time.Millisecond)
	poller.apiClientProvider = func(account string) (ecsDescribeServicesApi, error) {
		mockedApi := new(ecsDescribeServicesApiMock)
		mockedApi.On("DescribeServices", mock.Anything, mock.Anything).Return(&ecs.DescribeServicesOutput{
			Services: []types.Service{{
				ServiceArn:   extutil.Ptr("service-arn"),
				DesiredCount: 3,
				RunningCount: 2,
			}},
		}, nil)
		return mockedApi, nil
	}
	poller.Start(ctx)

	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"Duration":              100,
			"RunningCountCheckMode": "runningCountEqualsDesiredCount",
		},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":         {"42"},
				"aws-ecs.service.arn": {"service-arn"},
				"aws-ecs.cluster.arn": {"cluster-arn"},
			},
		}),
	})

	action := ServiceTaskCountCheckAction{
		poller: poller,
	}
	state := action.NewEmptyState()

	// When
	_, err := action.Prepare(ctx, &state, request)
	cancel()

	// Then
	assert.NoError(t, err)
	assert.LessOrEqual(t, state.Timeout, time.Now().Add(time.Second*100))
	assert.Equal(t, state.AwsAccount, "42")
	assert.Equal(t, state.ClusterArn, "cluster-arn")
	assert.Equal(t, state.ServiceArn, "service-arn")
	assert.Equal(t, state.InitialRunningCount, 2)
}

func TestServiceTaskCountCheck_status_checks_running_count(t *testing.T) {
	tests := []struct {
		name    string
		service types.Service
		state   ServiceTaskCountCheckState
		mode    string
		wanted  func(t *testing.T, result *action_kit_api.StatusResult)
	}{
		{
			name: "successful_check_completes_run",
			service: types.Service{
				RunningCount: 1,
			},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: runningCountMin1,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.True(t, result.Completed)
				assert.Nil(t, result.Error)
			},
		},
		{
			name: "completed_check_on_timeout",
			service: types.Service{
				RunningCount: 0,
			},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: runningCountMin1,
				Timeout:               time.Now().Add(-10 * time.Second),
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.True(t, result.Completed)
			},
		},
		{
			name: "runningCountMin1_check_failed",
			service: types.Service{
				RunningCount: 0,
			},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: runningCountMin1,
				Timeout:               time.Now().Add(-10 * time.Second),
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.Equal(t, action_kit_api.Failed, *result.Error.Status)
				assert.Contains(t, result.Error.Title, "no running task")
			},
		},
		{
			name: "runningCountEqualsDesiredCount_check_failed",
			service: types.Service{
				RunningCount: 1,
				DesiredCount: 2,
			},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: runningCountEqualsDesiredCount,
				Timeout:               time.Now().Add(-10 * time.Second),
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.Equal(t, action_kit_api.Failed, *result.Error.Status)
				assert.Contains(t, result.Error.Title, "1 of desired 2")
			},
		},
		{
			name: "runningCountLessThanDesiredCount_check_failed",
			service: types.Service{
				RunningCount: 1,
				DesiredCount: 1,
			},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: runningCountLessThanDesiredCount,
				Timeout:               time.Now().Add(-10 * time.Second),
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.Equal(t, action_kit_api.Failed, *result.Error.Status)
				assert.Contains(t, result.Error.Title, "has all 1 desired")
			},
		},
		{
			name: "runningCountIncreased_check_failed",
			service: types.Service{
				RunningCount: 2,
			},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: runningCountIncreased,
				Timeout:               time.Now().Add(-10 * time.Second),
				InitialRunningCount:   2,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.Equal(t, action_kit_api.Failed, *result.Error.Status)
				assert.Contains(t, result.Error.Title, "didn't increase")
			},
		},
		{
			name: "runningCountDecreased_check_failed",
			service: types.Service{
				RunningCount: 2,
			},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: runningCountDecreased,
				Timeout:               time.Now().Add(-10 * time.Second),
				InitialRunningCount:   2,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.Equal(t, action_kit_api.Failed, *result.Error.Status)
				assert.Contains(t, result.Error.Title, "didn't decrease")
			},
		},
		{
			name:    "wrongMode",
			service: types.Service{},
			state: ServiceTaskCountCheckState{
				RunningCountCheckMode: "notExisting",
				Timeout:               time.Now().Add(-10 * time.Second),
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult) {
				assert.Equal(t, action_kit_api.Failed, *result.Error.Status)
				assert.Contains(t, result.Error.Title, "unsupported check type")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			poller := NewServiceDescriptionPoller()
			poller.apiClientProvider = func(account string) (ecsDescribeServicesApi, error) {
				mockedApi := new(ecsDescribeServicesApiMock)
				mockedApi.On("DescribeServices", mock.Anything, mock.Anything).Return(&ecs.DescribeServicesOutput{
					Services: []types.Service{test.service},
				}, nil)
				return mockedApi, nil
			}
			poller.Register(test.state.AwsAccount, test.state.ClusterArn, test.state.ServiceArn)
			poller.pollAll(ctx)

			action := ServiceTaskCountCheckAction{
				poller: poller,
			}

			// When
			_, err := action.Start(ctx, &test.state)
			assert.NoError(t, err)
			result, err := action.Status(ctx, &test.state)
			assert.NoError(t, err)
			_, err = action.Stop(ctx, &test.state)
			assert.NoError(t, err)

			// Then
			assert.NoError(t, err)
			test.wanted(t, result)
		})
	}
}
