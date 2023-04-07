/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extaz

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func testPrepareAndStartAndStopBlackholeLocalStack(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	// Given
	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "41",
		clientProvider: func(account string) (azBlackholeEC2Api, azBlackholeImdsApi, error) {
			return clientEc2, clientImds, nil
		}}
	state := action.NewEmptyState()

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

	// When
	_, err := action.Prepare(ctx, &state, requestBodyPrepare)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1a", state.TargetZone)
	assert.Len(t, state.TargetSubnets, 1)
	vpcIds := reflect.ValueOf(state.TargetSubnets).MapKeys()
	assert.Len(t, state.TargetSubnets[vpcIds[0].String()], 1)
	assert.NotNil(t, state.AttackExecutionId)

	acls, err := clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	require.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 1)

	// Start
	// When
	_, attackStartErr := action.Start(ctx, &state)
	// Then
	assert.Nil(t, attackStartErr)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1a", state.TargetZone)
	assert.Len(t, state.NetworkAclIds, 1)
	newAssociationIds := reflect.ValueOf(state.OldNetworkAclIds).MapKeys()
	assert.NotEqual(t, "", state.OldNetworkAclIds[newAssociationIds[0].String()])
	assert.NotNil(t, state.AttackExecutionId)

	acls, err = clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	require.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 2)

	// Stop
	// When
	_, err = action.Stop(ctx, &state)

	assert.NoError(t, err)

	acls, err = clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	require.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 1)
}
