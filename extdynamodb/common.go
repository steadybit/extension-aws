// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extdynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/steadybit/extension-aws/v2/utils"
)

const (
	dynamodbIcon  = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmM0LjQyIDAgOCAxLjc5IDggNHM"
	tableTargetId = "com.steadybit.extension_aws.dynamodb.table"
)

type DynamodbApi interface {
	dynamodb.ListTablesAPIClient
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	DescribeContinuousBackups(ctx context.Context, params *dynamodb.DescribeContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error)
	DescribeTimeToLive(ctx context.Context, params *dynamodb.DescribeTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error)
	ListTagsOfResource(ctx context.Context, params *dynamodb.ListTagsOfResourceInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTagsOfResourceOutput, error)
	UpdateTable(ctx context.Context, params *dynamodb.UpdateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateTableOutput, error)
}

// TableThrottleAttackState captures the original provisioned capacity for a PROVISIONED-mode table
// (and each of its GSIs) so we can restore on Stop.
type TableThrottleAttackState struct {
	TableName        string
	Account          string
	Region           string
	DiscoveredByRole *string

	OriginalReadCapacity  int64
	OriginalWriteCapacity int64
	// GSI name → [read, write]
	OriginalGsiCapacity map[string][2]int64

	TargetReadCapacity  int64
	TargetWriteCapacity int64
}

type AppAutoScalingApi interface {
	applicationautoscaling.DescribeScalableTargetsAPIClient
}

func defaultDynamodbClientProvider(account string, region string, role *string) (DynamodbApi, AppAutoScalingApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, nil, err
	}
	return dynamodb.NewFromConfig(awsAccess.AwsConfig), applicationautoscaling.NewFromConfig(awsAccess.AwsConfig), nil
}
