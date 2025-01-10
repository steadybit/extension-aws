// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type ec2ClientMock struct {
	mock.Mock
}

func (m *ec2ClientMock) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

var instance = types.Instance{
	InstanceId: extutil.Ptr("i-0ef9adc9fbd3b19c5"),
	ImageId:    extutil.Ptr("ami-02fc9c535f43bbc91"),
	Placement: &types.Placement{
		AvailabilityZone: extutil.Ptr("us-east-1b"),
	},
	PrivateIpAddress: extutil.Ptr("10.3.92.28"),
	PrivateDnsName:   extutil.Ptr("ip-10-3-92-28.eu-central-1.compute.internal"),
	VpcId:            extutil.Ptr("vpc-003cf5dda88c814c6"),
	State: &types.InstanceState{
		Name: "running",
		Code: extutil.Ptr(int32(16)),
	},
	Tags: []types.Tag{
		{Key: extutil.Ptr("Name"), Value: extutil.Ptr("dev-demo-ngroup2")},
		{Key: extutil.Ptr("SpecialTag"), Value: extutil.Ptr("Great Thing")},
	},
}

func TestGetAllEc2Instances(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientMock)
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

	mockedZoneUtil := new(zoneMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1b"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	// When
	targets, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, ec2TargetId, target.TargetType)
	assert.Equal(t, "i-0ef9adc9fbd3b19c5 / dev-demo-ngroup2", target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"ami-02fc9c535f43bbc91"}, target.Attributes["aws-ec2.image"])
	assert.Equal(t, []string{"us-east-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"us-east-1b-id"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"10.3.92.28"}, target.Attributes["aws-ec2.ipv4.private"])
	assert.Equal(t, []string{"i-0ef9adc9fbd3b19c5"}, target.Attributes["aws-ec2.instance.id"])
	assert.Equal(t, []string{"ip-10-3-92-28.eu-central-1.compute.internal"}, target.Attributes["aws-ec2.hostname.internal"])
	assert.Equal(t, []string{"arn:aws:ec2:us-east-1:42:instance/i-0ef9adc9fbd3b19c5"}, target.Attributes["aws-ec2.arn"])
	assert.Equal(t, []string{"vpc-003cf5dda88c814c6"}, target.Attributes["aws-ec2.vpc"])
	assert.Equal(t, []string{"Great Thing"}, target.Attributes["aws-ec2.label.specialtag"])
	assert.Equal(t, []string{"running"}, target.Attributes["aws-ec2.state"])
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllEc2InstancesWithFilteredAttributes(t *testing.T) {
	// Given
	// set env var to filter out all attributes starting with "aws-ec2"
	config.Config.DiscoveryAttributesExcludesEc2 = []string{"aws-ec2.label.*", "aws-ec2.image"}
	mockedApi := new(ec2ClientMock)
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

	mockedZoneUtil := new(zoneMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1b"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	// When
	targets, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, ec2TargetId, target.TargetType)
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
	assert.Equal(t, []string{"running"}, target.Attributes["aws-ec2.state"])
	assert.NotContains(t, target.Attributes, "aws-ec2.label.specialtag")
	assert.NotContains(t, target.Attributes, "aws-ec2.image")
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestNameNotSet(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientMock)
	mockedReturnValue := ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: extutil.Ptr("i-0ef9adc9fbd3b19c5"),
						Placement: &types.Placement{
							AvailabilityZone: extutil.Ptr("us-east-1b"),
						},
					},
				},
			},
		},
	}
	mockedApi.On("DescribeInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	mockedZoneUtil := new(zoneMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1b"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	// When
	targets, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, "i-0ef9adc9fbd3b19c5", target.Label)
}

func TestGetAllEc2InstancesError(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientMock)

	mockedApi.On("DescribeInstances", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	mockedZoneUtil := new(zoneMock)
	mockedZone := types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1b"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1b-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	// When
	_, err := GetAllEc2Instances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.EqualError(t, err, "expected")
}
