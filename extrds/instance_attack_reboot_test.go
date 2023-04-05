// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
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

	attack := NewRdsInstanceAttack()
	state := attack.NewEmptyState()

	// When
	result, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.Nil(t, err)
	assert.Nil(t, result)
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

	attack := NewRdsInstanceAttack()
	state := attack.NewEmptyState()

	// When
	result, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.Nil(t, result)
	assert.Contains(t, err.(*extension_kit.ExtensionError).Title, "aws.rds.instance.id")
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

	attack := NewRdsInstanceAttack()
	state := attack.NewEmptyState()

	// When
	result, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.Nil(t, result)
	assert.Contains(t, err.(*extension_kit.ExtensionError).Title, "aws.account")
}

type rdsRebootDBInstanceApiMock struct {
	mock.Mock
}

func (m rdsRebootDBInstanceApiMock) RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, _ ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error) {
	args := m.Called(ctx, params)
	return nil, args.Error(1)
}

func TestStartInstanceReboot(t *testing.T) {
	// Given
	mockedApi := new(rdsRebootDBInstanceApiMock)
	mockedApi.On("RebootDBInstance", mock.Anything, mock.MatchedBy(func(params *rds.RebootDBInstanceInput) bool {
		require.Equal(t, "dev-db", *params.DBInstanceIdentifier)
		return true
	})).Return(nil, nil)
	state := RdsInstanceAttackState{
		DBInstanceIdentifier: "dev-db",
		Account:              "42",
	}

	// When
	result, err := startAttack(context.Background(), &state, func(account string) (RdsRebootDBInstanceClient, error) {
		assert.Equal(t, "42", account)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestStartInstanceRebootForwardRebootError(t *testing.T) {
	// Given
	mockedApi := new(rdsRebootDBInstanceApiMock)
	mockedApi.On("RebootDBInstance", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	state := RdsInstanceAttackState{
		DBInstanceIdentifier: "dev-db",
	}

	// When
	result, err := startAttack(context.Background(), &state, func(account string) (RdsRebootDBInstanceClient, error) {
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, result)
	assert.Equal(t, "Failed to execute database instance reboot", err.(*extension_kit.ExtensionError).Title)
}
