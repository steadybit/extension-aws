// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"encoding/json"
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
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareInstanceReboot(requestBodyJson)

	// Then
	assert.Nil(t, attackErr)
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
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareInstanceReboot(requestBodyJson)

	// Then
	assert.Nil(t, state)
	assert.Contains(t, attackErr.Title, "aws.rds.instance.id")
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
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareInstanceReboot(requestBodyJson)

	// Then
	assert.Nil(t, state)
	assert.Contains(t, attackErr.Title, "aws.account")
}

func TestPrepareInstanceRebootMustFailOnInvalidBody(t *testing.T) {
	// When
	state, attackErr := PrepareInstanceReboot([]byte{})

	// Then
	assert.Nil(t, state)
	assert.Contains(t, attackErr.Title, "Failed to parse request body")
}

func TestStartInstanceRebootMustFailOnInvalidBody(t *testing.T) {
	// When
	attackErr := StartInstanceReboot(context.Background(), []byte{}, nil)

	// Then
	assert.Contains(t, attackErr.Title, "Failed to parse request body")
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
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"DBInstanceIdentifier": "dev-db",
			"Account":              "42",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	attackError := StartInstanceReboot(context.Background(), requestBodyJson, func(account string) (RdsRebootDBInstanceClient, error) {
		assert.Equal(t, "42", account)
		return mockedApi, nil
	})

	// Then
	assert.Nil(t, attackError)
}

func TestStartInstanceRebootForwardRebootError(t *testing.T) {
	// Given
	mockedApi := new(rdsRebootDBInstanceApiMock)
	mockedApi.On("RebootDBInstance", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	requestBody := action_kit_api.StartActionRequestBody{
		State: map[string]interface{}{
			"DBInstanceIdentifier": "dev-db",
		},
	}
	requestBodyJson, err := json.Marshal(requestBody)
	require.Nil(t, err)

	// When
	attackError := StartInstanceReboot(context.Background(), requestBodyJson, func(account string) (RdsRebootDBInstanceClient, error) {
		return mockedApi, nil
	})

	// Then
	assert.Equal(t, "Failed to execute database instance reboot", attackError.Title)
}
