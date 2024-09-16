// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extmsk

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRebootBroker(t *testing.T) {
	// Given
	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.msk.cluster.arn":       {"arn"},
				"aws.msk.cluster.broker.id": {"1"},
				"aws.msk.cluster.name":      {"test"},
				"aws.account":               {"42"},
			},
		}),
	})

	attack := mskRebootBrokerAttack{}
	state := attack.NewEmptyState()

	// When
	_, err := attack.Prepare(context.Background(), &state, requestBody)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "arn", state.ClusterARN)
	assert.Equal(t, "test", state.ClusterName)
	assert.Equal(t, "42", state.Account)
	assert.Equal(t, "1", state.BrokerID)
}

func TestStartRebootBroker(t *testing.T) {
	// Given
	api := new(mskClusterApiMock)
	api.On("RebootBroker", mock.Anything, mock.MatchedBy(func(params *kafka.RebootBrokerInput) bool {
		require.Equal(t, "arn", *params.ClusterArn)
		require.Equal(t, []string{"1"}, params.BrokerIds)
		return true
	}), mock.Anything).Return(nil, nil)
	state := KafkaAttackState{
		ClusterName: "test",
		ClusterARN:  "arn",
		BrokerID:    "1",
		BrokerARN:   "broker-arn",
		Account:     "42",
	}
	action := mskRebootBrokerAttack{clientProvider: func(account string) (MskApi, error) {
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
	api := new(mskClusterApiMock)
	api.On("RebootBroker", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	state := KafkaAttackState{
		ClusterARN: "arn",
	}
	action := mskRebootBrokerAttack{clientProvider: func(account string) (MskApi, error) {
		return api, nil
	}}

	// When
	_, err := action.Start(context.Background(), &state)

	// Then
	assert.Error(t, err, "Failed to trigger kafka broker reboot")
	api.AssertExpectations(t)
}
