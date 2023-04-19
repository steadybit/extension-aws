// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"testing"
)

type rdsClientMock struct {
	mock.Mock
}

func (m *rdsClientMock) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rds.DescribeDBInstancesOutput), args.Error(1)
}

func TestGetAllRdsInstances(t *testing.T) {
	// Given
	mockedApi := new(rdsClientMock)
	mockedReturnValue := rds.DescribeDBInstancesOutput{
		DBInstances: []types.DBInstance{
			{
				DBInstanceArn:        discovery_kit_api.Ptr("arn"),
				DBInstanceIdentifier: discovery_kit_api.Ptr("identifier"),
				AvailabilityZone:     discovery_kit_api.Ptr("az"),
				Engine:               discovery_kit_api.Ptr("engine"),
				DBClusterIdentifier:  discovery_kit_api.Ptr("cluster"),
				DBInstanceStatus:     discovery_kit_api.Ptr("status"),
			},
		},
	}
	mockedApi.On("DescribeDBInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	targets, err := GetAllRdsInstances(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, rdsTargetId, target.TargetType)
	assert.Equal(t, "identifier", target.Label)
	assert.Equal(t, "arn", target.Id)
	assert.Equal(t, 8, len(target.Attributes))
	assert.Equal(t, []string{"cluster"}, target.Attributes["aws.rds.cluster"])
	assert.Equal(t, []string{"status"}, target.Attributes["aws.rds.instance.status"])
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
}

func TestGetAllRdsInstancesWithoutCluster(t *testing.T) {
	// Given
	mockedApi := new(rdsClientMock)
	mockedReturnValue := rds.DescribeDBInstancesOutput{
		DBInstances: []types.DBInstance{
			{
				DBInstanceArn:        discovery_kit_api.Ptr("arn"),
				DBInstanceIdentifier: discovery_kit_api.Ptr("identifier"),
				AvailabilityZone:     discovery_kit_api.Ptr("az"),
				Engine:               discovery_kit_api.Ptr("engine"),
				DBInstanceStatus:     discovery_kit_api.Ptr("status"),
				DBClusterIdentifier:  nil,
			},
		},
	}
	mockedApi.On("DescribeDBInstances", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	targets, err := GetAllRdsInstances(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, rdsTargetId, target.TargetType)
	assert.Equal(t, "identifier", target.Label)
	assert.Equal(t, "arn", target.Id)
	assert.Equal(t, 7, len(target.Attributes))
	assert.Equal(t, []string(nil), target.Attributes["aws.rds.cluster"])
}

func TestGetAllRdsInstancesWithPagination(t *testing.T) {
	// Given
	mockedApi := new(rdsClientMock)

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
	targets, err := GetAllRdsInstances(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(targets))
	assert.Equal(t, "arn1", targets[0].Id)
	assert.Equal(t, "arn2", targets[1].Id)
}

func TestGetAllRdsInstancesError(t *testing.T) {
	// Given
	mockedApi := new(rdsClientMock)

	mockedApi.On("DescribeDBInstances", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := GetAllRdsInstances(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, err.Error(), "expected")
}
