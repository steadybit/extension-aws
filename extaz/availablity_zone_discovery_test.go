// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extaz

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"testing"
)

type ec2ClientMock struct {
	mock.Mock
}

func (m ec2ClientMock) DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeAvailabilityZonesOutput), args.Error(1)
}

func TestGetAllAvailabilityZones(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientMock)
	mockedReturnValue := ec2.DescribeAvailabilityZonesOutput{
		AvailabilityZones: []types.AvailabilityZone{
			{
				ZoneName:   discovery_kit_api.Ptr("eu-central-1b"),
				RegionName: discovery_kit_api.Ptr("eu-central-1"),
				ZoneId:     discovery_kit_api.Ptr("euc1-az3"),
			},
		},
	}
	mockedApi.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	targets, err := GetAllAvailabilityZones(context.Background(), mockedApi, "42")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, azTargetId, target.TargetType)
	assert.Equal(t, "eu-central-1b", target.Label)
	assert.Equal(t, 5, len(target.Attributes))
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"eu-central-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"eu-central-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"euc1-az3"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"eu-central-1b@42"}, target.Attributes["aws.zone@account"])
}

func TestGetAllAvailabilityZonesError(t *testing.T) {
	// Given
	mockedApi := new(ec2ClientMock)

	mockedApi.On("DescribeAvailabilityZones", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := GetAllAvailabilityZones(context.Background(), mockedApi, "42")

	// Then
	assert.Equal(t, err.Error(), "expected")
}
