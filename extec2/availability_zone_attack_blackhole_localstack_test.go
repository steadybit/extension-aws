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
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func getRequiredApiCallsForSuccessfulStart() map[string]int {
	return map[string]int{
		"CreateNetworkAcl":             2, //START: one call per vpc
		"CreateNetworkAclEntry":        4, //START: 2 calls per created NACL
		"DescribeNetworkAcls":          2, //START: 2 calls (one per vpc)
		"DescribeSubnets":              1, //PREPARE: single call to fetch all subnets in the targeted availability zone
		"ReplaceNetworkAclAssociation": 3, //START: 3 calls / one per subnet
	}
}

func getRequiredApiCallsForSuccessfulRollback() map[string]int {
	return map[string]int{
		"DeleteNetworkAcl":             2, //STOP: one call per created NACL
		"DescribeNetworkAcls":          1, //STOP: 1 calls to get all created NACLS
		"ReplaceNetworkAclAssociation": 3, //STOP: 3 calls / one per subnet
	}
}

func testPrepareAndStartAndStopBlackholeLocalStack(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	defaultVpcId, createdVpcId := prepareAdditionalVpcWithTwoSubnets(t, clientEc2)

	// Given
	PermittedApiCalls = getRequiredApiCallsForSuccessfulStart()
	PermittedApiCalls["DescribeNetworkAcls"] = PermittedApiCalls["DescribeNetworkAcls"] + 2 //additional calls uses by this test
	action, state, requestBodyPrepare := prepareActionCallAzBlackhole(clientEc2, clientImds)

	// When
	ctx := context.Background()
	_, err := action.Prepare(ctx, &state, requestBodyPrepare)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1", state.TargetRegion)
	assert.Len(t, state.TargetSubnets, 2)               //default vpc and the one we created
	assert.Len(t, state.TargetSubnets[defaultVpcId], 1) //default vpc has 1 subnet
	assert.Len(t, state.TargetSubnets[createdVpcId], 2) //our vpc with 2 subnets
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
	assert.Len(t, state.NetworkAclIds, 2) //one per vpc
	newAssociationIds := reflect.ValueOf(state.OldNetworkAclIds).MapKeys()
	assert.NotEqual(t, "", state.OldNetworkAclIds[newAssociationIds[0].String()])
	assert.NotNil(t, state.AttackExecutionId)

	acls, err = clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	assert.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 4) //one per vpc (2 existing / 2 created by steadybit)

	for operation, count := range PermittedApiCalls {
		assert.Zero(t, count, "PermittedApiCalls[%s] should be 0", operation)
	}

	// Stop
	// Given
	PermittedApiCalls = getRequiredApiCallsForSuccessfulRollback()
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

/*
 * - Start the attack
 * - Simulate a throttling error during CreateNetworkAcl
 * - Prevent immediate rollback in Start
 * - Simulate a STOP call to roll back the attack by action kit
 */
func testApiThrottlingDuringStartWhileCreatingTheSecondNACL(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	prepareAdditionalVpcWithTwoSubnets(t, clientEc2)

	// Given
	// Simulate an error during CreateNetworkAcl / immediate rollback is also not possible because there are no permitted api calls for the rollback
	PermittedApiCalls = getRequiredApiCallsForSuccessfulStart()
	PermittedApiCalls["CreateNetworkAcl"] = PermittedApiCalls["CreateNetworkAcl"] - 1 //Simulate throttling error
	action, state, requestBodyPrepare := prepareActionCallAzBlackhole(clientEc2, clientImds)

	// When
	ctx := context.Background()
	_, err := action.Prepare(ctx, &state, requestBodyPrepare)
	assert.NoError(t, err)

	// Start
	_, err = action.Start(ctx, &state)
	assert.ErrorContains(t, err, "Failed to create network ACL for VPC")
	assert.False(t, isClean(t, clientEc2))

	// allow rollback api calls
	PermittedApiCalls = map[string]int{
		"DeleteNetworkAcl":             1, //Only one was created
		"DescribeNetworkAcls":          1, //1 calls to get all created NACLS
		"ReplaceNetworkAclAssociation": 1, //Only one was created / assigned
	}

	// Stop
	_, err = action.Stop(ctx, &state)
	assert.NoError(t, err)
	for operation, count := range PermittedApiCalls {
		assert.Zero(t, count, "PermittedApiCalls[%s] should be 0", operation)
	}
	assert.True(t, isClean(t, clientEc2))
}

/*
 * - Start the attack
 * - Simulate a throttling error during ReplaceNetworkAclAssociation
 * - Prevent immediate rollback in Start
 * - Simulate a STOP call to roll back the attack by action kit
 * - Check if a NACL without any associations is cleaned up
 */
func testApiThrottlingDuringStartWhileAssigningTheFirstNACL(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	prepareAdditionalVpcWithTwoSubnets(t, clientEc2)

	// Given
	// prevent stopping the attack and simulate an error during CreateNetworkAcl
	// Simulate an error during CreateNetworkAcl / immediate rollback is also not possible because there are no permitted api calls for the rollback
	PermittedApiCalls = getRequiredApiCallsForSuccessfulStart()
	PermittedApiCalls["ReplaceNetworkAclAssociation"] = 0 //Simulate throttling error
	action, state, requestBodyPrepare := prepareActionCallAzBlackhole(clientEc2, clientImds)

	// When
	ctx := context.Background()
	_, err := action.Prepare(ctx, &state, requestBodyPrepare)
	assert.NoError(t, err)

	// Start
	_, err = action.Start(ctx, &state)
	assert.ErrorContains(t, err, "Failed to replace network ACL associations for VPC")
	assert.False(t, isClean(t, clientEc2))

	// allow rollback api calls
	PermittedApiCalls = map[string]int{
		"DeleteNetworkAcl":             1, //Only one was created
		"DescribeNetworkAcls":          1, //1 calls to get all created NACLS
		"ReplaceNetworkAclAssociation": 0, //No associations were created
	}

	// Stop
	_, err = action.Stop(ctx, &state)
	assert.NoError(t, err)
	for operation, count := range PermittedApiCalls {
		assert.Zero(t, count, "PermittedApiCalls[%s] should be 0", operation)
	}
	assert.True(t, isClean(t, clientEc2))
}

func testApiThrottlingDuringStopWhileReassigningTheOldNACLs(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	prepareAdditionalVpcWithTwoSubnets(t, clientEc2)

	// Given
	PermittedApiCalls = getRequiredApiCallsForSuccessfulStart()
	action, state, requestBodyPrepare := prepareActionCallAzBlackhole(clientEc2, clientImds)
	ctx := context.Background()
	_, err := action.Prepare(ctx, &state, requestBodyPrepare)
	assert.NoError(t, err)
	_, err = action.Start(ctx, &state)
	assert.NoError(t, err)
	assert.False(t, isClean(t, clientEc2))

	// prevent rollback
	PermittedApiCalls = getRequiredApiCallsForSuccessfulRollback()
	PermittedApiCalls["ReplaceNetworkAclAssociation"] = 0 //Simulate throttling error
	_, err = action.Stop(ctx, &state)
	assert.ErrorContains(t, err, "Failed to rollback network ACL association")
	assert.False(t, isClean(t, clientEc2))

	// allow rollback api calls and rollback
	PermittedApiCalls = getRequiredApiCallsForSuccessfulRollback()
	_, err = action.Stop(ctx, &state)
	assert.NoError(t, err)
	for operation, count := range PermittedApiCalls {
		assert.Zero(t, count, "PermittedApiCalls[%s] should be 0", operation)
	}
	assert.True(t, isClean(t, clientEc2))
}

func testApiThrottlingDuringStopWhileDeletingNACLs(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	prepareAdditionalVpcWithTwoSubnets(t, clientEc2)

	// Given
	PermittedApiCalls = getRequiredApiCallsForSuccessfulStart()
	action, state, requestBodyPrepare := prepareActionCallAzBlackhole(clientEc2, clientImds)
	ctx := context.Background()
	_, err := action.Prepare(ctx, &state, requestBodyPrepare)
	assert.NoError(t, err)
	_, err = action.Start(ctx, &state)
	assert.NoError(t, err)
	assert.False(t, isClean(t, clientEc2))

	// prevent rollback (allow re-assignment but prevent delete nacl)
	PermittedApiCalls = getRequiredApiCallsForSuccessfulRollback()
	PermittedApiCalls["DeleteNetworkAcl"] = 0 //Simulate throttling error
	_, err = action.Stop(ctx, &state)
	assert.ErrorContains(t, err, "Failed to delete network acl")
	assert.False(t, isClean(t, clientEc2))

	// allow rollback api calls and rollback
	PermittedApiCalls = getRequiredApiCallsForSuccessfulRollback()
	PermittedApiCalls["ReplaceNetworkAclAssociation"] = 0 //They have been replaced already by the first try.
	_, err = action.Stop(ctx, &state)
	assert.NoError(t, err)
	for operation, count := range PermittedApiCalls {
		assert.Zero(t, count, "PermittedApiCalls[%s] should be 0", operation)
	}
	assert.True(t, isClean(t, clientEc2))
}

func prepareActionCallAzBlackhole(clientEc2 *ec2.Client, clientImds *imds.Client) (azBlackholeAction, BlackholeState, action_kit_api.PrepareActionRequestBody) {
	action := azBlackholeAction{
		extensionRootAccountNumber: "41",
		clientProvider: func(account string, region string, role *string) (blackholeEC2Api, blackholeImdsApi, error) {
			return clientEc2, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBodyPrepare := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.zone":    {"eu-west-1a"},
				"aws.region":  {"eu-west-1"},
				"aws.account": {"42"},
			},
		}),
		ExecutionContext: extutil.Ptr(action_kit_api.ExecutionContext{
			AgentAwsAccountId: aws.String("41"),
		}),
		ExecutionId: uuid.New(),
	})
	return action, state, requestBodyPrepare
}

func isClean(t *testing.T, clientEc2 blackholeEC2Api) bool {
	PermittedApiCalls = map[string]int{
		"DescribeNetworkAcls": 1,
	}
	// Check if the network ACLs created by Steadybit are still in place
	// If they are, the attack was not cleaned up properly
	acls, err := clientEc2.DescribeNetworkAcls(context.Background(), &ec2.DescribeNetworkAclsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{"created by steadybit"},
			},
		},
	})
	assert.NoError(t, err)
	return len(acls.NetworkAcls) == 0
}

func prepareAdditionalVpcWithTwoSubnets(t *testing.T, clientEc2 *ec2.Client) (string, string) {
	PermittedApiCalls = map[string]int{
		// The following calls are made to setup the test environment
		"CreateVpc":           1,
		"CreateSubnet":        2,
		"DescribeVpcs":        1,
		"DescribeNetworkAcls": 2,
	}
	// get default VPC-ID
	vpcs, err := clientEc2.DescribeVpcs(context.Background(), &ec2.DescribeVpcsInput{})
	assert.Nil(t, err)
	if len(vpcs.Vpcs) == 2 {
		log.Debug().Msgf("Reuse VPC from previous test.")
		if *vpcs.Vpcs[0].IsDefault {
			return *vpcs.Vpcs[0].VpcId, *vpcs.Vpcs[1].VpcId
		} else {
			return *vpcs.Vpcs[1].VpcId, *vpcs.Vpcs[0].VpcId
		}
	}
	assert.Len(t, vpcs.Vpcs, 1)
	defaultVpcId := *vpcs.Vpcs[0].VpcId

	// create a new VPC
	vpc, err := clientEc2.CreateVpc(context.Background(), &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.10.0.0/16"),
	})
	assert.Nil(t, err)
	log.Info().Msgf("Created VPC %s", *vpc.Vpc.VpcId)
	createdVpcId := *vpc.Vpc.VpcId

	// create subnets in the new VPC
	_, err = clientEc2.CreateSubnet(context.Background(), &ec2.CreateSubnetInput{
		VpcId:            vpc.Vpc.VpcId,
		CidrBlock:        aws.String("10.10.0.0/21"),
		AvailabilityZone: aws.String("eu-west-1a"),
	})
	assert.Nil(t, err)
	_, err = clientEc2.CreateSubnet(context.Background(), &ec2.CreateSubnetInput{
		VpcId:            vpc.Vpc.VpcId,
		CidrBlock:        aws.String("10.10.8.0/21"),
		AvailabilityZone: aws.String("eu-west-1a"),
	})
	assert.Nil(t, err)

	defaultVpcNacls, err := clientEc2.DescribeNetworkAcls(context.Background(), &ec2.DescribeNetworkAclsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{defaultVpcId},
			},
		},
	})
	assert.Nil(t, err)
	assert.Len(t, defaultVpcNacls.NetworkAcls, 1)
	createdVpcNacls, err := clientEc2.DescribeNetworkAcls(context.Background(), &ec2.DescribeNetworkAclsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{createdVpcId},
			},
		},
	})
	assert.Nil(t, err)
	assert.Len(t, createdVpcNacls.NetworkAcls, 1)
	return defaultVpcId, createdVpcId
}
