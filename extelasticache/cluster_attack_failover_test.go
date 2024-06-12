// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelasticache

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPrepareClusterFailover(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.elasticache.replication-group.id": {"my-cluster"},
				"aws.elasticache.cluster.id":           {"my-cluster-001"},
				"aws.account":                          {"42"},
			},
		}),
	})

	attack := replicationGroupFailoverAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "my-cluster", state.ReplicationGroupId)
	assert.Equal(t, "my-cluster-001", state.ReplicationGroupId)
}

func TestPrepareClusterFailoverMustRequireAnClusterId(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account": {"42"},
			},
		}),
	})

	attack := replicationGroupFailoverAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.ErrorContains(t, err, "aws.rds.cluster.id")
}

func TestPrepareClusterFailoverMustRequireAnAccountId(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.rds.cluster.id": {"my-cluster"},
			},
		}),
	})

	attack := replicationGroupFailoverAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.ErrorContains(t, err, "aws.account")
}

func TestStartClusterFailover(t *testing.T) {
	// Given
	api := new(replicationGroupApiMock)
	api.On("TestFailover", mock.Anything, mock.MatchedBy(func(params *rds.FailoverDBClusterInput) bool {
		require.Equal(t, "dev-db", *params.DBClusterIdentifier)
		return true
	}), mock.Anything).Return(nil, nil)
	state := ReplicationGroupAttackState{
		ReplicationGroupId: "dev-cache-001",
		ReplicationGroupId: "dev-cache",
		Account:            "42",
	}
	action := replicationGroupFailoverAttack{clientProvider: func(account string) (ReplicationGroupApi, error) {
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
	api := new(replicationGroupApiMock)
	api.On("TestFailover", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	state := ReplicationGroupAttackState{
		ReplicationGroupId: "dev-cache-001",
		ReplicationGroupId: "dev-cache",
		Account:            "42",
	}

	action := replicationGroupFailoverAttack{clientProvider: func(account string) (ReplicationGroupApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.Error(t, err, "Failed to failover cache cluster")
	api.AssertExpectations(t)
}
