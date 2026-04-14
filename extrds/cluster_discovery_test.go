// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"errors"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
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
				DBClusterArn:        new("arn"),
				DBClusterIdentifier: new("identifier"),
				AvailabilityZones:   []string{"zone-1", "zone-2"},
				Engine:              new("engine"),
				Status:              new("status"),
				MultiAZ:             new(true),
				TagList: []types.Tag{
					{Key: new("SpecialTag"), Value: new("Great Thing")},
				},
			},
		},
	}
	mockedApi.On("DescribeDBClusters", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	targets, err := getAllRdsClusters(context.Background(), mockedApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{
				Key:    "SpecialTag",
				Values: []string{"something else", "Great Thing"},
			},
		},
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, rdsClusterTargetId, target.TargetType)
	assert.Equal(t, "identifier", target.Label)
	assert.Equal(t, "arn", target.Id)
	assert.Equal(t, 10, len(target.Attributes))
	assert.Equal(t, []string{"status"}, target.Attributes["aws.rds.cluster.status"])
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws.rds.cluster.multi-az"])
	assert.Equal(t, []string{"Great Thing"}, target.Attributes["aws.rds.cluster.label.specialtag"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
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
	mockedApi.On("DescribeDBClusters", mock.Anything, withoutMarker).Return(new(rds.DescribeDBClustersOutput{
		Marker: new("marker"),
		DBClusters: []types.DBCluster{
			{
				DBClusterArn:        new("arn1"),
				DBClusterIdentifier: new("identifier1"),
				AvailabilityZones:   []string{"zone-1", "zone-2"},
				Engine:              new("engine1"),
				Status:              new("status"),
			},
		},
	}), nil)
	mockedApi.On("DescribeDBClusters", mock.Anything, withMarker).Return(new(rds.DescribeDBClustersOutput{
		DBClusters: []types.DBCluster{
			{
				DBClusterArn:        new("arn2"),
				DBClusterIdentifier: new("identifier2"),
				AvailabilityZones:   []string{"zone-1", "zone-2"},
				Engine:              new("engine2"),
				Status:              new("status2"),
			},
		},
	}), nil)

	// When
	targets, err := getAllRdsClusters(context.Background(), mockedApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
	})

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
	_, err := getAllRdsClusters(context.Background(), mockedApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    new("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, err.Error(), "expected")
}
