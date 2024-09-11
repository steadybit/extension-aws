// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extelasticache

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	types2 "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"testing"
)

func TestGetAllElasticacheReplicationGroups(t *testing.T) {
	// Given
	mockedApi := new(elasticacheReplicationGroupApiMock)
	mockedReturnValue := elasticache.DescribeReplicationGroupsOutput{
		Marker: nil,
		ReplicationGroups: []types2.ReplicationGroup{
			{
				ARN:                     aws.String("arn"),
				AtRestEncryptionEnabled: aws.Bool(false),
				AuthTokenEnabled:        aws.Bool(false),
				AutomaticFailover:       types2.AutomaticFailoverStatusEnabled,
				CacheNodeType:           aws.String("cache.t4g.micro"),
				ClusterEnabled:          aws.Bool(false),
				ClusterMode:             types2.ClusterModeDisabled,
				MemberClusters:          []string{"redis-steadybit-dev-001", "redis-steadybit-dev-002"},
				MultiAZ:                 types2.MultiAZStatusEnabled,
				NodeGroups: []types2.NodeGroup{
					{NodeGroupId: aws.String("0001"), NodeGroupMembers: nil, Status: aws.String("available")},
				},
				ReplicationGroupId: aws.String("redis-steadybit-dev"),
				Status:             aws.String("available"),
			},
		},
		ResultMetadata: middleware.Metadata{},
	}
	mockedApi.On("DescribeReplicationGroups", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	targets, err := getAllElasticacheReplicationGroups(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, elasticacheNodeGroupTargetId, target.TargetType)
	assert.Equal(t, "redis-steadybit-dev-0001", target.Label)
	assert.Equal(t, "redis-steadybit-dev-0001", target.Id)
	assert.Equal(t, 10, len(target.Attributes))
	assert.Equal(t, []string{"redis-steadybit-dev"}, target.Attributes["aws.elasticache.replication-group.id"])
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"available"}, target.Attributes["aws.elasticache.replication-group.status"])
	assert.Equal(t, []string{"enabled"}, target.Attributes["aws.elasticache.replication-group.automatic-failover"])
	assert.Equal(t, []string{"disabled"}, target.Attributes["aws.elasticache.replication-group.cluster-mode"])
	assert.Equal(t, []string{"enabled"}, target.Attributes["aws.elasticache.replication-group.multi-az"])
	assert.Equal(t, []string{"cache.t4g.micro"}, target.Attributes["aws.elasticache.replication-group.cache-node-type"])
	assert.Equal(t, []string{"0001"}, target.Attributes["aws.elasticache.replication-group.node-group.id"])
	assert.Equal(t, []string{"available"}, target.Attributes["aws.elasticache.replication-group.node-group.status"])
}

func TestGetAllElasticacheReplicationGroupsWithPagination(t *testing.T) {
	// Given
	mockedApi := new(elasticacheReplicationGroupApiMock)

	withMarker := mock.MatchedBy(func(arg *elasticache.DescribeReplicationGroupsInput) bool {
		return arg.Marker != nil
	})
	withoutMarker := mock.MatchedBy(func(arg *elasticache.DescribeReplicationGroupsInput) bool {
		return arg.Marker == nil
	})
	mockedApi.On("DescribeReplicationGroups", mock.Anything, withoutMarker).Return(discovery_kit_api.Ptr(elasticache.DescribeReplicationGroupsOutput{
		Marker: discovery_kit_api.Ptr("marker"),
		ReplicationGroups: []types2.ReplicationGroup{
			{
				ARN:                     aws.String("arn1"),
				AtRestEncryptionEnabled: aws.Bool(false),
				AuthTokenEnabled:        aws.Bool(false),
				AutomaticFailover:       types2.AutomaticFailoverStatusEnabled,
				CacheNodeType:           aws.String("cache.t4g.micro"),
				ClusterEnabled:          aws.Bool(false),
				ClusterMode:             types2.ClusterModeDisabled,
				MemberClusters:          []string{"redis-steadybit-dev-001", "redis-steadybit-dev-002"},
				MultiAZ:                 types2.MultiAZStatusEnabled,
				NodeGroups: []types2.NodeGroup{
					{NodeGroupId: aws.String("0001"), NodeGroupMembers: nil, Status: aws.String("available")},
				},
				ReplicationGroupId: aws.String("redis-steadybit-dev"),
				Status:             aws.String("available"),
			},
		},
		ResultMetadata: middleware.Metadata{},
	}), nil)
	mockedApi.On("DescribeReplicationGroups", mock.Anything, withMarker).Return(discovery_kit_api.Ptr(elasticache.DescribeReplicationGroupsOutput{
		Marker: nil,
		ReplicationGroups: []types2.ReplicationGroup{
			{
				ARN:                     aws.String("arn2"),
				AtRestEncryptionEnabled: aws.Bool(false),
				AuthTokenEnabled:        aws.Bool(false),
				AutomaticFailover:       types2.AutomaticFailoverStatusEnabled,
				CacheNodeType:           aws.String("cache.t4g.micro"),
				ClusterEnabled:          aws.Bool(false),
				ClusterMode:             types2.ClusterModeDisabled,
				MemberClusters:          []string{"redis-steadybit-stg-001", "redis-steadybit-stg-002"},
				MultiAZ:                 types2.MultiAZStatusEnabled,
				NodeGroups: []types2.NodeGroup{
					{NodeGroupId: aws.String("0001"), NodeGroupMembers: nil, Status: aws.String("available")},
				},
				ReplicationGroupId: aws.String("redis-steadybit-stg"),
				Status:             aws.String("available"),
			},
		},
		ResultMetadata: middleware.Metadata{},
	}), nil)

	// When
	targets, err := getAllElasticacheReplicationGroups(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(targets))
	assert.Equal(t, "redis-steadybit-dev-0001", targets[0].Id)
	assert.Equal(t, "redis-steadybit-stg-0001", targets[1].Id)
}

func TestGetAllRdsClustersError(t *testing.T) {
	// Given
	mockedApi := new(elasticacheReplicationGroupApiMock)

	mockedApi.On("DescribeReplicationGroups", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := getAllElasticacheReplicationGroups(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, err.Error(), "expected")
}
