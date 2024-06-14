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

func TestEcsServiceScaleAction_Prepare(t *testing.T) {
	action := ecsServiceScaleAction{}

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError error
		wantedState *ServiceScaleState
	}{
		{
			name: "Should return config",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"duration":     "180",
					"desiredCount": "5",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"aws-ecs.cluster.arn":  {"my-cluster-arn"},
						"aws-ecs.service.arn":  {"my-service-arn"},
						"aws-ecs.service.name": {"my-service-name"},
						"aws.account":          {"42"},
					},
				}),
			}),

			wantedState: &ServiceScaleState{
				Account:      "42",
				ClusterArn:   "my-cluster-arn",
				ServiceName:  "my-service-name",
				DesiredCount: 5,
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
				assert.EqualValues(t, tt.wantedState.ServiceName, state.ServiceName)
			}
		})
	}
}

type ecsServiceClientApiMock struct {
	mock.Mock
}

func (m *ecsServiceClientApiMock) UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}
func (m *ecsServiceClientApiMock) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

func TestEcsServiceScaleAction_Start(t *testing.T) {
	// Given
	api := new(ecsServiceClientApiMock)
	api.On("DescribeServices", mock.Anything, mock.Anything).Return(&ecs.DescribeServicesOutput{
		Services: []types.Service{{DesiredCount: 2}},
	}, nil)
	api.On("UpdateService", mock.Anything, mock.MatchedBy(func(params *ecs.UpdateServiceInput) bool {
		require.Equal(t, "my-service-name", *params.Service)
		require.Equal(t, "my-cluster-arn", *params.Cluster)
		require.Equal(t, int32(5), *params.DesiredCount)
		return true
	})).Return(nil, nil)

	action := ecsServiceScaleAction{clientProvider: func(account string) (ecsServiceScaleApi, error) {
		return api, nil
	}}

	// When
	state := &ServiceScaleState{
		Account:      "42",
		ClusterArn:   "my-cluster-arn",
		ServiceName:  "my-service-name",
		DesiredCount: int32(5),
	}
	result, err := action.Start(context.Background(), state)

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, int32(2), state.InitialDesiredCount)

	api.AssertExpectations(t)
}

func TestEcsServiceScaleAction_Stop(t *testing.T) {
	// Given
	api := new(ecsServiceClientApiMock)
	api.On("UpdateService", mock.Anything, mock.MatchedBy(func(params *ecs.UpdateServiceInput) bool {
		require.Equal(t, "my-service-name", *params.Service)
		require.Equal(t, "my-cluster-arn", *params.Cluster)
		require.Equal(t, int32(2), *params.DesiredCount)
		return true
	})).Return(nil, nil)

	action := ecsServiceScaleAction{clientProvider: func(account string) (ecsServiceScaleApi, error) {
		return api, nil
	}}

	// When
	state := &ServiceScaleState{
		Account:             "42",
		ClusterArn:          "my-cluster-arn",
		ServiceName:         "my-service-name",
		DesiredCount:        int32(5),
		InitialDesiredCount: int32(2),
	}
	result, err := action.Stop(context.Background(), state)

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, int32(2), state.InitialDesiredCount)

	api.AssertExpectations(t)
}

func TestEcsServiceScaleActionForwardsError(t *testing.T) {
	// Given
	api := new(ecsServiceClientApiMock)
	api.On("DescribeServices", mock.Anything, mock.Anything).Return(&ecs.DescribeServicesOutput{
		Services: []types.Service{{DesiredCount: 2}},
	}, nil)
	api.On("UpdateService", mock.Anything, mock.MatchedBy(func(params *ecs.UpdateServiceInput) bool {
		require.Equal(t, "my-service-name", *params.Service)
		require.Equal(t, "my-cluster-arn", *params.Cluster)
		require.Equal(t, int32(5), *params.DesiredCount)
		return true
	})).Return(nil, errors.New("expected"))
	action := ecsServiceScaleAction{clientProvider: func(account string) (ecsServiceScaleApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &ServiceScaleState{
		Account:      "42",
		ClusterArn:   "my-cluster-arn",
		ServiceName:  "my-service-name",
		DesiredCount: int32(5),
	})

	// Then
	assert.ErrorContains(t, err, "Failed to scale ecs service 'my-service-name'.")
	assert.Nil(t, result)

	api.AssertExpectations(t)
}
