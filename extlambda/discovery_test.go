/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extlambda

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	tagtypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type lambdaClientMock struct {
	mock.Mock
}

func (m *lambdaClientMock) ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lambda.ListFunctionsOutput), args.Error(1)
}

type tagClientMock struct {
	mock.Mock
}

func (m *tagClientMock) GetResources(ctx context.Context, params *resourcegroupstaggingapi.GetResourcesInput, optFns ...func(*resourcegroupstaggingapi.Options)) (*resourcegroupstaggingapi.GetResourcesOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resourcegroupstaggingapi.GetResourcesOutput), args.Error(1)
}

type lambdaDiscoveryEc2UtilMock struct {
	mock.Mock
}

func (m *lambdaDiscoveryEc2UtilMock) GetVpcName(awsAccountNumber string, region string, vpcId string) string {
	args := m.Called(awsAccountNumber, region, vpcId)
	return args.Get(0).(string)
}

func Test_getAllAwsLambdaFunctions(t *testing.T) {
	lambdaApi := new(lambdaClientMock)
	tagApi := new(tagClientMock)
	ec2util := new(lambdaDiscoveryEc2UtilMock)
	listedFunction := lambda.ListFunctionsOutput{
		Functions: []types.FunctionConfiguration{
			{
				Architectures: []types.Architecture{"x86_64"},
				CodeSize:      1024,
				Description:   extutil.Ptr("description"),
				Environment: extutil.Ptr(types.EnvironmentResponse{
					Variables: map[string]string{
						"FAILURE_INJECTION_PARAM": "env-fip",
					},
				}),
				VpcConfig: extutil.Ptr(types.VpcConfigResponse{
					VpcId: extutil.Ptr("vpc-123"),
				}),
				FunctionArn:  extutil.Ptr("arn"),
				FunctionName: extutil.Ptr("name"),
				LastModified: extutil.Ptr("last-modified"),
				MasterArn:    extutil.Ptr("master-arn"),
				MemorySize:   extutil.Ptr(int32(1024)),
				PackageType:  "package-type",
				RevisionId:   extutil.Ptr("revision-id"),
				Role:         extutil.Ptr("role"),
				Runtime:      "runtime",
				Timeout:      extutil.Ptr(int32(10)),
				Version:      extutil.Ptr("version"),
			},
		},
	}
	lambdaApi.On("ListFunctions", mock.Anything, mock.Anything, mock.Anything).Return(&listedFunction, nil)

	tags := resourcegroupstaggingapi.GetResourcesOutput{
		ResourceTagMappingList: []tagtypes.ResourceTagMapping{
			{
				ResourceARN: extutil.Ptr("arn"),
				Tags: []tagtypes.Tag{
					{
						Key:   extutil.Ptr("Example"),
						Value: extutil.Ptr("Tag123"),
					},
				},
			},
		},
	}
	tagApi.On("GetResources", mock.Anything, mock.Anything, mock.Anything).Return(&tags, nil)

	ec2util.On("GetVpcName", mock.Anything, mock.Anything, mock.Anything).Return("vpc-123-name")

	// When
	targets, err := getAllAwsLambdaFunctions(context.Background(), lambdaApi, tagApi, ec2util, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{
				Key:    "Example",
				Values: []string{"Tag123"},
			},
		},
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Len(t, targets, 1)

	target := targets[0]
	assert.Equal(t, lambdaTargetID, target.TargetType)
	assert.Equal(t, "name", target.Label)
	assert.Equal(t, "arn", target.Id)
	assert.Equal(t, 22, len(target.Attributes))
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"name"}, target.Attributes["aws.lambda.function-name"])
	assert.Equal(t, []string{"env-fip"}, target.Attributes["aws.lambda.failure-injection-param"])
	assert.Equal(t, []string{"Tag123"}, target.Attributes["aws.lambda.label.example"])
	assert.Equal(t, []string{"vpc-123"}, target.Attributes["aws.vpc.id"])
	assert.Equal(t, []string{"vpc-123-name"}, target.Attributes["aws.vpc.name"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
}

func Test_getAllAwsLambdaFunctions_withPagination(t *testing.T) {
	// Given
	mockedApi := new(lambdaClientMock)
	tagApi := new(tagClientMock)
	ec2util := new(lambdaDiscoveryEc2UtilMock)

	withMarker := mock.MatchedBy(func(arg *lambda.ListFunctionsInput) bool {
		return arg.Marker != nil
	})
	withoutMarker := mock.MatchedBy(func(arg *lambda.ListFunctionsInput) bool {
		return arg.Marker == nil
	})
	mockedApi.On("ListFunctions", mock.Anything, withoutMarker, mock.Anything).Return(&lambda.ListFunctionsOutput{
		NextMarker: discovery_kit_api.Ptr("marker"),
		Functions: []types.FunctionConfiguration{
			{
				FunctionArn: extutil.Ptr("arn1"),
			},
		},
	}, nil)
	mockedApi.On("ListFunctions", mock.Anything, withMarker, mock.Anything).Return(&lambda.ListFunctionsOutput{
		Functions: []types.FunctionConfiguration{
			{
				FunctionArn: extutil.Ptr("arn2"),
			},
		},
	}, nil)
	tags := resourcegroupstaggingapi.GetResourcesOutput{
		ResourceTagMappingList: []tagtypes.ResourceTagMapping{
			{
				ResourceARN: extutil.Ptr("arn"),
				Tags: []tagtypes.Tag{
					{
						Key:   extutil.Ptr("Example"),
						Value: extutil.Ptr("Tag123"),
					},
				},
			},
		},
	}
	tagApi.On("GetResources", mock.Anything, mock.Anything, mock.Anything).Return(&tags, nil)

	// When
	targets, err := getAllAwsLambdaFunctions(context.Background(), mockedApi, tagApi, ec2util, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, nil, err)
	assert.Len(t, targets, 2)
	assert.Equal(t, "arn1", targets[0].Id)
	assert.Equal(t, "arn2", targets[1].Id)
}

func Test_getAllAwsLambdaFunctions_withError(t *testing.T) {
	// Given
	clientApi := new(lambdaClientMock)
	tagApi := new(tagClientMock)
	ec2util := new(lambdaDiscoveryEc2UtilMock)
	clientApi.On("ListFunctions", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("error"))

	// When
	_, err := getAllAwsLambdaFunctions(context.Background(), clientApi, tagApi, ec2util, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
	})
	assert.Equal(t, "error", err.Error())
}
