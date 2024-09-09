// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelasticache

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTestFailover(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.elasticache.replication-group.node-group.id": {"0001"},
				"aws.elasticache.replication-group.id":            {"redis-steadybit-dev"},
				"aws.account":                                     {"42"},
			},
		}),
	})

	attack := elasticacheNodeGroupFailoverAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "0001", state.NodeGroupID)
	assert.Equal(t, "42", state.Account)
	assert.Equal(t, "redis-steadybit-dev", state.ReplicationGroupID)
}

func TestStartClusterFailover(t *testing.T) {
	// Given
	api := new(elasticacheReplicationGroupApiMock)
	api.On("TestFailover", mock.Anything, mock.MatchedBy(func(params *elasticache.TestFailoverInput) bool {
		require.Equal(t, "redis-steadybit-dev", *params.ReplicationGroupId)
		require.Equal(t, "0001", *params.NodeGroupId)
		return true
	}), mock.Anything).Return(nil, nil)
	state := ElasticacheClusterAttackState{
		ReplicationGroupID: "redis-steadybit-dev",
		NodeGroupID:        "0001",
		Account:            "42",
	}
	action := elasticacheNodeGroupFailoverAttack{clientProvider: func(account string) (ElasticacheApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartClusterFailoverForwardFailoverError(t *testing.T) {
	// Given
	api := new(elasticacheReplicationGroupApiMock)
	api.On("TestFailover", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	state := ElasticacheClusterAttackState{
		ReplicationGroupID: "redis-steadybit-dev",
	}
	action := elasticacheNodeGroupFailoverAttack{clientProvider: func(account string) (ElasticacheApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.Error(t, err, "Failed to failover elasticache nodegroup")
	api.AssertExpectations(t)
}
