// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type ecsServiceDiscoveryApiMock struct {
	mock.Mock
}

func (m *ecsServiceDiscoveryApiMock) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.ListClustersOutput), args.Error(1)
}

func (m *ecsServiceDiscoveryApiMock) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.ListServicesOutput), args.Error(1)
}

func (m *ecsServiceDiscoveryApiMock) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

var serviceClusterName = "sandbox-demo-ecs-fargate"
var serviceClusterArn = "arn:aws:ecs:eu-central-1:42:cluster/" + serviceClusterName
var serviceArn = "arn:aws:ecs:eu-central-1:503171660203:service/sandbox-demo-ecs-fargate/ecs-demo-gateway-service"
var serviceName = "ecs-demo-gateway-service"
var service = types.Service{
	ClusterArn:   extutil.Ptr(serviceClusterArn),
	ServiceArn:   extutil.Ptr(serviceArn),
	ServiceName:  extutil.Ptr(serviceName),
	DesiredCount: 3,
	Tags: []types.Tag{
		{Key: extutil.Ptr("test"), Value: extutil.Ptr("123")},
	},
}

func TestGetAllEcsServices(t *testing.T) {
	// Given
	mockedApi := new(ecsServiceDiscoveryApiMock)
	mockedApi.On("ListClusters", mock.Anything, mock.Anything).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{clusterArn},
	}, nil)
	mockedApi.On("ListServices", mock.Anything, mock.Anything).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{serviceArn},
	}, nil)
	mockedApi.On("DescribeServices", mock.Anything, mock.Anything).Return(&ecs.DescribeServicesOutput{
		Services: []types.Service{service},
	}, nil)

	// When
	targets, err := GetAllEcsServices("us-east-1", "42", mockedApi, context.Background())

	// Then
	assert.NoError(t, err)
	assert.Len(t, targets, 1)

	target := targets[0]
	assert.Equal(t, ecsServiceTargetId, target.TargetType)
	assert.Equal(t, serviceArn, target.Id)
	assert.Equal(t, serviceName, target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"123"}, target.Attributes["aws-ecs.service.label.test"])
	assert.Equal(t, []string{serviceClusterArn}, target.Attributes["aws-ecs.cluster.arn"])
	assert.Equal(t, []string{serviceClusterName}, target.Attributes["aws-ecs.cluster.name"])
	assert.Equal(t, []string{serviceArn}, target.Attributes["aws-ecs.service.arn"])
	assert.Equal(t, []string{serviceName}, target.Attributes["aws-ecs.service.name"])
	assert.Equal(t, []string{"3"}, target.Attributes["aws-ecs.service.desired-count"])
}
