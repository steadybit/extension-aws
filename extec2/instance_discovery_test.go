// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type instanceDiscoveryApiMock struct {
	mock.Mock
}

func (m *instanceDiscoveryApiMock) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

var instance = types.Instance{
	InstanceId: new("i-0ef9adc9fbd3b19c5"),
	ImageId:    new("ami-02fc9c535f43bbc91"),
	Placement: &types.Placement{
		AvailabilityZone: new("us-east-1b"),
	},
	SubnetId:         new("subnet-0e3b6b7b1b7b7b7b7"),
	PrivateIpAddress: new("10.3.92.28"),
	PrivateDnsName:   new("ip-10-3-92-28.eu-central-1.compute.internal"),
	VpcId:            new("vpc-003cf5dda88c814c6"),
	State: &types.InstanceState{
		Name: "running",
		Code: new(int32(16)),
	},
	Tags: []types.Tag{
		{Key: new("Name"), Value: new("dev-demo-ngroup2")},
		{Key: new("SpecialTag"), Value: new("Great Thing")},
	},
}

func TestGetAllEc2Instances(t *testing.T) {
	// Given
	mockedApi := new(instanceDiscoveryApiMock)
	mockedReturnValue := ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					instance,
				},
			},
		},
	}
	mockedApi.On("DescribeInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	mockedZoneUtil := new(ec2UtilMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   new("us-east-1b"),
		RegionName: new("us-east-1"),
		ZoneId:     new("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)
	mockedZoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")
	// When
	targets, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, ec2TargetType, target.TargetType)
	assert.Equal(t, "i-0ef9adc9fbd3b19c5 / dev-demo-ngroup2", target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"ami-02fc9c535f43bbc91"}, target.Attributes["aws-ec2.image"])
	assert.Equal(t, []string{"us-east-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"us-east-1b-id"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"10.3.92.28"}, target.Attributes["aws-ec2.ipv4.private"])
	assert.Equal(t, []string{"subnet-0e3b6b7b1b7b7b7b7"}, target.Attributes["aws.ec2.subnet.id"])
	assert.Equal(t, []string{"i-0ef9adc9fbd3b19c5"}, target.Attributes["aws-ec2.instance.id"])
	assert.Equal(t, []string{"ip-10-3-92-28.eu-central-1.compute.internal"}, target.Attributes["aws-ec2.hostname.internal"])
	assert.Equal(t, []string{"arn:aws:ec2:us-east-1:42:instance/i-0ef9adc9fbd3b19c5"}, target.Attributes["aws-ec2.arn"])
	assert.Equal(t, []string{"vpc-003cf5dda88c814c6"}, target.Attributes["aws-ec2.vpc"])
	assert.Equal(t, []string{"Great Thing"}, target.Attributes["aws-ec2.label.specialtag"])
	assert.Equal(t, []string{"running"}, target.Attributes["aws-ec2.state"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllEc2InstancesWithFilteredAttributes(t *testing.T) {
	// Given
	// set env var to filter out all attributes starting with "aws-ec2"
	config.Config.DiscoveryAttributesExcludesEc2 = []string{"aws-ec2.label.*", "aws-ec2.image"}
	mockedApi := new(instanceDiscoveryApiMock)
	mockedReturnValue := ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					instance,
				},
			},
		},
	}
	mockedApi.On("DescribeInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	mockedZoneUtil := new(ec2UtilMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   new("us-east-1b"),
		RegionName: new("us-east-1"),
		ZoneId:     new("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)
	mockedZoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")

	// When
	targets, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, ec2TargetType, target.TargetType)
	assert.Equal(t, "i-0ef9adc9fbd3b19c5 / dev-demo-ngroup2", target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"us-east-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"us-east-1b-id"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"10.3.92.28"}, target.Attributes["aws-ec2.ipv4.private"])
	assert.Equal(t, []string{"i-0ef9adc9fbd3b19c5"}, target.Attributes["aws-ec2.instance.id"])
	assert.Equal(t, []string{"ip-10-3-92-28.eu-central-1.compute.internal"}, target.Attributes["aws-ec2.hostname.internal"])
	assert.Equal(t, []string{"arn:aws:ec2:us-east-1:42:instance/i-0ef9adc9fbd3b19c5"}, target.Attributes["aws-ec2.arn"])
	assert.Equal(t, []string{"vpc-003cf5dda88c814c6"}, target.Attributes["aws-ec2.vpc"])
	assert.Equal(t, []string{"vpc-003cf5dda88c814c6"}, target.Attributes["aws.vpc.id"])
	assert.Equal(t, []string{"vpc-123-name"}, target.Attributes["aws.vpc.name"])
	assert.Equal(t, []string{"running"}, target.Attributes["aws-ec2.state"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
	assert.NotContains(t, target.Attributes, "aws-ec2.label.specialtag")
	assert.NotContains(t, target.Attributes, "aws-ec2.image")
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllEc2InstancesShouldApplyTagFilters(t *testing.T) {
	// Given
	mockedApi := new(instanceDiscoveryApiMock)
	mockedReturnValue := ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					instance,
				},
			},
		},
	}
	mockedApi.On("DescribeInstances", mock.Anything, mock.MatchedBy(func(input *ec2.DescribeInstancesInput) bool {
		return aws.ToString(input.Filters[0].Name) == "tag:application" && input.Filters[0].Values[0] == "demo"
	})).Return(&mockedReturnValue, nil)

	mockedZoneUtil := new(ec2UtilMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   new("us-east-1b"),
		RegionName: new("us-east-1"),
		ZoneId:     new("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)
	mockedZoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")

	// When
	targets, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []config.TagFilter{
			{
				Key:    "application",
				Values: []string{"demo"},
			},
		},
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))
	mockedApi.AssertExpectations(t)
}

func TestNameNotSet(t *testing.T) {
	// Given
	mockedApi := new(instanceDiscoveryApiMock)
	mockedReturnValue := ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: new("i-0ef9adc9fbd3b19c5"),
						Placement: &types.Placement{
							AvailabilityZone: new("us-east-1b"),
						},
					},
				},
			},
		},
	}
	mockedApi.On("DescribeInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	mockedZoneUtil := new(ec2UtilMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   new("us-east-1b"),
		RegionName: new("us-east-1"),
		ZoneId:     new("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)
	mockedZoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")

	// When
	targets, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, "i-0ef9adc9fbd3b19c5", target.Label)
}

func TestGetAllEc2InstancesError(t *testing.T) {
	// Given
	mockedApi := new(instanceDiscoveryApiMock)

	mockedApi.On("DescribeInstances", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	mockedZoneUtil := new(ec2UtilMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   new("us-east-1b"),
		RegionName: new("us-east-1"),
		ZoneId:     new("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)
	mockedZoneUtil.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")

	// When
	_, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.EqualError(t, err, "expected")
}
