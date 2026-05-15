// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type ebsPauseIoApiMock struct {
	mock.Mock
}

func (m *ebsPauseIoApiMock) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}
func (m *ebsPauseIoApiMock) DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ssm.DescribeInstanceInformationOutput), args.Error(1)
}
func (m *ebsPauseIoApiMock) SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ssm.SendCommandOutput), args.Error(1)
}
func (m *ebsPauseIoApiMock) GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ssm.GetCommandInvocationOutput), args.Error(1)
}

func newPauseIoRequest() action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{"duration": "30s"},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":                           {"42"},
				"aws.region":                            {"us-east-1"},
				"aws.ebs.volume.id":                     {"vol-0123456789abcdef0"},
				"aws.ebs.volume.attachment.instance-id": {"i-deadbeef"},
				"aws.ebs.volume.attachment.device":      {"/dev/sdf"},
				"extension-aws.discovered-by-role":      {"arn:role"},
			},
		}),
	})
}

func newPauseIoAttack(api *ebsPauseIoApiMock) ebsPauseIoAttack {
	return ebsPauseIoAttack{
		clientProvider: func(account string, region string, role *string) (ebsPauseIoApi, error) { return api, nil },
	}
}

func mockLinuxOnlineInstance(api *ebsPauseIoApiMock) {
	api.On("DescribeInstances", mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{{
				InstanceId:      aws.String("i-deadbeef"),
				PlatformDetails: aws.String("Linux/UNIX"),
			}},
		}},
	}, nil)
	api.On("DescribeInstanceInformation", mock.Anything, mock.Anything).Return(&ssm.DescribeInstanceInformationOutput{
		InstanceInformationList: []ssmtypes.InstanceInformation{{
			InstanceId: aws.String("i-deadbeef"),
			PingStatus: ssmtypes.PingStatusOnline,
		}},
	}, nil)
}

func TestPauseIoPrepareSuccess(t *testing.T) {
	api := new(ebsPauseIoApiMock)
	mockLinuxOnlineInstance(api)
	a := newPauseIoAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newPauseIoRequest())
	require.NoError(t, err)
	assert.Equal(t, "vol-0123456789abcdef0", state.VolumeId)
	assert.Equal(t, "i-deadbeef", state.InstanceId)
	assert.Equal(t, "/dev/sdf", state.AwsDeviceName)
}

func TestPauseIoPrepareRejectsUnattached(t *testing.T) {
	req := newPauseIoRequest()
	req.Target.Attributes["aws.ebs.volume.attachment.instance-id"] = []string{""}
	a := newPauseIoAttack(new(ebsPauseIoApiMock))
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, req)
	require.Error(t, err)
}

func TestPauseIoPrepareRejectsWindows(t *testing.T) {
	api := new(ebsPauseIoApiMock)
	api.On("DescribeInstances", mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{{
				InstanceId:      aws.String("i-deadbeef"),
				PlatformDetails: aws.String("Windows"),
				Platform:        ec2types.PlatformValuesWindows,
			}},
		}},
	}, nil)
	a := newPauseIoAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newPauseIoRequest())
	require.Error(t, err)
}

func TestPauseIoPrepareRejectsSsmNotOnline(t *testing.T) {
	api := new(ebsPauseIoApiMock)
	api.On("DescribeInstances", mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{InstanceId: aws.String("i-deadbeef"), PlatformDetails: aws.String("Linux/UNIX")}}}},
	}, nil)
	api.On("DescribeInstanceInformation", mock.Anything, mock.Anything).Return(&ssm.DescribeInstanceInformationOutput{
		InstanceInformationList: []ssmtypes.InstanceInformation{{
			InstanceId: aws.String("i-deadbeef"),
			PingStatus: ssmtypes.PingStatusConnectionLost,
		}},
	}, nil)
	a := newPauseIoAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newPauseIoRequest())
	require.Error(t, err)
}

func TestPauseIoStartSendsFreezeCommand(t *testing.T) {
	api := new(ebsPauseIoApiMock)
	api.On("SendCommand", mock.Anything, mock.MatchedBy(func(in *ssm.SendCommandInput) bool {
		require.Equal(t, []string{"i-deadbeef"}, in.InstanceIds)
		require.Equal(t, "AWS-RunShellScript", aws.ToString(in.DocumentName))
		commands := in.Parameters["commands"]
		require.GreaterOrEqual(t, len(commands), 2)
		// Volume ID exported into the shell environment for the freeze script.
		joined := commands[0] + "\n" + commands[1]
		require.Contains(t, joined, "vol-0123456789abcdef0")
		// AWS device suffix passed through for the Xen fallback.
		require.Contains(t, joined, "AWS_DEV_SUFFIX")
		// Last command is the freeze script body.
		require.Contains(t, commands[len(commands)-1], "fsfreeze --freeze")
		return true
	})).Return(&ssm.SendCommandOutput{Command: &ssmtypes.Command{CommandId: aws.String("cmd-1")}}, nil)
	api.On("GetCommandInvocation", mock.Anything, mock.Anything).Return(&ssm.GetCommandInvocationOutput{
		Status: ssmtypes.CommandInvocationStatusSuccess,
	}, nil)
	a := newPauseIoAttack(api)
	state := EbsPauseIoAttackState{Account: "42", Region: "us-east-1", VolumeId: "vol-0123456789abcdef0", InstanceId: "i-deadbeef", AwsDeviceName: "/dev/sdf"}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	assert.Equal(t, "cmd-1", state.StartCommandId)
	api.AssertExpectations(t)
}

func TestPauseIoStartSurfacesSsmFailure(t *testing.T) {
	api := new(ebsPauseIoApiMock)
	api.On("SendCommand", mock.Anything, mock.Anything).Return(&ssm.SendCommandOutput{Command: &ssmtypes.Command{CommandId: aws.String("cmd-x")}}, nil)
	api.On("GetCommandInvocation", mock.Anything, mock.Anything).Return(&ssm.GetCommandInvocationOutput{
		Status:               ssmtypes.CommandInvocationStatusFailed,
		StandardErrorContent: aws.String("could not resolve block device for vol-..."),
	}, nil)
	a := newPauseIoAttack(api)
	state := EbsPauseIoAttackState{InstanceId: "i-deadbeef", VolumeId: "vol-0123456789abcdef0"}
	_, err := a.Start(context.Background(), &state)
	require.Error(t, err)
}

func TestPauseIoStopSendsUnfreezeCommand(t *testing.T) {
	api := new(ebsPauseIoApiMock)
	api.On("SendCommand", mock.Anything, mock.MatchedBy(func(in *ssm.SendCommandInput) bool {
		joined := joinCommands(in.Parameters["commands"])
		require.Contains(t, joined, "fsfreeze --unfreeze")
		return true
	})).Return(&ssm.SendCommandOutput{Command: &ssmtypes.Command{CommandId: aws.String("cmd-stop")}}, nil)
	api.On("GetCommandInvocation", mock.Anything, mock.Anything).Return(&ssm.GetCommandInvocationOutput{
		Status: ssmtypes.CommandInvocationStatusSuccess,
	}, nil)
	a := newPauseIoAttack(api)
	state := EbsPauseIoAttackState{InstanceId: "i-deadbeef", VolumeId: "vol-0123456789abcdef0", StartCommandId: "cmd-1"}
	_, err := a.Stop(context.Background(), &state)
	require.NoError(t, err)
	api.AssertExpectations(t)
}

func TestPauseIoStopReportsManualCleanupHintOnSendError(t *testing.T) {
	api := new(ebsPauseIoApiMock)
	api.On("SendCommand", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	a := newPauseIoAttack(api)
	state := EbsPauseIoAttackState{InstanceId: "i-deadbeef", VolumeId: "vol-0123456789abcdef0"}
	_, err := a.Stop(context.Background(), &state)
	require.Error(t, err)
	// User-facing error must include enough info to recover manually.
	assert.Contains(t, err.Error(), "i-deadbeef")
	assert.Contains(t, err.Error(), "fsfreeze --unfreeze")
}

// joinCommands flattens a []string into one big string for substring assertions in tests.
func joinCommands(in []string) string {
	out := ""
	for _, s := range in {
		out += s + "\n"
	}
	return out
}
