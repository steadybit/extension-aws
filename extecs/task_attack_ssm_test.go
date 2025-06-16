// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

var (
	mockApi = mockEcsTaskSsmApi{}

	testSsmAction = &ecsTaskSsmAction{
		clientProvider: func(account string, region string, role *string) (ecsTaskSsmApi, error) {
			return &mockApi, nil
		},
		ssmCommandInvocation: ssmCommandInvocation{
			documentVersion:  "$LATEST",
			documentName:     "MyDocument",
			getParameters:    mockGetParameters,
			stepNameToOutput: "step-0",
		},
	}

	updateTask = func(state map[string][]string) error {
		return nil
	}

	taskWithHeartbeat = &ecsTaskSsmAction{
		clientProvider: func(account string, region string, role *string) (ecsTaskSsmApi, error) {
			return &mockApi, nil
		},
		ssmCommandInvocation: ssmCommandInvocation{
			documentVersion:           "$LATEST",
			documentName:              "MyDocument",
			getParameters:             mockGetParameters,
			stepNameToOutput:          "step-0",
			updateHeartbeatParameters: &updateTask,
		},
	}

	testTarget = &action_kit_api.Target{
		Attributes: map[string][]string{
			"aws.account":      {"account"},
			"aws.region":       {"region"},
			"aws-ecs.task.arn": {"task"},
		},
	}
)

func Test_ecsTaskSsmAction_Prepare(t *testing.T) {
	tests := []struct {
		name                   string
		request                action_kit_api.PrepareActionRequestBody
		instanceInformation    *ssm.DescribeInstanceInformationOutput
		instanceInformationErr error
		wantState              TaskSsmActionState
		wantErr                assert.ErrorAssertionFunc
	}{
		{
			name: "should forward error from getParameters",
			request: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{"error": "true"},
				Target: testTarget,
			},
			wantErr: assert.Error,
		},
		{
			name: "should set parameters",
			request: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: testTarget,
			},
			instanceInformation: &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: []types.InstanceInformation{
					{InstanceId: extutil.Ptr("mi-0")},
				},
			},
			wantErr: assert.NoError,
			wantState: TaskSsmActionState{
				Account:           "account",
				Region:            "region",
				TaskArn:           "task",
				ManagedInstanceId: "mi-0",
				Parameters: map[string][]string{
					"param1": {"value1"},
					"param2": {"value2"},
				},
				Comment: "Steadybit Experiment",
			},
		},
		{
			name: "should error on managed instance error",
			request: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: testTarget,
			},
			instanceInformationErr: errors.New("test-error"),
			wantErr:                assert.Error,
		},
		{
			name: "should error on no managed instance found",
			request: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: testTarget,
			},
			instanceInformation: &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: []types.InstanceInformation{},
			},
			wantErr: assert.Error,
		},
		{
			name: "should error on multiple managed instance found",
			request: action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: testTarget,
			},
			instanceInformation: &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: []types.InstanceInformation{
					{InstanceId: extutil.Ptr("mi-0")},
					{InstanceId: extutil.Ptr("mi-1")},
				},
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApi.ExpectedCalls = nil
			mockApi.On("DescribeInstanceInformation", mock.Anything, mock.Anything, mock.Anything).Return(tt.instanceInformation, tt.instanceInformationErr)

			state := TaskSsmActionState{}
			_, err := testSsmAction.Prepare(context.Background(), &state, tt.request)
			if !tt.wantErr(t, err, fmt.Sprintf("Prepare(bg, state, %v)", tt.request)) {
				return
			}
			if err == nil {
				assert.Equalf(t, tt.wantState, state, "Prepare(bg, state, %v)", tt.request)
			}
		})
	}
}

func Test_ecsTaskSsmAction_Start(t *testing.T) {
	tests := []struct {
		name           string
		state          TaskSsmActionState
		sendCommand    *ssm.SendCommandOutput
		sendCommandErr error
		wantResultErr  bool
		wantState      TaskSsmActionState
	}{
		{
			name: "should start command",
			sendCommand: &ssm.SendCommandOutput{
				Command: &types.Command{
					CommandId: extutil.Ptr("command-0"),
				},
			},
			wantState: TaskSsmActionState{
				CommandId: "command-0",
			},
		},
		{
			name:           "should propagate error",
			sendCommandErr: errors.New("test-error"),
			wantResultErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApi.ExpectedCalls = nil
			mockApi.On("SendCommand", mock.Anything, mock.Anything, mock.Anything).Return(tt.sendCommand, tt.sendCommandErr)

			state := tt.state
			got, err := testSsmAction.Start(context.Background(), &state)
			require.NoError(t, err)

			if !tt.wantResultErr {
				assert.Nilf(t, got.Error, "Start(bg, %v)", tt.state)
			} else {
				assert.NotNilf(t, got.Error, "Start(bg, %v)", tt.state)
			}
			if err == nil {
				assert.Equalf(t, tt.wantState, state, "Start(bg, %v)", tt.state)
			}
		})
	}
}

func Test_ecsTaskSsmAction_Status(t *testing.T) {
	tests := []struct {
		name                   string
		state                  TaskSsmActionState
		commandInvocation      *ssm.GetCommandInvocationOutput
		commandInvocationErr   error
		instanceInformation    *ssm.DescribeInstanceInformationOutput
		instanceInformationErr error
		wantResultErr          bool
		wantResultCompleted    bool
		wantState              TaskSsmActionState
		wantErr                assert.ErrorAssertionFunc
	}{
		{
			name:                 "should continue on invocation not found",
			commandInvocationErr: &types.InvocationDoesNotExist{},
			wantErr:              assert.NoError,
		},
		{
			name:                 "should error on other invocation errors",
			commandInvocationErr: errors.New("test-error"),
			wantErr:              assert.Error,
		},
		{
			name:                "should successful on invocation completed",
			commandInvocation:   &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusSuccess},
			wantErr:             assert.NoError,
			wantResultCompleted: true,
			wantState:           TaskSsmActionState{CommandEnded: true},
		},
		{
			name:                "should error on invocation failed",
			commandInvocation:   &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusFailed},
			wantResultErr:       true,
			wantResultCompleted: true,
			wantErr:             assert.NoError,
			wantState:           TaskSsmActionState{CommandEnded: true},
		},
		{
			name:                "should continue on invocation in progress",
			commandInvocation:   &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusInProgress},
			instanceInformation: &ssm.DescribeInstanceInformationOutput{InstanceInformationList: []types.InstanceInformation{{InstanceId: extutil.Ptr("mi-0")}}},
			wantErr:             assert.NoError,
		},
		{
			name:                "should error on managed instance went away",
			commandInvocation:   &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusInProgress},
			instanceInformation: &ssm.DescribeInstanceInformationOutput{InstanceInformationList: []types.InstanceInformation{}},
			wantErr:             assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApi.ExpectedCalls = nil
			mockApi.On("GetCommandInvocation", mock.Anything, mock.Anything, mock.Anything).Return(tt.commandInvocation, tt.commandInvocationErr)
			mockApi.On("DescribeInstanceInformation", mock.Anything, mock.Anything, mock.Anything).Return(tt.instanceInformation, tt.instanceInformationErr)

			state := tt.state
			got, err := testSsmAction.Status(context.Background(), &state)
			if !tt.wantErr(t, err, fmt.Sprintf("Status(bg, %v)", tt.state)) {
				return
			}

			resultCompleted := false
			var resultError *action_kit_api.ActionKitError
			if got != nil {
				resultCompleted = got.Completed
				resultError = got.Error
			}
			if !tt.wantResultErr {
				assert.Nilf(t, resultError, "Status(bg, %v)", tt.state)
			} else {
				assert.NotNilf(t, resultError, "Status(bg, %v)", tt.state)
			}
			assert.Equalf(t, tt.wantResultCompleted, resultCompleted, "Status(bg, %v)", tt.state)
			if err == nil {
				assert.Equalf(t, tt.wantState, state, "Status(bg, %v)", tt.state)
			}
		})
	}
}

func Test_ecsTaskSsmAction_Stop(t *testing.T) {
	tests := []struct {
		name                 string
		state                TaskSsmActionState
		commandInvocation    *ssm.GetCommandInvocationOutput
		commandInvocationErr error
		cancelCommandErr     error
		wantResultErr        bool
		wantErr              assert.ErrorAssertionFunc
		wantCancel           bool
	}{
		{
			name:    "should do nothing if command never started",
			wantErr: assert.NoError,
		},
		{
			name:    "should do nothing if command already ended",
			state:   TaskSsmActionState{CommandEnded: true},
			wantErr: assert.NoError,
		},
		{
			name:              "should cancel command if not ended",
			state:             TaskSsmActionState{CommandId: "command-0"},
			commandInvocation: &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusCancelled},
			wantResultErr:     true,
			wantErr:           assert.NoError,
			wantCancel:        true,
		},
		{
			name:              "should propagate cancel error",
			state:             TaskSsmActionState{CommandId: "command-0"},
			commandInvocation: &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusCancelled},
			cancelCommandErr:  errors.New("test-error"),
			wantErr:           assert.Error,
			wantCancel:        true,
		},
		{
			name:              "should error when command not ends within timeout",
			state:             TaskSsmActionState{CommandId: "command-0"},
			commandInvocation: &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusInProgress},
			wantErr:           assert.Error,
			wantCancel:        true,
		},
	}

	oldMaxWaitForOutput := maxWaitForOutput
	maxWaitForOutput = 100 * time.Millisecond
	defer func() {
		maxWaitForOutput = oldMaxWaitForOutput
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApi.ExpectedCalls = nil
			mockApi.On("GetCommandInvocation", mock.Anything, mock.Anything, mock.Anything).Return(tt.commandInvocation, tt.commandInvocationErr)
			mockApi.On("CancelCommand", mock.Anything, mock.Anything, mock.Anything).Return(&ssm.CancelCommandOutput{}, tt.cancelCommandErr)

			state := tt.state
			got, err := testSsmAction.Stop(context.Background(), &state)
			if !tt.wantErr(t, err, fmt.Sprintf("Stop(bg, %v)", tt.state)) {
				return
			}
			var resultError *action_kit_api.ActionKitError
			if got != nil {
				resultError = got.Error
			}
			if !tt.wantResultErr {
				assert.Nilf(t, resultError, "Stop(bg, %v)", tt.state)
			} else {
				assert.NotNilf(t, resultError, "Stop(bg, %v)", tt.state)
			}
			if tt.wantCancel {
				mockApi.AssertCalled(t, "CancelCommand", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func Test_ecsTaskSsmActionHeartbeat(t *testing.T) {
	heartbeatDuration := 100 * time.Millisecond
	setHeartbeatTimeForTest(t, heartbeatDuration, 1*time.Second)

	var calls uint64
	mockApi.On("SendCommand", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		atomic.AddUint64(&calls, 1)
	}).Return(&ssm.SendCommandOutput{
		Command: &types.Command{
			CommandId: extutil.Ptr("command-0"),
		},
	}, nil)

	actionDuration := 1 * time.Second
	state := TaskSsmActionState{
		ExecutionId: uuid.New(),
		Duration:    actionDuration,
		CommandId:   "command-0",
	}

	startResult, err := taskWithHeartbeat.Start(context.Background(), &state)
	assert.NoError(t, err)
	assert.NotNil(t, startResult)

	time.Sleep(actionDuration)
	assert.GreaterOrEqual(t, atomic.LoadUint64(&calls), uint64(1))

	state.CommandEnded = true
	_, err = taskWithHeartbeat.Stop(context.Background(), &state)
	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	callsAfterStop := atomic.LoadUint64(&calls)
	time.Sleep(heartbeatDuration)
	assert.Equal(t, callsAfterStop, atomic.LoadUint64(&calls))
}

func Test_ecsTaskSsmActionHeartbeatTimeout(t *testing.T) {
	heartbeatTimeout := 10 * time.Millisecond
	setHeartbeatTimeForTest(t, 1*time.Second, heartbeatTimeout)

	var calls uint64
	mockApi.On("SendCommand", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		atomic.AddUint64(&calls, 1)
	}).Return(&ssm.SendCommandOutput{
		Command: &types.Command{
			CommandId: extutil.Ptr("command-0"),
		},
	}, nil)

	actionDuration := 0 * time.Second
	state := TaskSsmActionState{
		ExecutionId: uuid.New(),
		Duration:    actionDuration,
		CommandId:   "command-0",
	}

	startResult, err := taskWithHeartbeat.Start(context.Background(), &state)
	assert.NoError(t, err)
	assert.NotNil(t, startResult)

	time.Sleep(10 * time.Millisecond)
	callsAfterTimeout := atomic.LoadUint64(&calls)
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, callsAfterTimeout, atomic.LoadUint64(&calls))
}

func setHeartbeatTimeForTest(t *testing.T, testHeartbeatDuration time.Duration, testHeartbeatTimeout time.Duration) {
	originalHeartbeatDuration := heartbeatDuration
	originalHeartbeatTimeout := testHeartbeatTimeout
	t.Cleanup(func() {
		heartbeatDuration = originalHeartbeatDuration
		heartbeatTimeout = originalHeartbeatTimeout
	})
	heartbeatDuration = testHeartbeatDuration
	heartbeatTimeout = testHeartbeatTimeout
}

func mockGetParameters(req action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	if _, ok := req.Config["error"]; ok {
		return nil, fmt.Errorf("error")
	} else {
		return map[string][]string{
			"param1": {"value1"},
			"param2": {"value2"},
		}, nil
	}
}

type mockEcsTaskSsmApi struct {
	mock.Mock
}

func (m *mockEcsTaskSsmApi) SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ssm.SendCommandOutput), args.Error(1)
}

func (m *mockEcsTaskSsmApi) CancelCommand(ctx context.Context, params *ssm.CancelCommandInput, optFns ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ssm.CancelCommandOutput), args.Error(1)
}

func (m *mockEcsTaskSsmApi) DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ssm.DescribeInstanceInformationOutput), args.Error(1)
}

func (m *mockEcsTaskSsmApi) GetCommandInvocation(ctx context.Context, input *ssm.GetCommandInvocationInput, f ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	args := m.Called(ctx, input, f)
	return args.Get(0).(*ssm.GetCommandInvocationOutput), args.Error(1)
}
