/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extaz

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

func testPrepareAndStartAndStopBlackholeLocalStack(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client) {
	// Prepare
	defaultVpcId, createdVpcId := prepareAdditionalVpcWithTwoSubnets(t, clientEc2)

	// Given
	PermittedApiCalls = map[string]int{
		"CreateNetworkAcl":             2,         //START: one call per vpc
		"CreateNetworkAclEntry":        4,         //START: 2 calls per created NACL
		"DeleteNetworkAcl":             2,         //STOP: one call per created NACL
		"DescribeNetworkAcls":          2 + 1 + 3, // START: 2 calls (one per vpc) and STOP: 1 calls to get all created NACLS + 3 calls from this test
		"DescribeSubnets":              1,         // PREPARE: single call to fetch all subnets in the targeted availability zone
		"DescribeTags":                 2,         //STOP: one call per created NACL
		"ReplaceNetworkAclAssociation": 3 + 3,     //START: 3 calls / one per subnet and STOP 3 calls / one per subnet
	}
	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "41",
		clientProvider: func(account string) (azBlackholeEC2Api, azBlackholeImdsApi, error) {
			return clientEc2, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBodyPrepare := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
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
	})

	// When
	_, err := action.Prepare(ctx, &state, requestBodyPrepare)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1a", state.TargetZone)
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
	assert.Equal(t, "eu-west-1a", state.TargetZone)
	assert.Len(t, state.NetworkAclIds, 2) //one per vpc
	newAssociationIds := reflect.ValueOf(state.OldNetworkAclIds).MapKeys()
	assert.NotEqual(t, "", state.OldNetworkAclIds[newAssociationIds[0].String()])
	assert.NotNil(t, state.AttackExecutionId)

	acls, err = clientEc2.DescribeNetworkAcls(ctx, &ec2.DescribeNetworkAclsInput{})
	assert.Nil(t, err)
	assert.Len(t, acls.NetworkAcls, 4) //one per vpc (2 existing / 2 created by steadybit)

	// Stop
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

func prepareAdditionalVpcWithTwoSubnets(t *testing.T, clientEc2 *ec2.Client) (string, string) {
	PermittedApiCalls = map[string]int{
		// The following calls are made to setup the test environment
		"CreateVpc":           1,
		"CreateSubnet":        2,
		"DescribeSubnets":     1,
		"DescribeVpcs":        1,
		"DescribeNetworkAcls": 2,
	}
	// get default VPC-ID
	vpcs, err := clientEc2.DescribeVpcs(context.Background(), &ec2.DescribeVpcsInput{})
	assert.Nil(t, err)
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
