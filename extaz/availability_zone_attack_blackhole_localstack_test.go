/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extaz

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func testPrepareAndStartAndStopBlackholeLocalStack(t *testing.T, clientEc2 ec2.Client, clientImds imds.Client) {
	// Prepare
	// Given
	requestBodyPrepare := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.zone":    {"eu-west-1a"},
				"aws.account": {"42"},
			},
		}),
		ExecutionContext: extutil.Ptr(action_kit_api.ExecutionContext{
			AgentAwsAccountId: aws.String("41"),
		}),
		ExecutionId: uuid.New(),
	}
	requestBodyPrepareJson, err := json.Marshal(requestBodyPrepare)
	require.Nil(t, err)

	// When
	state, attackErr := PrepareBlackhole(context.Background(), requestBodyPrepareJson, "41", func(account string) (AZBlackholeEC2Api, AZBlackholeImdsApi, error) {
		return &clientEc2, &clientImds, nil
	})

	// Then
	assert.Nil(t, attackErr)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1a", state.TargetZone)
	assert.Len(t, state.TargetSubnets, 1)
	vpcIds := reflect.ValueOf(state.TargetSubnets).MapKeys()
	assert.Len(t, state.TargetSubnets[vpcIds[0].String()], 1)
	assert.NotNil(t, state.AttackExecutionId)

	acls, err := clientEc2.DescribeNetworkAcls(context.Background(), &ec2.DescribeNetworkAclsInput{})
	require.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 1)

	// Start
	// Given
	var convertedState action_kit_api.ActionState
	err = extconversion.Convert(*state, &convertedState)
	require.Nil(t, err)

	requestBodyStart := action_kit_api.StartActionRequestBody{
		State: convertedState,
	}
	requestBodyStartJson, err := json.Marshal(requestBodyStart)
	require.Nil(t, err)

	// When
	state, attackStartErr := StartBlackhole(context.Background(), requestBodyStartJson, func(account string) (AZBlackholeEC2Api, error) {
		return &clientEc2, nil
	})

	// Then
	assert.Nil(t, attackStartErr)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1a", state.TargetZone)
	assert.Len(t, state.NetworkAclIds, 1)
	newAssociationIds := reflect.ValueOf(state.OldNetworkAclIds).MapKeys()
	assert.NotEqual(t, "", state.OldNetworkAclIds[newAssociationIds[0].String()])
	assert.NotNil(t, state.AttackExecutionId)

	acls, err = clientEc2.DescribeNetworkAcls(context.Background(), &ec2.DescribeNetworkAclsInput{})
	require.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 2)

	// Stop
	// Given
	var convertedStateStop action_kit_api.ActionState
	err = extconversion.Convert(*state, &convertedStateStop)
	require.Nil(t, err)

	requestBodyStop := action_kit_api.StopActionRequestBody{
		State: convertedStateStop,
	}
	requestBodyStopJson, err := json.Marshal(requestBodyStop)
	require.Nil(t, err)
	// When
	stopResult, attackErr := StopBlackhole(requestBodyStopJson, context.Background(), func(account string) (AZBlackholeEC2Api, error) {
		return &clientEc2, nil
	})

	assert.Nil(t, attackErr)
	assert.NotNil(t, stopResult)

	acls, err = clientEc2.DescribeNetworkAcls(context.Background(), &ec2.DescribeNetworkAclsInput{})
	require.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 1)
}
