// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extdynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	aastypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type ddbApiMock struct {
	mock.Mock
}

func (m *ddbApiMock) ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.ListTablesOutput), args.Error(1)
}

func (m *ddbApiMock) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.DescribeTableOutput), args.Error(1)
}

func (m *ddbApiMock) DescribeContinuousBackups(ctx context.Context, params *dynamodb.DescribeContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.DescribeContinuousBackupsOutput), args.Error(1)
}

func (m *ddbApiMock) DescribeTimeToLive(ctx context.Context, params *dynamodb.DescribeTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.DescribeTimeToLiveOutput), args.Error(1)
}

func (m *ddbApiMock) ListTagsOfResource(ctx context.Context, params *dynamodb.ListTagsOfResourceInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTagsOfResourceOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.ListTagsOfResourceOutput), args.Error(1)
}

func (m *ddbApiMock) UpdateTable(ctx context.Context, params *dynamodb.UpdateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateTableOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.UpdateTableOutput), args.Error(1)
}

type aasApiMock struct {
	mock.Mock
}

func (m *aasApiMock) DescribeScalableTargets(ctx context.Context, params *applicationautoscaling.DescribeScalableTargetsInput, optFns ...func(*applicationautoscaling.Options)) (*applicationautoscaling.DescribeScalableTargetsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*applicationautoscaling.DescribeScalableTargetsOutput), args.Error(1)
}

func TestGetAllTables(t *testing.T) {
	ddb := new(ddbApiMock)
	aas := new(aasApiMock)

	ddb.On("ListTables", mock.Anything, mock.Anything).Return(&dynamodb.ListTablesOutput{TableNames: []string{"orders"}}, nil)
	ddb.On("DescribeTable", mock.Anything, mock.MatchedBy(func(p *dynamodb.DescribeTableInput) bool {
		return aws.ToString(p.TableName) == "orders"
	})).Return(&dynamodb.DescribeTableOutput{
		Table: &types.TableDescription{
			TableName:                 aws.String("orders"),
			TableArn:                  aws.String("arn:aws:dynamodb:us-east-1:42:table/orders"),
			BillingModeSummary:        &types.BillingModeSummary{BillingMode: types.BillingModeProvisioned},
			TableClassSummary:         &types.TableClassSummary{TableClass: types.TableClassStandard},
			DeletionProtectionEnabled: aws.Bool(false),
			StreamSpecification: &types.StreamSpecification{
				StreamEnabled:  aws.Bool(true),
				StreamViewType: types.StreamViewTypeNewAndOldImages,
			},
			SSEDescription: &types.SSEDescription{SSEType: types.SSETypeKms},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndexDescription{
				{
					IndexName: aws.String("by-customer"),
					ProvisionedThroughput: &types.ProvisionedThroughputDescription{
						ReadCapacityUnits:  aws.Int64(5),
						WriteCapacityUnits: aws.Int64(5),
					},
				},
				{
					IndexName: aws.String("by-status"),
					ProvisionedThroughput: &types.ProvisionedThroughputDescription{
						ReadCapacityUnits:  aws.Int64(0),
						WriteCapacityUnits: aws.Int64(0),
					},
				},
			},
			LocalSecondaryIndexes: []types.LocalSecondaryIndexDescription{
				{IndexName: aws.String("by-time")},
			},
			Replicas: []types.ReplicaDescription{
				{RegionName: aws.String("eu-west-1")},
				{RegionName: aws.String("us-east-1")},
			},
			GlobalTableVersion: aws.String("2019.11.21"),
		},
	}, nil)
	ddb.On("DescribeContinuousBackups", mock.Anything, mock.Anything).Return(&dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &types.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &types.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: types.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil)
	ddb.On("DescribeTimeToLive", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTimeToLiveOutput{
		TimeToLiveDescription: &types.TimeToLiveDescription{TimeToLiveStatus: types.TimeToLiveStatusDisabled},
	}, nil)
	ddb.On("ListTagsOfResource", mock.Anything, mock.Anything).Return(&dynamodb.ListTagsOfResourceOutput{
		Tags: []types.Tag{{Key: aws.String("application"), Value: aws.String("Demo")}},
	}, nil)

	aas.On("DescribeScalableTargets", mock.Anything, mock.Anything).Return(&applicationautoscaling.DescribeScalableTargetsOutput{
		ScalableTargets: []aastypes.ScalableTarget{
			{
				ResourceId:        aws.String("table/orders"),
				ScalableDimension: aastypes.ScalableDimensionDynamoDBTableReadCapacityUnits,
				MinCapacity:       aws.Int32(5),
				MaxCapacity:       aws.Int32(40000),
			},
			// Note: no write autoscaling on table
			{
				ResourceId:        aws.String("table/orders/index/by-customer"),
				ScalableDimension: aastypes.ScalableDimensionDynamoDBIndexReadCapacityUnits,
				MinCapacity:       aws.Int32(5),
				MaxCapacity:       aws.Int32(20000),
			},
		},
	}, nil)

	targets, err := getAllTables(context.Background(), ddb, aas, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    aws.String("arn:role"),
		TagFilters:    []extConfig.TagFilter{{Key: "application", Values: []string{"Demo"}}},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	tgt := targets[0]
	assert.Equal(t, tableTargetId, tgt.TargetType)
	assert.Equal(t, "orders", tgt.Label)
	assert.Equal(t, []string{"orders"}, tgt.Attributes["aws.dynamodb.table.name"])
	assert.Equal(t, []string{"PROVISIONED"}, tgt.Attributes["aws.dynamodb.billing-mode"])
	assert.Equal(t, []string{"STANDARD"}, tgt.Attributes["aws.dynamodb.table-class"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws.dynamodb.deletion-protection"])
	assert.Equal(t, []string{"true"}, tgt.Attributes["aws.dynamodb.pitr.enabled"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws.dynamodb.ttl.enabled"])
	assert.Equal(t, []string{"true"}, tgt.Attributes["aws.dynamodb.streams.enabled"])
	assert.Equal(t, []string{"NEW_AND_OLD_IMAGES"}, tgt.Attributes["aws.dynamodb.streams.view-type"])
	assert.Equal(t, []string{"KMS"}, tgt.Attributes["aws.dynamodb.sse.type"])
	assert.Equal(t, []string{"2"}, tgt.Attributes["aws.dynamodb.gsi.count"])
	assert.Equal(t, []string{"by-customer", "by-status"}, tgt.Attributes["aws.dynamodb.gsi.names"])
	assert.Equal(t, []string{"PROVISIONED"}, tgt.Attributes["aws.dynamodb.gsi.by-customer.billing-mode"])
	assert.Equal(t, []string{"PAY_PER_REQUEST"}, tgt.Attributes["aws.dynamodb.gsi.by-status.billing-mode"])
	assert.Equal(t, []string{"1"}, tgt.Attributes["aws.dynamodb.lsi.count"])
	assert.Equal(t, []string{"eu-west-1", "us-east-1"}, tgt.Attributes["aws.dynamodb.global-table.replicas"])
	assert.Equal(t, []string{"2019.11.21"}, tgt.Attributes["aws.dynamodb.global-table.version"])

	// Autoscaling table-level: read enabled with min/max, write disabled
	assert.Equal(t, []string{"true"}, tgt.Attributes["aws.dynamodb.autoscaling.read.enabled"])
	assert.Equal(t, []string{"5"}, tgt.Attributes["aws.dynamodb.autoscaling.read.min"])
	assert.Equal(t, []string{"40000"}, tgt.Attributes["aws.dynamodb.autoscaling.read.max"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws.dynamodb.autoscaling.write.enabled"])

	// Autoscaling per-GSI
	assert.Equal(t, []string{"true"}, tgt.Attributes["aws.dynamodb.autoscaling.gsi.by-customer.read.enabled"])
	assert.Equal(t, []string{"5"}, tgt.Attributes["aws.dynamodb.autoscaling.gsi.by-customer.read.min"])
	assert.Equal(t, []string{"20000"}, tgt.Attributes["aws.dynamodb.autoscaling.gsi.by-customer.read.max"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws.dynamodb.autoscaling.gsi.by-customer.write.enabled"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws.dynamodb.autoscaling.gsi.by-status.read.enabled"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws.dynamodb.autoscaling.gsi.by-status.write.enabled"])

	assert.Equal(t, []string{"Demo"}, tgt.Attributes["aws.dynamodb.label.application"])
	assert.Equal(t, []string{"arn:role"}, tgt.Attributes["extension-aws.discovered-by-role"])
}

func TestGetAllTablesPayPerRequestDefaultsSse(t *testing.T) {
	ddb := new(ddbApiMock)
	aas := new(aasApiMock)
	ddb.On("ListTables", mock.Anything, mock.Anything).Return(&dynamodb.ListTablesOutput{TableNames: []string{"events"}}, nil)
	ddb.On("DescribeTable", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTableOutput{
		Table: &types.TableDescription{
			TableName:          aws.String("events"),
			TableArn:           aws.String("arn:t"),
			BillingModeSummary: &types.BillingModeSummary{BillingMode: types.BillingModePayPerRequest},
		},
	}, nil)
	ddb.On("DescribeContinuousBackups", mock.Anything, mock.Anything).Return(&dynamodb.DescribeContinuousBackupsOutput{}, nil)
	ddb.On("DescribeTimeToLive", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTimeToLiveOutput{}, nil)
	ddb.On("ListTagsOfResource", mock.Anything, mock.Anything).Return(&dynamodb.ListTagsOfResourceOutput{}, nil)
	aas.On("DescribeScalableTargets", mock.Anything, mock.Anything).Return(&applicationautoscaling.DescribeScalableTargetsOutput{}, nil)

	targets, err := getAllTables(context.Background(), ddb, aas, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"PAY_PER_REQUEST"}, targets[0].Attributes["aws.dynamodb.billing-mode"])
	assert.Equal(t, []string{"AWS_OWNED"}, targets[0].Attributes["aws.dynamodb.sse.type"], "should default to AWS_OWNED when SSE absent")
	assert.Equal(t, []string{"false"}, targets[0].Attributes["aws.dynamodb.streams.enabled"], "should default streams.enabled to false")
	assert.Equal(t, []string{"0"}, targets[0].Attributes["aws.dynamodb.gsi.count"])
}

func TestGetAllTablesContinuesWhenAutoscalingFails(t *testing.T) {
	ddb := new(ddbApiMock)
	aas := new(aasApiMock)
	ddb.On("ListTables", mock.Anything, mock.Anything).Return(&dynamodb.ListTablesOutput{TableNames: []string{"a"}}, nil)
	ddb.On("DescribeTable", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTableOutput{
		Table: &types.TableDescription{TableName: aws.String("a"), TableArn: aws.String("arn:a"), BillingModeSummary: &types.BillingModeSummary{BillingMode: types.BillingModeProvisioned}},
	}, nil)
	ddb.On("DescribeContinuousBackups", mock.Anything, mock.Anything).Return(&dynamodb.DescribeContinuousBackupsOutput{}, nil)
	ddb.On("DescribeTimeToLive", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTimeToLiveOutput{}, nil)
	ddb.On("ListTagsOfResource", mock.Anything, mock.Anything).Return(&dynamodb.ListTagsOfResourceOutput{}, nil)
	aas.On("DescribeScalableTargets", mock.Anything, mock.Anything).Return(nil, errors.New("permission denied"))

	targets, err := getAllTables(context.Background(), ddb, aas, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	assert.Equal(t, []string{"false"}, targets[0].Attributes["aws.dynamodb.autoscaling.read.enabled"])
	assert.Equal(t, []string{"false"}, targets[0].Attributes["aws.dynamodb.autoscaling.write.enabled"])
}

func TestGetAllTablesError(t *testing.T) {
	ddb := new(ddbApiMock)
	aas := new(aasApiMock)
	aas.On("DescribeScalableTargets", mock.Anything, mock.Anything).Return(&applicationautoscaling.DescribeScalableTargetsOutput{}, nil)
	ddb.On("ListTables", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	_, err := getAllTables(context.Background(), ddb, aas, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.EqualError(t, err, "expected")
}
