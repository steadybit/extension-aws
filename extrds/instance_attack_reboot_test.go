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

func TestPrepareInstanceReboot(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"force-failover": true,
		},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.rds.instance.id": {"my-instance"},
				"aws.account":         {"42"},
			},
		}),
	})

	attack := rdsInstanceRebootAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "my-instance", state.DBInstanceIdentifier)
	assert.Equal(t, true, state.ForceFailover)
}

func TestPrepareInstanceRebootMustRequireAnInstanceId(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account": {"42"},
			},
		}),
	})

	attack := rdsInstanceRebootAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.ErrorContains(t, err, "aws.rds.instance.id")
}

func TestPrepareInstanceRebootMustRequireAnAccountId(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.rds.instance.id": {"my-instance"},
			},
		}),
	})

	attack := rdsInstanceRebootAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.ErrorContains(t, err, "aws.account")
}

func TestStartInstanceReboot(t *testing.T) {
	// Given
	api := new(rdsDBInstanceApiMock)
	api.On("RebootDBInstance", mock.Anything, mock.MatchedBy(func(params *rds.RebootDBInstanceInput) bool {
		require.Equal(t, "dev-db", *params.DBInstanceIdentifier)
		require.Equal(t, true, *params.ForceFailover)
		return true
	}), mock.Anything).Return(nil, nil)
	state := RdsInstanceAttackState{
		DBInstanceIdentifier: "dev-db",
		Account:              "42",
		ForceFailover:        true,
	}
	action := rdsInstanceRebootAttack{clientProvider: func(account string) (rdsDBInstanceApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartInstanceRebootForwardRebootError(t *testing.T) {
	// Given
	api := new(rdsDBInstanceApiMock)
	api.On("RebootDBInstance", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	state := RdsInstanceAttackState{
		DBInstanceIdentifier: "dev-db",
	}
	action := rdsInstanceRebootAttack{clientProvider: func(account string) (rdsDBInstanceApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.Error(t, err, "Failed to reboot database instance")
	api.AssertExpectations(t)
}
