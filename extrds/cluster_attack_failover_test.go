// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

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
				"aws.rds.cluster.id": {"my-cluster"},
				"aws.account":        {"42"},
			},
		}),
	})

	attack := rdsClusterFailoverAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "my-cluster", state.DBClusterIdentifier)
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

	attack := rdsClusterFailoverAttack{}
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

	attack := rdsClusterFailoverAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.ErrorContains(t, err, "aws.account")
}

func TestStartClusterFailover(t *testing.T) {
	// Given
	api := new(rdsDBClusterApiMock)
	api.On("FailoverDBCluster", mock.Anything, mock.MatchedBy(func(params *rds.FailoverDBClusterInput) bool {
		require.Equal(t, "dev-db", *params.DBClusterIdentifier)
		return true
	}), mock.Anything).Return(nil, nil)
	state := RdsClusterAttackState{
		DBClusterIdentifier: "dev-db",
		Account:             "42",
	}
	action := rdsClusterFailoverAttack{clientProvider: func(account string) (rdsDBClusterApi, error) {
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
	api := new(rdsDBClusterApiMock)
	api.On("FailoverDBCluster", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	state := RdsClusterAttackState{
		DBClusterIdentifier: "dev-db",
	}
	action := rdsClusterFailoverAttack{clientProvider: func(account string) (rdsDBClusterApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.Error(t, err, "Failed to failover database cluster")
	api.AssertExpectations(t)
}
