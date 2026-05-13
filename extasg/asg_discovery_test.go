// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extasg

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func (m *asgApiMock) SuspendProcesses(ctx context.Context, params *autoscaling.SuspendProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.SuspendProcessesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*autoscaling.SuspendProcessesOutput), args.Error(1)
}

func (m *asgApiMock) ResumeProcesses(ctx context.Context, params *autoscaling.ResumeProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.ResumeProcessesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*autoscaling.ResumeProcessesOutput), args.Error(1)
}

func TestGetAllAsgs(t *testing.T) {
	// Given
	mockedApi := new(asgApiMock)
	mockedApi.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []types.AutoScalingGroup{
			{
				AutoScalingGroupName:   aws.String("web-asg"),
				AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:42:autoScalingGroup:abc:autoScalingGroupName/web-asg"),
				AvailabilityZones:      []string{"us-east-1a", "us-east-1b"},
				VPCZoneIdentifier:      aws.String("subnet-aaa,subnet-bbb"),
				MinSize:                aws.Int32(1),
				MaxSize:                aws.Int32(5),
				DesiredCapacity:        aws.Int32(2),
				DefaultCooldown:        aws.Int32(300),
				HealthCheckType:        aws.String("ELB"),
				HealthCheckGracePeriod: aws.Int32(60),
				CapacityRebalance:      aws.Bool(true),
				MaxInstanceLifetime:    aws.Int32(2592000),
				LaunchTemplate: &types.LaunchTemplateSpecification{
					LaunchTemplateId: aws.String("lt-123"),
					Version:          aws.String("$Latest"),
				},
				SuspendedProcesses: []types.SuspendedProcess{
					{ProcessName: aws.String("AZRebalance"), SuspensionReason: aws.String("manual")},
				},
				TargetGroupARNs:                  []string{"arn:tg-1"},
				TerminationPolicies:              []string{"OldestInstance", "Default"},
				NewInstancesProtectedFromScaleIn: aws.Bool(false),
				Tags: []types.TagDescription{
					{Key: aws.String("application"), Value: aws.String("Demo")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
	}, nil)

	// When
	targets, err := getAllAsgs(context.Background(), mockedApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    aws.String("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{Key: "application", Values: []string{"Demo"}},
		},
	})

	// Then
	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	target := targets[0]
	assert.Equal(t, asgTargetId, target.TargetType)
	assert.Equal(t, "web-asg", target.Label)
	assert.Equal(t, "arn:aws:autoscaling:us-east-1:42:autoScalingGroup:abc:autoScalingGroupName/web-asg", target.Id)

	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"web-asg"}, target.Attributes["aws.asg.name"])
	assert.Equal(t, []string{"us-east-1a", "us-east-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"us-east-1a", "us-east-1b"}, target.Attributes["aws.asg.availability-zones"])
	assert.Equal(t, []string{"subnet-aaa", "subnet-bbb"}, target.Attributes["aws.asg.subnets"])
	assert.Equal(t, []string{"1"}, target.Attributes["aws.asg.min-size"])
	assert.Equal(t, []string{"5"}, target.Attributes["aws.asg.max-size"])
	assert.Equal(t, []string{"300"}, target.Attributes["aws.asg.default-cooldown"])
	assert.Equal(t, []string{"ELB"}, target.Attributes["aws.asg.health-check-type"])
	assert.Equal(t, []string{"60"}, target.Attributes["aws.asg.health-check-grace-period"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws.asg.capacity-rebalance"])
	assert.Equal(t, []string{"false"}, target.Attributes["aws.asg.new-instances-protected-from-scale-in"])
	assert.Equal(t, []string{"2592000"}, target.Attributes["aws.asg.max-instance-lifetime"])
	assert.Equal(t, []string{"AZRebalance"}, target.Attributes["aws.asg.suspended-processes"])
	assert.Equal(t, []string{"false"}, target.Attributes["aws.asg.mixed-instances-policy.enabled"])
	assert.Equal(t, []string{"lt-123"}, target.Attributes["aws.asg.launch-template.id"])
	assert.Equal(t, []string{"$Latest"}, target.Attributes["aws.asg.launch-template.version"])
	assert.Equal(t, []string{"latest"}, target.Attributes["aws.asg.launch-template.version-mode"])
	assert.Equal(t, []string{"arn:tg-1"}, target.Attributes["aws.asg.target-group-arns"])
	assert.Equal(t, []string{"OldestInstance", "Default"}, target.Attributes["aws.asg.termination-policies"])
	assert.Equal(t, []string{"Demo"}, target.Attributes["aws.asg.label.application"])
	assert.Equal(t, []string{"prod"}, target.Attributes["aws.asg.label.environment"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])

	// DesiredCapacity must NOT be exposed (volatile under autoscaling)
	_, hasDesired := target.Attributes["aws.asg.desired-capacity"]
	assert.False(t, hasDesired, "desired-capacity must not be exposed (volatile)")
}

func TestGetAllAsgsTagFilterMismatch(t *testing.T) {
	mockedApi := new(asgApiMock)
	mockedApi.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []types.AutoScalingGroup{
			{
				AutoScalingGroupName: aws.String("other-asg"),
				AutoScalingGroupARN:  aws.String("arn:other"),
				MinSize:              aws.Int32(0),
				MaxSize:              aws.Int32(1),
				Tags: []types.TagDescription{
					{Key: aws.String("application"), Value: aws.String("Other")},
				},
			},
		},
	}, nil)

	targets, err := getAllAsgs(context.Background(), mockedApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		TagFilters: []extConfig.TagFilter{
			{Key: "application", Values: []string{"Demo"}},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(targets))
}

func TestGetAllAsgsLaunchTemplateVersionModes(t *testing.T) {
	cases := []struct {
		version string
		mode    string
	}{
		{"$Latest", "latest"},
		{"$Default", "default"},
		{"3", "pinned"},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			mockedApi := new(asgApiMock)
			mockedApi.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []types.AutoScalingGroup{
					{
						AutoScalingGroupName: aws.String("a"),
						AutoScalingGroupARN:  aws.String("arn:a"),
						MinSize:              aws.Int32(0),
						MaxSize:              aws.Int32(1),
						LaunchTemplate: &types.LaunchTemplateSpecification{
							LaunchTemplateId: aws.String("lt-x"),
							Version:          aws.String(tc.version),
						},
					},
				},
			}, nil)
			targets, err := getAllAsgs(context.Background(), mockedApi, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
			assert.NoError(t, err)
			assert.Equal(t, []string{tc.mode}, targets[0].Attributes["aws.asg.launch-template.version-mode"])
		})
	}
}

func TestGetAllAsgsMixedInstancesPolicy(t *testing.T) {
	mockedApi := new(asgApiMock)
	mockedApi.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []types.AutoScalingGroup{
			{
				AutoScalingGroupName: aws.String("mixed"),
				AutoScalingGroupARN:  aws.String("arn:mixed"),
				MinSize:              aws.Int32(0),
				MaxSize:              aws.Int32(1),
				MixedInstancesPolicy: &types.MixedInstancesPolicy{
					LaunchTemplate: &types.LaunchTemplate{
						LaunchTemplateSpecification: &types.LaunchTemplateSpecification{
							LaunchTemplateId: aws.String("lt-mixed"),
							Version:          aws.String("$Default"),
						},
					},
				},
			},
		},
	}, nil)
	targets, err := getAllAsgs(context.Background(), mockedApi, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"true"}, targets[0].Attributes["aws.asg.mixed-instances-policy.enabled"])
	assert.Equal(t, []string{"lt-mixed"}, targets[0].Attributes["aws.asg.launch-template.id"])
	assert.Equal(t, []string{"default"}, targets[0].Attributes["aws.asg.launch-template.version-mode"])
}

func TestGetAllAsgsError(t *testing.T) {
	mockedApi := new(asgApiMock)
	mockedApi.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	_, err := getAllAsgs(context.Background(), mockedApi, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.EqualError(t, err, "expected")
}
