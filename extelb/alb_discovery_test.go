// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelb

import (
	"context"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

type albDiscoveryApiMock struct {
	mock.Mock
}

func (m *albDiscoveryApiMock) DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticloadbalancingv2.DescribeLoadBalancersOutput), args.Error(1)
}

func (m *albDiscoveryApiMock) DescribeTags(ctx context.Context, params *elasticloadbalancingv2.DescribeTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTagsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticloadbalancingv2.DescribeTagsOutput), args.Error(1)
}

func (m *albDiscoveryApiMock) DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticloadbalancingv2.DescribeListenersOutput), args.Error(1)
}

type albDiscoveryEc2UtilMock struct {
	mock.Mock
}

func (m *albDiscoveryEc2UtilMock) GetZone(awsAccountNumber string, awsZone string, region string) *ec2types.AvailabilityZone {
	args := m.Called(awsAccountNumber, awsZone, region)
	return args.Get(0).(*ec2types.AvailabilityZone)
}

func (m *albDiscoveryEc2UtilMock) GetVpcName(awsAccountNumber string, region string, vpcId string) string {
	args := m.Called(awsAccountNumber, region, vpcId)
	return args.Get(0).(string)
}

var albArn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-app-balancer/123"
var alb = types.LoadBalancer{
	LoadBalancerArn:  new(albArn),
	DNSName:          new("my-app-balancer-1234567890.us-east-1.elb.amazonaws.com"),
	LoadBalancerName: new("my-app-balancer"),
	Type:             types.LoadBalancerTypeEnumApplication,
	VpcId:            new("vpc-123"),
	AvailabilityZones: []types.AvailabilityZone{
		{
			ZoneName: new("us-east-1a"),
		},
		{
			ZoneName: new("us-east-1b"),
		},
	},
}

var nlbArn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/my-net-balancer/123"
var nlb = types.LoadBalancer{
	LoadBalancerArn:  new(nlbArn),
	LoadBalancerName: new("my-net-balancer"),
	Type:             types.LoadBalancerTypeEnumNetwork,
	AvailabilityZones: []types.AvailabilityZone{
		{
			ZoneName: new("us-east-1b"),
		},
		{
			ZoneName: new("us-east-1c"),
		},
	},
}

func TestGetAllAlbTargets(t *testing.T) {
	// Given
	mockedApi := new(albDiscoveryApiMock)
	mockedApi.On("DescribeLoadBalancers", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeLoadBalancersOutput{
		LoadBalancers: []types.LoadBalancer{alb, nlb},
	}, nil)
	mockedApi.On("DescribeTags", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.DescribeTagsInput) bool {
		require.Equal(t, albArn, params.ResourceArns[0])
		require.Equal(t, nlbArn, params.ResourceArns[1])
		return true
	})).Return(&elasticloadbalancingv2.DescribeTagsOutput{
		TagDescriptions: []types.TagDescription{
			{
				ResourceArn: new(albArn),
				Tags: []types.Tag{
					{
						Key:   new("elbv2.k8s.aws/cluster"),
						Value: new("test-cluster"),
					},
					{
						Key:   new("service.k8s.aws/resource"),
						Value: new("LoadBalancer"),
					},
					{
						Key:   new("service.k8s.aws/stack"),
						Value: new("steadybit-demo/gateway"),
					},
				},
			},
			{
				ResourceArn: new(nlbArn),
				Tags:        []types.Tag{},
			},
		},
	}, nil)
	mockedApi.On("DescribeListeners", mock.Anything, mock.MatchedBy(func(params *elasticloadbalancingv2.DescribeListenersInput) bool {
		require.Equal(t, albArn, *params.LoadBalancerArn)
		return true
	})).Return(&elasticloadbalancingv2.DescribeListenersOutput{
		Listeners: []types.Listener{
			{
				Port: new(int32(80)),
			},
			{
				Port: new(int32(443)),
			},
		},
	}, nil)

	mockedZoneUtil := new(albDiscoveryEc2UtilMock)
	mockedZone1a := ec2types.AvailabilityZone{
		ZoneName:   new("us-east-1a"),
		RegionName: new("us-east-1"),
		ZoneId:     new("us-east-1a-id"),
	}
	mockedZone1b := ec2types.AvailabilityZone{
		ZoneName:   new("us-east-1b"),
		RegionName: new("us-east-1"),
		ZoneId:     new("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.MatchedBy(func(params string) bool {
		return params == "us-east-1a"
	})).Return(&mockedZone1a)
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.MatchedBy(func(params string) bool {
		return params == "us-east-1b"
	})).Return(&mockedZone1b)
	mockedZoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")

	// When
	targets, err := GetAlbs(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{
				Key:    "service.k8s.aws/stack",
				Values: []string{"steadybit-demo/gateway"},
			},
		},
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, albArn, target.Id)
	assert.Equal(t, albTargetId, target.TargetType)
	assert.Equal(t, "my-app-balancer", target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"us-east-1a", "us-east-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"us-east-1a-id", "us-east-1b-id"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"vpc-123"}, target.Attributes["aws.vpc.id"])
	assert.Equal(t, []string{"vpc-123-name"}, target.Attributes["aws.vpc.name"])
	assert.Equal(t, []string{"my-app-balancer"}, target.Attributes["aws-elb.alb.name"])
	assert.Equal(t, []string{"my-app-balancer-1234567890.us-east-1.elb.amazonaws.com"}, target.Attributes["aws-elb.alb.dns"])
	assert.Equal(t, []string{albArn}, target.Attributes["aws-elb.alb.arn"])
	assert.Equal(t, []string{"80", "443"}, target.Attributes["aws-elb.alb.listener.port"])
	assert.Equal(t, []string{"LoadBalancer"}, target.Attributes["aws-elb.alb.label.service.k8s.aws/resource"])
	assert.Equal(t, []string{"test-cluster"}, target.Attributes["k8s.cluster-name"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
	mockedApi.AssertExpectations(t)
}
