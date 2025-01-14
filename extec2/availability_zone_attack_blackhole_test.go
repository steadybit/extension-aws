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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

type clientEC2ApiMock struct {
	mock.Mock
}

func (m *clientEC2ApiMock) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeSubnetsOutput), args.Error(1)
}

func (m *clientEC2ApiMock) DescribeNetworkAcls(ctx context.Context, params *ec2.DescribeNetworkAclsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkAclsOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeNetworkAclsOutput), args.Error(1)
}

func (m *clientEC2ApiMock) CreateNetworkAcl(ctx context.Context, params *ec2.CreateNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.CreateNetworkAclOutput), args.Error(1)
}

func (m *clientEC2ApiMock) CreateNetworkAclEntry(ctx context.Context, params *ec2.CreateNetworkAclEntryInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkAclEntryOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.CreateNetworkAclEntryOutput), args.Error(1)
}

func (m *clientEC2ApiMock) ReplaceNetworkAclAssociation(ctx context.Context, params *ec2.ReplaceNetworkAclAssociationInput, optFns ...func(*ec2.Options)) (*ec2.ReplaceNetworkAclAssociationOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.ReplaceNetworkAclAssociationOutput), args.Error(1)
}

func (m *clientEC2ApiMock) DeleteNetworkAcl(ctx context.Context, params *ec2.DeleteNetworkAclInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkAclOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DeleteNetworkAclOutput), args.Error(1)
}

type clientImdsApiMock struct {
	mock.Mock
}

func (m *clientImdsApiMock) GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*imds.GetInstanceIdentityDocumentOutput), args.Error(1)
}

func TestPrepareBlackhole(t *testing.T) {
	clientEc2 := new(clientEC2ApiMock)
	clientEc2.On("DescribeSubnets", mock.Anything, mock.MatchedBy(func(params *ec2.DescribeSubnetsInput) bool {
		require.Equal(t, extutil.Ptr("availabilityZone"), params.Filters[0].Name)
		require.Equal(t, "eu-west-1a", params.Filters[0].Values[0])
		return true
	})).Return(extutil.Ptr(ec2.DescribeSubnetsOutput{
		Subnets: []types.Subnet{
			{
				SubnetId: extutil.Ptr("subnet-1"),
				VpcId:    extutil.Ptr("vpcId-1"),
			}, {
				SubnetId: extutil.Ptr("subnet-2"),
				VpcId:    extutil.Ptr("vpcId-1"),
			},
		},
	}), nil)
	clientImds := new(clientImdsApiMock)
	clientImds.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(extutil.Ptr(imds.GetInstanceIdentityDocumentOutput{
		InstanceIdentityDocument: imds.InstanceIdentityDocument{
			AccountID: "43",
		},
	}), nil)

	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "",
		clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
			return clientEc2, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
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
	})

	// When
	_, err := action.Prepare(ctx, &state, requestBody)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "42", state.ExtensionAwsAccount)
	assert.Equal(t, "eu-west-1", state.TargetRegion)
	assert.Equal(t, []string{"subnet-1", "subnet-2"}, state.TargetSubnets["vpcId-1"])
	assert.NotNil(t, state.AttackExecutionId)
	clientEc2.AssertExpectations(t)
	clientImds.AssertExpectations(t)
}

func TestShouldNotAttackWhenExtensionIsInTargetAccountId(t *testing.T) {
	// Given
	clientImds := new(clientImdsApiMock)
	clientImds.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything, mock.Anything).Return(extutil.Ptr(imds.GetInstanceIdentityDocumentOutput{
		InstanceIdentityDocument: imds.InstanceIdentityDocument{
			AccountID: "42",
		},
	}), nil)

	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "42",
		clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
			return nil, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
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
	})

	// When
	_, err := action.Prepare(ctx, &state, requestBody)
	// Then
	assert.ErrorContains(t, err, "The extension is running in a protected AWS account ([42]). Attack is disabled to prevent an extension lockout.")
	clientImds.AssertExpectations(t)
}

func TestShouldNotAttackWhenExtensionIsInTargetAccountIdViaStsClient(t *testing.T) {
	// Given
	clientImds := new(clientImdsApiMock)
	clientImds.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "42",
		clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
			return nil, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
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
	})

	// When
	_, err := action.Prepare(ctx, &state, requestBody)
	// Then
	assert.ErrorContains(t, err, "The extension is running in a protected AWS account ([42]). Attack is disabled to prevent an extension lockout.")
	clientImds.AssertExpectations(t)
}

func TestShouldNotAttackWhenExtensionAccountIsUnknown(t *testing.T) {
	// Given
	clientImds := new(clientImdsApiMock)
	clientImds.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "",
		clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
			return nil, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.zone":    {"eu-west-1a"},
				"aws.region":  {"eu-west-1"},
				"aws.account": {"42"},
			},
		}),
		ExecutionContext: extutil.Ptr(action_kit_api.ExecutionContext{
			AgentAwsAccountId: nil,
		}),
	})

	// When
	_, err := action.Prepare(ctx, &state, requestBody)
	// Then
	assert.ErrorContains(t, err, "Could not get AWS Account of the extension. Attack is disabled to prevent an extension lockout.")
	clientImds.AssertExpectations(t)
}

func TestShouldNotAttackWhenAgentAccountIsUnknown(t *testing.T) {
	// Given
	clientImds := new(clientImdsApiMock)
	clientImds.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "",
		clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
			return nil, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
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
	})

	// When
	_, err := action.Prepare(ctx, &state, requestBody)
	// Then
	assert.ErrorContains(t, err, "Could not get AWS Account of the extension. Attack is disabled to prevent an extension lockout.")
	clientImds.AssertExpectations(t)
}

func TestShouldNotAttackWhenAgentIsInTargetAccountId(t *testing.T) {
	// Given
	clientImds := new(clientImdsApiMock)
	clientImds.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything, mock.Anything).Return(extutil.Ptr(imds.GetInstanceIdentityDocumentOutput{
		InstanceIdentityDocument: imds.InstanceIdentityDocument{
			AccountID: "41",
		},
	}), nil)

	ctx := context.Background()
	action := azBlackholeAction{
		extensionRootAccountNumber: "",
		clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
			return nil, clientImds, nil
		}}
	state := action.NewEmptyState()

	requestBody := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"action": "stop",
		},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.zone":    {"eu-west-1a"},
				"aws.region":  {"eu-west-1"},
				"aws.account": {"42"},
			},
		}),
		ExecutionContext: extutil.Ptr(action_kit_api.ExecutionContext{
			AgentAwsAccountId: aws.String("42"),
		}),
	})

	// When
	_, err := action.Prepare(ctx, &state, requestBody)
	// Then
	assert.ErrorContains(t, err, "The agent is running in the same AWS account (42) as the target. Attack is disabled to prevent an agent lockout.")

	clientImds.AssertExpectations(t)
}

func TestStartBlackhole(t *testing.T) {
	// Given
	executionId := uuid.New()
	clientEc2 := new(clientEC2ApiMock)
	clientEc2.On("DescribeNetworkAcls", mock.Anything, mock.MatchedBy(func(params *ec2.DescribeNetworkAclsInput) bool {
		require.Equal(t, extutil.Ptr("association.subnet-id"), params.Filters[0].Name)
		require.Equal(t, "subnet-1", params.Filters[0].Values[0])
		require.Equal(t, "subnet-2", params.Filters[0].Values[1])
		return true
	}), mock.Anything).Return(extutil.Ptr(ec2.DescribeNetworkAclsOutput{
		NetworkAcls: []types.NetworkAcl{
			{
				Associations: []types.NetworkAclAssociation{
					{
						NetworkAclAssociationId: extutil.Ptr("association-1"),
						NetworkAclId:            extutil.Ptr("nacl-1"),
						SubnetId:                extutil.Ptr("subnet-1"),
					}, {
						NetworkAclAssociationId: extutil.Ptr("association-2"),
						NetworkAclId:            extutil.Ptr("nacl-2"),
						SubnetId:                extutil.Ptr("subnet-2"),
					}, {
						NetworkAclAssociationId: extutil.Ptr("association-3"),
						NetworkAclId:            extutil.Ptr("nacl-3"),
						SubnetId:                extutil.Ptr("subnet-3"),
					},
				},
			},
		},
	}), nil)

	clientEc2.On("CreateNetworkAcl", mock.Anything, mock.MatchedBy(func(params *ec2.CreateNetworkAclInput) bool {
		require.Equal(t, extutil.Ptr("vpcId-1"), params.VpcId)
		require.Equal(t, extutil.Ptr("created by steadybit"), params.TagSpecifications[0].Tags[0].Value)
		require.Equal(t, extutil.Ptr("steadybit-attack-execution-id"), params.TagSpecifications[0].Tags[1].Key)
		require.Equal(t, extutil.Ptr(executionId.String()), params.TagSpecifications[0].Tags[1].Value)
		require.Equal(t, extutil.Ptr("steadybit-replaced subnet-1"), params.TagSpecifications[0].Tags[2].Key)
		require.Equal(t, extutil.Ptr("nacl-1"), params.TagSpecifications[0].Tags[2].Value)
		require.Equal(t, extutil.Ptr("steadybit-replaced subnet-2"), params.TagSpecifications[0].Tags[3].Key)
		require.Equal(t, extutil.Ptr("nacl-2"), params.TagSpecifications[0].Tags[3].Value)
		return true
	}), mock.Anything).Return(extutil.Ptr(ec2.CreateNetworkAclOutput{
		NetworkAcl: &types.NetworkAcl{
			NetworkAclId: extutil.Ptr("NEW nacl-4"),
		},
	}), nil)

	clientEc2.On("CreateNetworkAclEntry", mock.Anything, mock.MatchedBy(func(params *ec2.CreateNetworkAclEntryInput) bool {
		require.Equal(t, extutil.Ptr("NEW nacl-4"), params.NetworkAclId)
		return true
	}), mock.Anything).Return(extutil.Ptr(ec2.CreateNetworkAclEntryOutput{}), nil)

	clientEc2.On("ReplaceNetworkAclAssociation", mock.Anything, mock.MatchedBy(func(params *ec2.ReplaceNetworkAclAssociationInput) bool {
		require.Equal(t, extutil.Ptr("NEW nacl-4"), params.NetworkAclId)
		return *params.AssociationId == "association-1"
	}), mock.Anything).Return(extutil.Ptr(ec2.ReplaceNetworkAclAssociationOutput{
		NewAssociationId: extutil.Ptr("NEW association-4"),
	}), nil)

	clientEc2.On("ReplaceNetworkAclAssociation", mock.Anything, mock.MatchedBy(func(params *ec2.ReplaceNetworkAclAssociationInput) bool {
		require.Equal(t, extutil.Ptr("NEW nacl-4"), params.NetworkAclId)
		return *params.AssociationId == "association-2"
	}), mock.Anything).Return(extutil.Ptr(ec2.ReplaceNetworkAclAssociationOutput{
		NewAssociationId: extutil.Ptr("NEW association-5"),
	}), nil)

	ctx := context.Background()
	action := azBlackholeAction{clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
		return clientEc2, nil, nil
	}}

	state := BlackholeState{
		AgentAWSAccount:     "41",
		ExtensionAwsAccount: "43",
		TargetRegion:        "eu-west-1",
		TargetSubnets: map[string][]string{
			"vpcId-1": {"subnet-1", "subnet-2"},
		},
		AttackExecutionId: executionId,
	}

	// When
	_, err := action.Start(ctx, &state)
	// Then
	assert.NoError(t, err)
	assert.Equal(t, "41", state.AgentAWSAccount)
	assert.Equal(t, "43", state.ExtensionAwsAccount)
	assert.Equal(t, []string{"subnet-1", "subnet-2"}, state.TargetSubnets["vpcId-1"])
	assert.Equal(t, "NEW nacl-4", state.NetworkAclIds[0])
	assert.Equal(t, "nacl-1", state.OldNetworkAclIds["NEW association-4"])
	assert.Equal(t, "nacl-2", state.OldNetworkAclIds["NEW association-5"])
	assert.NotNil(t, state.AttackExecutionId)

	clientEc2.AssertExpectations(t)
}

func TestStopBlackhole(t *testing.T) {
	// Given
	executionId := uuid.New()
	clientEc2 := new(clientEC2ApiMock)

	clientEc2.On("DescribeNetworkAcls", mock.Anything, mock.MatchedBy(func(params *ec2.DescribeNetworkAclsInput) bool {
		require.Equal(t, aws.String("tag:Name"), params.Filters[0].Name)
		require.Equal(t, "created by steadybit", params.Filters[0].Values[0])
		require.Equal(t, aws.String("tag:steadybit-attack-execution-id"), params.Filters[1].Name)
		require.Equal(t, executionId.String(), params.Filters[1].Values[0])
		return true
	}), mock.Anything).Return(extutil.Ptr(ec2.DescribeNetworkAclsOutput{
		NetworkAcls: []types.NetworkAcl{
			{
				NetworkAclId: extutil.Ptr("NEW nacl-4"),
				Tags: []types.Tag{
					{
						Key:   extutil.Ptr("steadybit-replaced subnet-1"),
						Value: extutil.Ptr("nacl-1"),
					}, {
						Key:   extutil.Ptr("steadybit-replaced subnet-2"),
						Value: extutil.Ptr("nacl-2"),
					},
				},
				Associations: []types.NetworkAclAssociation{
					{
						NetworkAclAssociationId: extutil.Ptr("NEW association-4"),
						NetworkAclId:            extutil.Ptr("NEW nacl-4"),
						SubnetId:                extutil.Ptr("subnet-1"),
					}, {
						NetworkAclAssociationId: extutil.Ptr("NEW association-5"),
						NetworkAclId:            extutil.Ptr("NEW nacl-4"),
						SubnetId:                extutil.Ptr("subnet-2"),
					},
				},
			},
		},
	}), nil)

	clientEc2.On("ReplaceNetworkAclAssociation", mock.Anything, mock.MatchedBy(func(params *ec2.ReplaceNetworkAclAssociationInput) bool {
		if *params.AssociationId == "NEW association-4" && *params.NetworkAclId == "nacl-1" {
			return true
		}
		if *params.AssociationId == "NEW association-5" && *params.NetworkAclId == "nacl-2" {
			return true
		}
		return false
	}), mock.Anything).Return(extutil.Ptr(ec2.ReplaceNetworkAclAssociationOutput{
		NewAssociationId: extutil.Ptr("New New association-6"),
	}), nil)

	clientEc2.On("DeleteNetworkAcl", mock.Anything, mock.MatchedBy(func(params *ec2.DeleteNetworkAclInput) bool {
		require.Equal(t, extutil.Ptr("NEW nacl-4"), params.NetworkAclId)
		return true
	}), mock.Anything).Return(extutil.Ptr(ec2.DeleteNetworkAclOutput{}), nil)

	ctx := context.Background()
	action := azBlackholeAction{clientProvider: func(account string, region string) (blackholeEC2Api, blackholeImdsApi, error) {
		return clientEc2, nil, nil
	}}

	state := BlackholeState{
		AgentAWSAccount:     "41",
		ExtensionAwsAccount: "43",
		TargetRegion:        "eu-west-1",
		TargetSubnets: map[string][]string{
			"vpcId-1": {"subnet-1", "subnet-2"},
		},
		AttackExecutionId: executionId,
		NetworkAclIds:     []string{"NEW nacl-4"},
		OldNetworkAclIds: map[string]string{
			"NEW association-4": "nacl-1",
			"NEW association-5": "nacl-2",
		},
	}

	// When
	_, err := action.Stop(ctx, &state)
	// Then
	assert.NoError(t, err)

	clientEc2.AssertExpectations(t)
}
