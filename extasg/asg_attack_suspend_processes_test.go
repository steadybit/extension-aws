// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extasg

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPrepareSuspendProcessesFiltersAlreadySuspended(t *testing.T) {
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{
			"processes": []any{"Launch", "HealthCheck", "AZRebalance"},
		},
		Target: new(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.asg.name":                     {"web-asg"},
				"aws.asg.suspended-processes":      {"AZRebalance"},
				"aws.account":                      {"42"},
				"aws.region":                       {"us-east-1"},
				"extension-aws.discovered-by-role": {"arn:role"},
			},
		}),
	})

	attack := asgSuspendProcessesAttack{}
	state := attack.NewEmptyState()

	_, err := attack.Prepare(context.Background(), &state, requestBody)
	require.NoError(t, err)
	assert.Equal(t, "web-asg", state.AutoScalingGroupName)
	assert.Equal(t, "42", state.Account)
	assert.Equal(t, "us-east-1", state.Region)
	assert.Equal(t, "arn:role", *state.DiscoveredByRole)
	// AZRebalance was already suspended → must be excluded
	assert.ElementsMatch(t, []string{"Launch", "HealthCheck"}, state.SuspendedProcesses)
}

func TestPrepareSuspendProcessesAllAlreadySuspended(t *testing.T) {
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{
			"processes": []any{"Launch"},
		},
		Target: new(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.asg.name":                {"web-asg"},
				"aws.asg.suspended-processes": {"Launch"},
				"aws.account":                 {"42"},
				"aws.region":                  {"us-east-1"},
			},
		}),
	})

	attack := asgSuspendProcessesAttack{}
	state := attack.NewEmptyState()
	result, err := attack.Prepare(context.Background(), &state, requestBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Messages)
	assert.Equal(t, 0, len(state.SuspendedProcesses))
}

func TestPrepareSuspendProcessesNoneSelected(t *testing.T) {
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{
			"processes": []any{},
		},
		Target: new(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.asg.name": {"web-asg"},
				"aws.account":  {"42"},
				"aws.region":   {"us-east-1"},
			},
		}),
	})

	attack := asgSuspendProcessesAttack{}
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, requestBody)
	require.Error(t, err)
}

func TestStartSuspendProcessesCallsApi(t *testing.T) {
	api := new(asgApiMock)
	api.On("SuspendProcesses", mock.Anything, mock.MatchedBy(func(p *autoscaling.SuspendProcessesInput) bool {
		require.Equal(t, "web-asg", *p.AutoScalingGroupName)
		require.ElementsMatch(t, []string{"Launch", "HealthCheck"}, p.ScalingProcesses)
		return true
	})).Return(&autoscaling.SuspendProcessesOutput{}, nil)

	attack := asgSuspendProcessesAttack{clientProvider: func(account string, region string, role *string) (AsgApi, error) {
		return api, nil
	}}
	state := AsgAttackState{
		AutoScalingGroupName: "web-asg",
		Account:              "42",
		Region:               "us-east-1",
		SuspendedProcesses:   []string{"Launch", "HealthCheck"},
	}
	_, err := attack.Start(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartSuspendProcessesNoOpWhenEmpty(t *testing.T) {
	api := new(asgApiMock)
	attack := asgSuspendProcessesAttack{clientProvider: func(account string, region string, role *string) (AsgApi, error) {
		return api, nil
	}}
	state := AsgAttackState{AutoScalingGroupName: "web-asg", Account: "42", Region: "us-east-1"}
	_, err := attack.Start(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertNotCalled(t, "SuspendProcesses", mock.Anything, mock.Anything)
}

func TestStopResumesProcesses(t *testing.T) {
	api := new(asgApiMock)
	api.On("ResumeProcesses", mock.Anything, mock.MatchedBy(func(p *autoscaling.ResumeProcessesInput) bool {
		require.Equal(t, "web-asg", *p.AutoScalingGroupName)
		require.ElementsMatch(t, []string{"Launch", "HealthCheck"}, p.ScalingProcesses)
		return true
	})).Return(&autoscaling.ResumeProcessesOutput{}, nil)

	attack := asgSuspendProcessesAttack{clientProvider: func(account string, region string, role *string) (AsgApi, error) {
		return api, nil
	}}
	state := AsgAttackState{
		AutoScalingGroupName: "web-asg",
		Account:              "42",
		Region:               "us-east-1",
		SuspendedProcesses:   []string{"Launch", "HealthCheck"},
	}
	_, err := attack.Stop(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartSuspendProcessesPropagatesError(t *testing.T) {
	api := new(asgApiMock)
	api.On("SuspendProcesses", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	attack := asgSuspendProcessesAttack{clientProvider: func(account string, region string, role *string) (AsgApi, error) {
		return api, nil
	}}
	state := AsgAttackState{AutoScalingGroupName: "web-asg", Account: "42", Region: "us-east-1", SuspendedProcesses: []string{"Launch"}}
	_, err := attack.Start(context.Background(), &state)
	assert.Error(t, err)
}
