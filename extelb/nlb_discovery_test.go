// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extelb

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type nlbDiscoveryApiMock struct {
	mock.Mock
}

func (m *nlbDiscoveryApiMock) DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticloadbalancingv2.DescribeLoadBalancersOutput), args.Error(1)
}

func (m *nlbDiscoveryApiMock) DescribeTags(ctx context.Context, params *elasticloadbalancingv2.DescribeTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTagsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticloadbalancingv2.DescribeTagsOutput), args.Error(1)
}

func (m *nlbDiscoveryApiMock) DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticloadbalancingv2.DescribeListenersOutput), args.Error(1)
}

func (m *nlbDiscoveryApiMock) DescribeLoadBalancerAttributes(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancerAttributesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput), args.Error(1)
}

func TestGetNlbsFiltersOutAlb(t *testing.T) {
	api := new(nlbDiscoveryApiMock)
	albArnLocal := "arn:aws:elasticloadbalancing:us-east-1:42:loadbalancer/app/my-alb/1"
	nlbArnLocal := "arn:aws:elasticloadbalancing:us-east-1:42:loadbalancer/net/my-nlb/2"
	api.On("DescribeLoadBalancers", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeLoadBalancersOutput{
		LoadBalancers: []types.LoadBalancer{
			{
				LoadBalancerArn:  &albArnLocal,
				LoadBalancerName: new("my-alb"),
				Type:             types.LoadBalancerTypeEnumApplication,
			},
			{
				LoadBalancerArn:  &nlbArnLocal,
				LoadBalancerName: new("my-nlb"),
				DNSName:          new("my-nlb-1234.elb.us-east-1.amazonaws.com"),
				Type:             types.LoadBalancerTypeEnumNetwork,
				Scheme:           types.LoadBalancerSchemeEnumInternetFacing,
				IpAddressType:    types.IpAddressTypeIpv4,
				VpcId:            new("vpc-1"),
				AvailabilityZones: []types.AvailabilityZone{
					{ZoneName: new("us-east-1a"), SubnetId: new("subnet-a")},
					{ZoneName: new("us-east-1b"), SubnetId: new("subnet-b")},
				},
			},
		},
	}, nil)
	api.On("DescribeTags", mock.Anything, mock.MatchedBy(func(p *elasticloadbalancingv2.DescribeTagsInput) bool {
		// Should ONLY include the NLB ARN (ALB filtered out)
		return len(p.ResourceArns) == 1 && p.ResourceArns[0] == nlbArnLocal
	})).Return(&elasticloadbalancingv2.DescribeTagsOutput{
		TagDescriptions: []types.TagDescription{
			{ResourceArn: &nlbArnLocal, Tags: []types.Tag{
				{Key: new("application"), Value: new("Demo")},
			}},
		},
	}, nil)
	api.On("DescribeListeners", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeListenersOutput{
		Listeners: []types.Listener{
			{Port: new(int32(443)), Protocol: types.ProtocolEnumTls},
			{Port: new(int32(80)), Protocol: types.ProtocolEnumTcp},
		},
	}, nil)
	api.On("DescribeLoadBalancerAttributes", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput{
		Attributes: []types.LoadBalancerAttribute{
			{Key: new("load_balancing.cross_zone.enabled"), Value: new("false")},
			{Key: new("deletion_protection.enabled"), Value: new("true")},
			{Key: new("access_logs.s3.enabled"), Value: new("false")},
		},
	}, nil)

	zoneUtil := new(albDiscoveryEc2UtilMock)
	zoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&ec2types.AvailabilityZone{ZoneId: new("us-east-1a-id")})
	zoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-1-name")

	targets, err := getNlbs(context.Background(), api, zoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	tgt := targets[0]
	assert.Equal(t, nlbTargetId, tgt.TargetType)
	assert.Equal(t, "my-nlb", tgt.Label)
	assert.Equal(t, []string{"my-nlb"}, tgt.Attributes["aws-elb.nlb.name"])
	assert.Equal(t, []string{"internet-facing"}, tgt.Attributes["aws-elb.nlb.scheme"])
	assert.Equal(t, []string{"ipv4"}, tgt.Attributes["aws-elb.nlb.ip-address-type"])
	assert.ElementsMatch(t, []string{"443", "80"}, tgt.Attributes["aws-elb.nlb.listener.port"])
	assert.ElementsMatch(t, []string{"TLS", "TCP"}, tgt.Attributes["aws-elb.nlb.listener.protocol"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws-elb.nlb.cross-zone-load-balancing"])
	assert.Equal(t, []string{"true"}, tgt.Attributes["aws-elb.nlb.deletion-protection"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws-elb.nlb.access-logs.enabled"])
	assert.Equal(t, []string{"subnet-a", "subnet-b"}, tgt.Attributes["aws-elb.nlb.subnets"])
	assert.Equal(t, []string{"vpc-1"}, tgt.Attributes["aws.vpc.id"])
	assert.Equal(t, []string{"Demo"}, tgt.Attributes["aws-elb.nlb.label.application"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, tgt.Attributes["extension-aws.discovered-by-role"])
}

func TestGetNlbsEmptyWhenOnlyAlbs(t *testing.T) {
	api := new(nlbDiscoveryApiMock)
	api.On("DescribeLoadBalancers", mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeLoadBalancersOutput{
		LoadBalancers: []types.LoadBalancer{
			{LoadBalancerArn: new("arn:alb"), LoadBalancerName: new("only-alb"), Type: types.LoadBalancerTypeEnumApplication},
		},
	}, nil)
	zoneUtil := new(albDiscoveryEc2UtilMock)
	targets, err := getNlbs(context.Background(), api, zoneUtil, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(targets))
	api.AssertNotCalled(t, "DescribeTags", mock.Anything, mock.Anything)
	api.AssertNotCalled(t, "DescribeLoadBalancerAttributes", mock.Anything, mock.Anything)
}
