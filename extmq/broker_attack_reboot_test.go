// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extmq

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPrepareReboot(t *testing.T) {
	body := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":        {"42"},
				"aws.region":         {"us-east-1"},
				"aws.mq.broker.id":   {"b-1234"},
				"aws.mq.broker.name": {"my-broker"},
			},
		}),
	})
	attack := brokerRebootAttack{}
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, body)
	require.NoError(t, err)
	assert.Equal(t, "b-1234", state.BrokerID)
	assert.Equal(t, "my-broker", state.BrokerName)
	assert.Equal(t, "42", state.Account)
	assert.Equal(t, "us-east-1", state.Region)
}

func TestStartRebootCallsApi(t *testing.T) {
	api := new(mqApiMock)
	api.On("RebootBroker", mock.Anything, mock.MatchedBy(func(p *mq.RebootBrokerInput) bool {
		return aws.ToString(p.BrokerId) == "b-1234"
	})).Return(&mq.RebootBrokerOutput{}, nil)

	attack := brokerRebootAttack{clientProvider: func(account string, region string, role *string) (MqApi, error) {
		return api, nil
	}}
	state := BrokerAttackState{BrokerID: "b-1234", BrokerName: "my-broker", Account: "42", Region: "us-east-1"}
	_, err := attack.Start(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartRebootForwardsError(t *testing.T) {
	api := new(mqApiMock)
	api.On("RebootBroker", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	attack := brokerRebootAttack{clientProvider: func(account string, region string, role *string) (MqApi, error) {
		return api, nil
	}}
	state := BrokerAttackState{BrokerID: "b-1234", BrokerName: "my-broker"}
	_, err := attack.Start(context.Background(), &state)
	assert.Error(t, err)
}
