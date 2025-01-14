/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extec2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func getRequiredApiCallsForSuccessfulStartSubnet() map[string]int {
	return map[string]int{
		"CreateNetworkAcl":             1, //START: only a single NACL
		"CreateNetworkAclEntry":        2, //START: 2 calls per created NACL
		"DescribeNetworkAcls":          1, //START: only a single VPC attacked
		"DescribeSubnets":              0, //PREPARE: not required by subnet attack
		"ReplaceNetworkAclAssociation": 1, //START: only a single subnet
	}
}

func getRequiredApiCallsForSuccessfulRollbackSubnet() map[string]int {
	return map[string]int{
		"DeleteNetworkAcl":             1, //STOP: one call per created NACL
		"DescribeNetworkAcls":          1, //STOP: 1 calls to get all created NACLS
		"ReplaceNetworkAclAssociation": 1, //STOP: only a single subnet
	}
}

func testPrepareAndStartAndStopBlackholeSubnetLocalStack(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	_, createdVpcId := prepareAdditionalVpcWithTwoSubnets(t, clientEc2)

	// Given
	ctx := context.Background()
	PermittedApiCalls = getRequiredApiCallsForSuccessfulStartSubnet()
	PermittedApiCalls["DescribeNetworkAcls"] = PermittedApiCalls["DescribeNetworkAcls"] + 2 //additional calls uses by this test
	PermittedApiCalls["DescribeSubnets"] = PermittedApiCalls["DescribeSubnets"] + 1         //additional calls uses by this test

	subnets, err := clientEc2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{createdVpcId},
			},
		},
	})
	assert.Nil(t, err)
	assert.Len(t, subnets.Subnets, 2)
	subnetId := *subnets.Subnets[0].SubnetId
	action, state, requestBodyPrepare := prepareActionCallSubnetBlackhole(createdVpcId, subnetId, clientEc2, clientImds)

	// When
	_, err = action.Prepare(ctx, &state, requestBodyPrepare)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1", state.TargetRegion)
	assert.Len(t, state.TargetSubnets, 1)
	assert.Len(t, state.TargetSubnets[createdVpcId], 1) //only the attacked subnet
	assert.NotNil(t, state.AttackExecutionId)

	acls, err := clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	assert.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 2) //one per vpc

	// Start
	// When
	attackStartResult, attackStartErr := action.Start(ctx, &state)
	// Then
	assert.NoError(t, attackStartErr)
	if attackStartResult != nil {
		assert.Nil(t, attackStartResult.Error)
	}
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1", state.TargetRegion)
	assert.Len(t, state.NetworkAclIds, 1) //only a single NACL
	newAssociationIds := reflect.ValueOf(state.OldNetworkAclIds).MapKeys()
	assert.NotEqual(t, "", state.OldNetworkAclIds[newAssociationIds[0].String()])
	assert.NotNil(t, state.AttackExecutionId)

	acls, err = clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	assert.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 3) //one per vpc (2 existing / 1 created by steadybit)

	for operation, count := range PermittedApiCalls {
		assert.Zero(t, count, "PermittedApiCalls[%s] should be 0", operation)
	}

	// Stop
	// Given
	PermittedApiCalls = getRequiredApiCallsForSuccessfulRollbackSubnet()
	PermittedApiCalls["DescribeNetworkAcls"] = PermittedApiCalls["DescribeNetworkAcls"] + 1 //additional calls uses by this test

	// When
	attackStopResult, attackStopErr := action.Stop(ctx, &state)
	assert.NoError(t, attackStopErr)
	if attackStopResult != nil {
		assert.Nil(t, attackStopResult.Error)
	}

	acls, err = clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	assert.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 2) //one per vpc

	for operation, count := range PermittedApiCalls {
		assert.Zero(t, count, "PermittedApiCalls[%s] should be 0", operation)
	}
}

func prepareActionCallSubnetBlackhole(vpcId string, subnetId string, clientEc2 *ec2.Client, clientImds *imds.Client) (subnetBlackholeAction, BlackholeState, action_kit_api.PrepareActionRequestBody) {
	action := subnetBlackholeAction{
		extensionRootAccountNumber: "41",
		clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
			return clientEc2, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBodyPrepare := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.vpc.id":        {vpcId},
				"aws.ec2.subnet.id": {subnetId},
				"aws.account":       {"42"},
				"aws.region":        {"eu-west-1"},
			},
		}),
		ExecutionContext: extutil.Ptr(action_kit_api.ExecutionContext{
			AgentAwsAccountId: aws.String("41"),
		}),
		ExecutionId: uuid.New(),
	})
	return action, state, requestBodyPrepare
}
