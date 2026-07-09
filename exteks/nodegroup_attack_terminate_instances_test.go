// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteks

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type asgApiMock struct {
	mock.Mock
}

func (m *asgApiMock) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*autoscaling.DescribeAutoScalingGroupsOutput), args.Error(1)
}

type ec2ApiMock struct {
	mock.Mock
}

func (m *ec2ApiMock) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.TerminateInstancesOutput), args.Error(1)
}

func newTerminateRequest(pct int) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{"percentage": pct},
		Target: new(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":                      {"42"},
				"aws.region":                       {"us-east-1"},
				"aws.eks.cluster.name":             {"prod"},
				"aws.eks.nodegroup.name":           {"workers"},
				"extension-aws.discovered-by-role": {"arn:role"},
			},
		}),
	})
}

// identityRng returns the identity permutation, making instance selection deterministic in tests.
func identityRng(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func mockNodegroupWithAsgs(eksApi *eksApiMock, asgNames ...string) {
	asgRefs := make([]types.AutoScalingGroup, 0, len(asgNames))
	for _, n := range asgNames {
		asgRefs = append(asgRefs, types.AutoScalingGroup{Name: aws.String(n)})
	}
	eksApi.On("DescribeNodegroup", mock.Anything, mock.Anything).Return(&eks.DescribeNodegroupOutput{
		Nodegroup: &types.Nodegroup{
			NodegroupName: aws.String("workers"),
			ClusterName:   aws.String("prod"),
			Resources:     &types.NodegroupResources{AutoScalingGroups: asgRefs},
		},
	}, nil)
}

func mockAsgWithInstances(asgApi *asgApiMock, instances ...autoscalingtypes.Instance) {
	asgApi.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []autoscalingtypes.AutoScalingGroup{
			{Instances: instances},
		},
	}, nil)
}

func newAttack(eksApi *eksApiMock, asgApi *asgApiMock, ec2Api *ec2ApiMock) eksNodegroupTerminateInstancesAttack {
	return eksNodegroupTerminateInstancesAttack{
		eksClientProvider: func(account string, region string, role *string) (EksApi, error) { return eksApi, nil },
		asgClientProvider: func(account string, region string, role *string) (EksAsgApi, error) { return asgApi, nil },
		ec2ClientProvider: func(account string, region string, role *string) (EksEc2Api, error) { return ec2Api, nil },
		rng:               identityRng,
	}
}

func TestPrepareSamplesPercentageOfInServiceInstances(t *testing.T) {
	eksApi := new(eksApiMock)
	asgApi := new(asgApiMock)
	mockNodegroupWithAsgs(eksApi, "asg-1")
	mockAsgWithInstances(asgApi,
		autoscalingtypes.Instance{InstanceId: aws.String("i-1"), LifecycleState: autoscalingtypes.LifecycleStateInService},
		autoscalingtypes.Instance{InstanceId: aws.String("i-2"), LifecycleState: autoscalingtypes.LifecycleStateInService},
		autoscalingtypes.Instance{InstanceId: aws.String("i-3"), LifecycleState: autoscalingtypes.LifecycleStateInService},
		autoscalingtypes.Instance{InstanceId: aws.String("i-4"), LifecycleState: autoscalingtypes.LifecycleStateInService},
		autoscalingtypes.Instance{InstanceId: aws.String("i-5"), LifecycleState: autoscalingtypes.LifecycleStateInService},
		autoscalingtypes.Instance{InstanceId: aws.String("i-6"), LifecycleState: autoscalingtypes.LifecycleStateInService},
	)
	attack := newAttack(eksApi, asgApi, new(ec2ApiMock))
	state := attack.NewEmptyState()

	_, err := attack.Prepare(context.Background(), &state, newTerminateRequest(33))
	require.NoError(t, err)
	// ceil(6 * 0.33) = 2
	assert.Equal(t, 2, len(state.InstanceIds))
}

func TestPrepareSkipsNonInServiceInstances(t *testing.T) {
	eksApi := new(eksApiMock)
	asgApi := new(asgApiMock)
	mockNodegroupWithAsgs(eksApi, "asg-1")
	mockAsgWithInstances(asgApi,
		autoscalingtypes.Instance{InstanceId: aws.String("i-pending"), LifecycleState: autoscalingtypes.LifecycleStatePending},
		autoscalingtypes.Instance{InstanceId: aws.String("i-terminating"), LifecycleState: "Terminating"},
		autoscalingtypes.Instance{InstanceId: aws.String("i-1"), LifecycleState: autoscalingtypes.LifecycleStateInService},
		autoscalingtypes.Instance{InstanceId: aws.String("i-2"), LifecycleState: autoscalingtypes.LifecycleStateInService},
	)
	attack := newAttack(eksApi, asgApi, new(ec2ApiMock))
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newTerminateRequest(100))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"i-1", "i-2"}, state.InstanceIds)
}

func TestPrepareAtLeastOneInstanceWhenPercentageWouldRoundToZero(t *testing.T) {
	eksApi := new(eksApiMock)
	asgApi := new(asgApiMock)
	mockNodegroupWithAsgs(eksApi, "asg-1")
	mockAsgWithInstances(asgApi,
		autoscalingtypes.Instance{InstanceId: aws.String("i-1"), LifecycleState: autoscalingtypes.LifecycleStateInService},
	)
	attack := newAttack(eksApi, asgApi, new(ec2ApiMock))
	state := attack.NewEmptyState()
	// 1 instance × 1% would round to 0 without ceil; we always pick at least one.
	_, err := attack.Prepare(context.Background(), &state, newTerminateRequest(1))
	require.NoError(t, err)
	assert.Equal(t, []string{"i-1"}, state.InstanceIds)
}

func TestPrepareRejectsOutOfRangePercentage(t *testing.T) {
	eksApi := new(eksApiMock)
	asgApi := new(asgApiMock)
	attack := newAttack(eksApi, asgApi, new(ec2ApiMock))
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newTerminateRequest(0))
	require.Error(t, err)
	_, err = attack.Prepare(context.Background(), &state, newTerminateRequest(101))
	require.Error(t, err)
}

func TestPrepareErrorsWhenNoAsgUnderlyingNodeGroup(t *testing.T) {
	eksApi := new(eksApiMock)
	eksApi.On("DescribeNodegroup", mock.Anything, mock.Anything).Return(&eks.DescribeNodegroupOutput{
		Nodegroup: &types.Nodegroup{NodegroupName: aws.String("workers"), ClusterName: aws.String("prod")},
	}, nil)
	attack := newAttack(eksApi, new(asgApiMock), new(ec2ApiMock))
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newTerminateRequest(33))
	require.Error(t, err)
}

func TestPrepareErrorsWhenNoInServiceInstances(t *testing.T) {
	eksApi := new(eksApiMock)
	asgApi := new(asgApiMock)
	mockNodegroupWithAsgs(eksApi, "asg-1")
	mockAsgWithInstances(asgApi,
		autoscalingtypes.Instance{InstanceId: aws.String("i-pending"), LifecycleState: autoscalingtypes.LifecycleStatePending},
	)
	attack := newAttack(eksApi, asgApi, new(ec2ApiMock))
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newTerminateRequest(33))
	require.Error(t, err)
}

func TestStartTerminatesSelectedInstances(t *testing.T) {
	ec2Api := new(ec2ApiMock)
	ec2Api.On("TerminateInstances", mock.Anything, mock.MatchedBy(func(p *ec2.TerminateInstancesInput) bool {
		require.ElementsMatch(t, []string{"i-1", "i-2"}, p.InstanceIds)
		return true
	})).Return(&ec2.TerminateInstancesOutput{}, nil)
	attack := newAttack(new(eksApiMock), new(asgApiMock), ec2Api)
	state := EksNodegroupTerminateInstancesAttackState{
		ClusterName: "prod", NodegroupName: "workers", Account: "42", Region: "us-east-1",
		InstanceIds: []string{"i-1", "i-2"},
	}
	_, err := attack.Start(context.Background(), &state)
	assert.NoError(t, err)
	ec2Api.AssertExpectations(t)
}

func TestStartErrorsWhenNoInstancesSelected(t *testing.T) {
	attack := newAttack(new(eksApiMock), new(asgApiMock), new(ec2ApiMock))
	state := EksNodegroupTerminateInstancesAttackState{ClusterName: "prod", NodegroupName: "workers"}
	_, err := attack.Start(context.Background(), &state)
	require.Error(t, err)
}

func TestStartForwardsTerminateError(t *testing.T) {
	ec2Api := new(ec2ApiMock)
	ec2Api.On("TerminateInstances", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	attack := newAttack(new(eksApiMock), new(asgApiMock), ec2Api)
	state := EksNodegroupTerminateInstancesAttackState{InstanceIds: []string{"i-1"}}
	_, err := attack.Start(context.Background(), &state)
	require.Error(t, err)
}
