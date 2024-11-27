// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"errors"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"testing"
)

func TestGetAllRdsInstances(t *testing.T) {
	// Given
	mockedApi := new(rdsDBInstanceApiMock)
	mockedReturnValue := rds.DescribeDBInstancesOutput{
		DBInstances: []types.DBInstance{
			{
				DBInstanceArn:        discovery_kit_api.Ptr("arn"),
				DBInstanceIdentifier: discovery_kit_api.Ptr("identifier"),
				AvailabilityZone:     discovery_kit_api.Ptr("us-east-1a"),
				Engine:               discovery_kit_api.Ptr("engine"),
				DBClusterIdentifier:  discovery_kit_api.Ptr("cluster"),
				DBInstanceStatus:     discovery_kit_api.Ptr("status"),
				TagList: []types.Tag{
					{Key: discovery_kit_api.Ptr("SpecialTag"), Value: discovery_kit_api.Ptr("Great Thing")},
				},
			},
		},
	}
	mockedApi.On("DescribeDBInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	mockedZoneUtil := new(zoneMock)
	mockedZone := ec2types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1a"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1a-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	// When
	targets, err := getAllRdsInstances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, rdsInstanceTargetId, target.TargetType)
	assert.Equal(t, "identifier", target.Label)
	assert.Equal(t, "arn", target.Id)
	assert.Equal(t, 10, len(target.Attributes))
	assert.Equal(t, []string{"cluster"}, target.Attributes["aws.rds.cluster"])
	assert.Equal(t, []string{"status"}, target.Attributes["aws.rds.instance.status"])
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"us-east-1a"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"us-east-1a-id"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"Great Thing"}, target.Attributes["aws.rds.label.specialtag"])
}

func TestGetAllRdsInstancesWithoutCluster(t *testing.T) {
	// Given
	mockedApi := new(rdsDBInstanceApiMock)
	mockedReturnValue := rds.DescribeDBInstancesOutput{
		DBInstances: []types.DBInstance{
			{
				DBInstanceArn:        discovery_kit_api.Ptr("arn"),
				DBInstanceIdentifier: discovery_kit_api.Ptr("identifier"),
				AvailabilityZone:     discovery_kit_api.Ptr("us-east-1a"),
				Engine:               discovery_kit_api.Ptr("engine"),
				DBInstanceStatus:     discovery_kit_api.Ptr("status"),
				DBClusterIdentifier:  nil,
				TagList: []types.Tag{
					{Key: discovery_kit_api.Ptr("SpecialTag"), Value: discovery_kit_api.Ptr("Great Thing")},
				},
			},
		},
	}
	mockedApi.On("DescribeDBInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)
	mockedZoneUtil := new(zoneMock)
	mockedZone := ec2types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1a"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1a-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	// When
	targets, err := getAllRdsInstances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, rdsInstanceTargetId, target.TargetType)
	assert.Equal(t, "identifier", target.Label)
	assert.Equal(t, "arn", target.Id)
	assert.Equal(t, 9, len(target.Attributes))
	assert.Equal(t, []string(nil), target.Attributes["aws.rds.cluster"])
	assert.Equal(t, []string{"Great Thing"}, target.Attributes["aws.rds.label.specialtag"])
}

func TestGetAllRdsInstancesWithPagination(t *testing.T) {
	// Given
	mockedApi := new(rdsDBInstanceApiMock)
	mockedZoneUtil := new(zoneMock)
	mockedZone := ec2types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1a"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1a-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	withMarker := mock.MatchedBy(func(arg *rds.DescribeDBInstancesInput) bool {
		return arg.Marker != nil
	})
	withoutMarker := mock.MatchedBy(func(arg *rds.DescribeDBInstancesInput) bool {
		return arg.Marker == nil
	})
	mockedApi.On("DescribeDBInstances", mock.Anything, withoutMarker).Return(discovery_kit_api.Ptr(rds.DescribeDBInstancesOutput{
		Marker: discovery_kit_api.Ptr("marker"),
		DBInstances: []types.DBInstance{
			{
				DBInstanceArn:        discovery_kit_api.Ptr("arn1"),
				DBInstanceIdentifier: discovery_kit_api.Ptr("identifier1"),
				AvailabilityZone:     discovery_kit_api.Ptr("az1"),
				Engine:               discovery_kit_api.Ptr("engine1"),
				DBInstanceStatus:     discovery_kit_api.Ptr("status"),
				DBClusterIdentifier:  nil,
			},
		},
	}), nil)
	mockedApi.On("DescribeDBInstances", mock.Anything, withMarker).Return(discovery_kit_api.Ptr(rds.DescribeDBInstancesOutput{
		DBInstances: []types.DBInstance{
			{
				DBInstanceArn:        discovery_kit_api.Ptr("arn2"),
				DBInstanceIdentifier: discovery_kit_api.Ptr("identifier2"),
				AvailabilityZone:     discovery_kit_api.Ptr("az2"),
				Engine:               discovery_kit_api.Ptr("engine2"),
				DBInstanceStatus:     discovery_kit_api.Ptr("status2"),
				DBClusterIdentifier:  nil,
			},
		},
	}), nil)

	// When
	targets, err := getAllRdsInstances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(targets))
	assert.Equal(t, "arn1", targets[0].Id)
	assert.Equal(t, "arn2", targets[1].Id)
}

func TestGetAllRdsInstancesError(t *testing.T) {
	// Given
	mockedApi := new(rdsDBInstanceApiMock)
	mockedZoneUtil := new(zoneMock)
	mockedZone := ec2types.AvailabilityZone{
		ZoneName:   discovery_kit_api.Ptr("us-east-1a"),
		RegionName: discovery_kit_api.Ptr("us-east-1"),
		ZoneId:     discovery_kit_api.Ptr("us-east-1a-id"),
	}
	mockedZoneUtil.On("GetZone", mock.Anything, mock.Anything, mock.Anything).Return(&mockedZone)

	mockedApi.On("DescribeDBInstances", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := getAllRdsInstances(context.Background(), mockedApi, mockedZoneUtil, "42", "us-east-1")

	// Then
	assert.Equal(t, err.Error(), "expected")
}
