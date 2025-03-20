// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extecs

import (
	"context"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type ecsClientMock struct {
	mock.Mock
}

func (m *ecsClientMock) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.ListTasksOutput), args.Error(1)
}

func (m *ecsClientMock) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.DescribeTasksOutput), args.Error(1)
}

func (m *ecsClientMock) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.ListClustersOutput), args.Error(1)
}

type taskDiscoveryEc2UtilMock struct {
	mock.Mock
}

func (m *taskDiscoveryEc2UtilMock) GetZone(awsAccountNumber string, awsZone string, region string) *ec2types.AvailabilityZone {
	args := m.Called(awsAccountNumber, awsZone, region)
	return args.Get(0).(*ec2types.AvailabilityZone)
}
func (m *taskDiscoveryEc2UtilMock) GetVpcName(awsAccountNumber string, region string, vpcId string) string {
	args := m.Called(awsAccountNumber, region, vpcId)
	return args.Get(0).(string)
}

var taskArn = "arn:aws:ecs:eu-central-1:42:task/sandbox-demo-ecs-fargate/15ac9bc28dce4a6fb757580ac87eb854"
var clusterArn = "arn:aws:ecs:eu-central-1:42:cluster/sandbox-demo-ecs-fargate"
var task = types.Task{
	TaskArn:          extutil.Ptr(taskArn),
	AvailabilityZone: extutil.Ptr("us-east-1b"),
	ClusterArn:       extutil.Ptr(clusterArn),
	LastStatus:       extutil.Ptr("RUNNING"),
	LaunchType:       types.LaunchTypeFargate,
	Tags: []types.Tag{
		{Key: extutil.Ptr("aws:ecs:clusterName"), Value: extutil.Ptr("sandbox-demo-ecs-fargate")},
		{Key: extutil.Ptr("aws:ecs:serviceName"), Value: extutil.Ptr("ecs-demo-gateway-service")},
		{Key: extutil.Ptr("test"), Value: extutil.Ptr("123")},
	},
	Containers: []types.Container{
		{
			Image: extutil.Ptr("public.ecr.aws/amazon-ssm-agent/amazon-ssm-agent:latest"),
		},
	},
}
var taskArn2 = "arn:aws:ecs:eu-central-1:42:task/sandbox-demo-ecs-fargate/15ac9bc28dce4a6fb757580ac87eb855"
var taskStopped = types.Task{
	TaskArn:          extutil.Ptr(taskArn2),
	AvailabilityZone: extutil.Ptr("us-east-1b"),
	ClusterArn:       extutil.Ptr(clusterArn),
	LastStatus:       extutil.Ptr("STOPPED"),
	LaunchType:       types.LaunchTypeFargate,
	Tags: []types.Tag{
		{Key: extutil.Ptr("aws:ecs:clusterName"), Value: extutil.Ptr("sandbox-demo-ecs-fargate")},
		{Key: extutil.Ptr("aws:ecs:serviceName"), Value: extutil.Ptr("ecs-demo-gateway-service")},
		{Key: extutil.Ptr("test"), Value: extutil.Ptr("123")},
	},
}

func TestGetAllEcsTasks(t *testing.T) {
	// Given
	mockedApi, mockedZoneUtil := mockApisTaskDiscovery()

	// When
	targets, err := GetAllEcsTasks(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, ecsTaskTargetId, target.TargetType)
	assert.Equal(t, taskArn, target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"us-east-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"us-east-1b-id"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"123"}, target.Attributes["aws-ecs.task.label.test"])
	assert.Equal(t, []string{clusterArn}, target.Attributes["aws-ecs.cluster.arn"])
	assert.Equal(t, []string{"ecs-demo-gateway-service"}, target.Attributes["aws-ecs.service.name"])
	assert.Equal(t, []string{"sandbox-demo-ecs-fargate"}, target.Attributes["aws-ecs.cluster.name"])
	assert.Equal(t, []string{"FARGATE"}, target.Attributes["aws-ecs.task.launch-type"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws-ecs.task.amazon-ssm-agent"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
}

func TestGetAllEcsTasksShouldApplyTagFilters(t *testing.T) {
	// Given
	mockedApi, mockedZoneUtil := mockApisTaskDiscovery()

	// When
	targets, err := GetAllEcsTasks(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{
				Key:    "test",
				Values: []string{"123"},
			},
		},
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	// When
	targets, err = GetAllEcsTasks(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{
				Key:    "test",
				Values: []string{"xxx"},
			},
		},
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, len(targets))
}

func mockApisTaskDiscovery() (*ecsClientMock, *taskDiscoveryEc2UtilMock) {
	mockedApi := new(ecsClientMock)
	mockedApi.On("ListClusters", mock.Anything, mock.Anything).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{clusterArn},
	}, nil)
	mockedApi.On("ListTasks", mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn, taskArn2},
	}, nil)
	mockedApi.On("DescribeTasks", mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{task, taskStopped},
	}, nil)

	mockedZoneUtil := new(taskDiscoveryEc2UtilMock)
	mockedZone := ec2types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1b"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)
	return mockedApi, mockedZoneUtil
}
