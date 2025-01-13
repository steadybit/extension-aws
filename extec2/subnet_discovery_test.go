// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type subnetDiscoveryApiMock struct {
	mock.Mock
}

func (m *subnetDiscoveryApiMock) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeSubnetsOutput), args.Error(1)
}

func TestGetAllSubnets(t *testing.T) {
	// Given
	mockedApi := new(subnetDiscoveryApiMock)
	mockedReturnValue := ec2.DescribeSubnetsOutput{
		Subnets: []types.Subnet{
			{
				SubnetId:  extutil.Ptr("subnet-0ef9adc9fbd3b19c5"),
				CidrBlock: extutil.Ptr("10.10.0.0/21"),
				VpcId:     extutil.Ptr("vpc-123"),
				Tags: []types.Tag{
					{Key: extutil.Ptr("Name"), Value: extutil.Ptr("dev-demo-ngroup2")},
					{Key: extutil.Ptr("SpecialTag"), Value: extutil.Ptr("Great Thing")},
				},
				AvailabilityZone:   extutil.Ptr("eu-central-1b"),
				AvailabilityZoneId: extutil.Ptr("euc1-az3"),
			},
		},
	}
	mockedApi.On("DescribeSubnets", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	mockedZoneUtil := new(ec2UtilMock)
	mockedZoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")
	// When
	targets, err := GetAllSubnets(context.Background(), mockedApi, mockedZoneUtil, "42", "eu-central-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, subnetTargetType, target.TargetType)
	assert.Equal(t, "subnet-0ef9adc9fbd3b19c5 / dev-demo-ngroup2", target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"eu-central-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"eu-central-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"euc1-az3"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"subnet-0ef9adc9fbd3b19c5"}, target.Attributes["aws.ec2.subnet.id"])
	assert.Equal(t, []string{"dev-demo-ngroup2"}, target.Attributes["aws.ec2.subnet.name"])
	assert.Equal(t, []string{"10.10.0.0/21"}, target.Attributes["aws.ec2.subnet.cidr"])
	assert.Equal(t, []string{"Great Thing"}, target.Attributes["aws.ec2.subnet.label.specialtag"])
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}
