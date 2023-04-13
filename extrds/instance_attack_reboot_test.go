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
	requestBody := action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.rds.instance.id": {"my-instance"},
				"aws.account":         {"42"},
			},
		}),
	}

	attack := rdsInstanceAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "my-instance", state.DBInstanceIdentifier)
}

func TestPrepareInstanceRebootMustRequireAnInstanceId(t *testing.T) {
	// Given
	requestBody := action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account": {"42"},
			},
		}),
	}

	attack := rdsInstanceAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.ErrorContains(t, err, "aws.rds.instance.id")
}

func TestPrepareInstanceRebootMustRequireAnAccountId(t *testing.T) {
	// Given
	requestBody := action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.rds.instance.id": {"my-instance"},
			},
		}),
	}

	attack := rdsInstanceAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.ErrorContains(t, err, "aws.account")
}

type rdsRebootDBInstanceApiMock struct {
	mock.Mock
}

func (m *rdsRebootDBInstanceApiMock) RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, opts ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error) {
	args := m.Called(ctx, params, opts)
	return nil, args.Error(1)
}

func TestStartInstanceReboot(t *testing.T) {
	// Given
	api := new(rdsRebootDBInstanceApiMock)
	api.On("RebootDBInstance", mock.Anything, mock.MatchedBy(func(params *rds.RebootDBInstanceInput) bool {
		require.Equal(t, "dev-db", *params.DBInstanceIdentifier)
		return true
	}), mock.Anything).Return(nil, nil)
	state := RdsInstanceAttackState{
		DBInstanceIdentifier: "dev-db",
		Account:              "42",
	}
	action := rdsInstanceAttack{clientProvider: func(account string) (rdsRebootDBInstanceApi, error) {
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
	api := new(rdsRebootDBInstanceApiMock)
	api.On("RebootDBInstance", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	state := RdsInstanceAttackState{
		DBInstanceIdentifier: "dev-db",
	}
	action := rdsInstanceAttack{clientProvider: func(account string) (rdsRebootDBInstanceApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.Error(t, err, "Failed to execute database instance reboot")
	api.AssertExpectations(t)
}
