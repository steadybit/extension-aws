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

func TestGetAllRdsClusters(t *testing.T) {
	// Given
	mockedApi := new(rdsDBClusterApiMock)
	mockedReturnValue := rds.DescribeDBClustersOutput{
		DBClusters: []types.DBCluster{
			{
				DBClusterArn:        discovery_kit_api.Ptr("arn"),
				DBClusterIdentifier: discovery_kit_api.Ptr("identifier"),
				AvailabilityZones:   []string{"zone-1", "zone-2"},
				Engine:              discovery_kit_api.Ptr("engine"),
				Status:              discovery_kit_api.Ptr("status"),
				MultiAZ:             discovery_kit_api.Ptr(true),
			},
		},
	}
	mockedApi.On("DescribeDBClusters", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	targets, err := getAllRdsClusters(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, rdsClusterTargetId, target.TargetType)
	assert.Equal(t, "identifier", target.Label)
	assert.Equal(t, "arn", target.Id)
	assert.Equal(t, 8, len(target.Attributes))
	assert.Equal(t, []string{"status"}, target.Attributes["aws.rds.cluster.status"])
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws.rds.cluster.multi-az"])
}

func TestGetAllRdsClustersWithPagination(t *testing.T) {
	// Given
	mockedApi := new(rdsDBClusterApiMock)

	withMarker := mock.MatchedBy(func(arg *rds.DescribeDBClustersInput) bool {
		return arg.Marker != nil
	})
	withoutMarker := mock.MatchedBy(func(arg *rds.DescribeDBClustersInput) bool {
		return arg.Marker == nil
	})
	mockedApi.On("DescribeDBClusters", mock.Anything, withoutMarker).Return(discovery_kit_api.Ptr(rds.DescribeDBClustersOutput{
		Marker: discovery_kit_api.Ptr("marker"),
		DBClusters: []types.DBCluster{
			{
				DBClusterArn:        discovery_kit_api.Ptr("arn1"),
				DBClusterIdentifier: discovery_kit_api.Ptr("identifier1"),
				AvailabilityZones:   []string{"zone-1", "zone-2"},
				Engine:              discovery_kit_api.Ptr("engine1"),
				Status:              discovery_kit_api.Ptr("status"),
			},
		},
	}), nil)
	mockedApi.On("DescribeDBClusters", mock.Anything, withMarker).Return(discovery_kit_api.Ptr(rds.DescribeDBClustersOutput{
		DBClusters: []types.DBCluster{
			{
				DBClusterArn:        discovery_kit_api.Ptr("arn2"),
				DBClusterIdentifier: discovery_kit_api.Ptr("identifier2"),
				AvailabilityZones:   []string{"zone-1", "zone-2"},
				Engine:              discovery_kit_api.Ptr("engine2"),
				Status:              discovery_kit_api.Ptr("status2"),
			},
		},
	}), nil)

	// When
	targets, err := getAllRdsClusters(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(targets))
	assert.Equal(t, "arn1", targets[0].Id)
	assert.Equal(t, "arn2", targets[1].Id)
}

func TestGetAllRdsClustersError(t *testing.T) {
	// Given
	mockedApi := new(rdsDBClusterApiMock)

	mockedApi.On("DescribeDBClusters", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := getAllRdsClusters(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, err.Error(), "expected")
}
