// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extdynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newThrottleRequest(read, write int) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{"readCapacity": read, "writeCapacity": write},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":                      {"42"},
				"aws.region":                       {"us-east-1"},
				"aws.dynamodb.table.name":          {"my-table"},
				"extension-aws.discovered-by-role": {"arn:role"},
			},
		}),
	})
}

func newThrottleAttack(api *ddbApiMock) tableThrottleAttack {
	return tableThrottleAttack{
		clientProvider: func(account string, region string, role *string) (DynamodbApi, error) { return api, nil },
	}
}

func TestThrottlePrepareCapturesTableAndGsiCapacity(t *testing.T) {
	api := new(ddbApiMock)
	api.On("DescribeTable", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTableOutput{
		Table: &ddbtypes.TableDescription{
			TableName:          aws.String("my-table"),
			BillingModeSummary: &ddbtypes.BillingModeSummary{BillingMode: ddbtypes.BillingModeProvisioned},
			ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
				ReadCapacityUnits:  aws.Int64(100),
				WriteCapacityUnits: aws.Int64(50),
			},
			GlobalSecondaryIndexes: []ddbtypes.GlobalSecondaryIndexDescription{
				{
					IndexName: aws.String("gsi-1"),
					ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
						ReadCapacityUnits:  aws.Int64(20),
						WriteCapacityUnits: aws.Int64(10),
					},
				},
			},
		},
	}, nil)
	a := newThrottleAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newThrottleRequest(1, 1))
	require.NoError(t, err)
	assert.Equal(t, int64(100), state.OriginalReadCapacity)
	assert.Equal(t, int64(50), state.OriginalWriteCapacity)
	assert.Equal(t, [2]int64{20, 10}, state.OriginalGsiCapacity["gsi-1"])
	assert.Equal(t, int64(1), state.TargetReadCapacity)
	assert.Equal(t, int64(1), state.TargetWriteCapacity)
}

func TestThrottlePrepareRejectsNoOpCapacityChange(t *testing.T) {
	api := new(ddbApiMock)
	api.On("DescribeTable", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTableOutput{
		Table: &ddbtypes.TableDescription{
			TableName:          aws.String("my-table"),
			BillingModeSummary: &ddbtypes.BillingModeSummary{BillingMode: ddbtypes.BillingModeProvisioned},
			ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
				ReadCapacityUnits:  aws.Int64(1),
				WriteCapacityUnits: aws.Int64(1),
			},
		},
	}, nil)
	a := newThrottleAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newThrottleRequest(1, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already equals current capacity")
}

func TestThrottlePrepareAllowsNoOpWhenGsiNeedsChange(t *testing.T) {
	api := new(ddbApiMock)
	api.On("DescribeTable", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTableOutput{
		Table: &ddbtypes.TableDescription{
			TableName:          aws.String("my-table"),
			BillingModeSummary: &ddbtypes.BillingModeSummary{BillingMode: ddbtypes.BillingModeProvisioned},
			ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
				ReadCapacityUnits:  aws.Int64(1),
				WriteCapacityUnits: aws.Int64(1),
			},
			GlobalSecondaryIndexes: []ddbtypes.GlobalSecondaryIndexDescription{
				{
					IndexName: aws.String("gsi-1"),
					ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
						ReadCapacityUnits:  aws.Int64(10),
						WriteCapacityUnits: aws.Int64(10),
					},
				},
			},
		},
	}, nil)
	a := newThrottleAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newThrottleRequest(1, 1))
	require.NoError(t, err)
}

func TestThrottlePrepareRejectsPayPerRequest(t *testing.T) {
	api := new(ddbApiMock)
	api.On("DescribeTable", mock.Anything, mock.Anything).Return(&dynamodb.DescribeTableOutput{
		Table: &ddbtypes.TableDescription{
			TableName:          aws.String("my-table"),
			BillingModeSummary: &ddbtypes.BillingModeSummary{BillingMode: ddbtypes.BillingModePayPerRequest},
		},
	}, nil)
	a := newThrottleAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newThrottleRequest(1, 1))
	require.Error(t, err)
}

func TestThrottleStartUpdatesTableAndGsis(t *testing.T) {
	api := new(ddbApiMock)
	api.On("UpdateTable", mock.Anything, mock.MatchedBy(func(p *dynamodb.UpdateTableInput) bool {
		require.Equal(t, "my-table", aws.ToString(p.TableName))
		require.NotNil(t, p.ProvisionedThroughput)
		require.Equal(t, int64(1), aws.ToInt64(p.ProvisionedThroughput.ReadCapacityUnits))
		require.Equal(t, int64(2), aws.ToInt64(p.ProvisionedThroughput.WriteCapacityUnits))
		require.Len(t, p.GlobalSecondaryIndexUpdates, 1)
		require.Equal(t, "gsi-1", aws.ToString(p.GlobalSecondaryIndexUpdates[0].Update.IndexName))
		require.Equal(t, int64(1), aws.ToInt64(p.GlobalSecondaryIndexUpdates[0].Update.ProvisionedThroughput.ReadCapacityUnits))
		require.Equal(t, int64(2), aws.ToInt64(p.GlobalSecondaryIndexUpdates[0].Update.ProvisionedThroughput.WriteCapacityUnits))
		return true
	})).Return(&dynamodb.UpdateTableOutput{}, nil)
	a := newThrottleAttack(api)
	state := TableThrottleAttackState{
		TableName: "my-table", Account: "42", Region: "us-east-1",
		OriginalReadCapacity: 100, OriginalWriteCapacity: 50,
		OriginalGsiCapacity: map[string][2]int64{"gsi-1": {20, 10}},
		TargetReadCapacity:  1, TargetWriteCapacity: 2,
	}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	api.AssertExpectations(t)
}

func TestThrottleStopRestoresOriginalCapacity(t *testing.T) {
	api := new(ddbApiMock)
	api.On("UpdateTable", mock.Anything, mock.MatchedBy(func(p *dynamodb.UpdateTableInput) bool {
		require.Equal(t, int64(100), aws.ToInt64(p.ProvisionedThroughput.ReadCapacityUnits))
		require.Equal(t, int64(50), aws.ToInt64(p.ProvisionedThroughput.WriteCapacityUnits))
		require.Len(t, p.GlobalSecondaryIndexUpdates, 1)
		require.Equal(t, int64(20), aws.ToInt64(p.GlobalSecondaryIndexUpdates[0].Update.ProvisionedThroughput.ReadCapacityUnits))
		require.Equal(t, int64(10), aws.ToInt64(p.GlobalSecondaryIndexUpdates[0].Update.ProvisionedThroughput.WriteCapacityUnits))
		return true
	})).Return(&dynamodb.UpdateTableOutput{}, nil)
	a := newThrottleAttack(api)
	state := TableThrottleAttackState{
		TableName:            "my-table",
		OriginalReadCapacity: 100, OriginalWriteCapacity: 50,
		OriginalGsiCapacity: map[string][2]int64{"gsi-1": {20, 10}},
		TargetReadCapacity:  1, TargetWriteCapacity: 1,
	}
	_, err := a.Stop(context.Background(), &state)
	require.NoError(t, err)
	api.AssertExpectations(t)
}

func TestThrottleStartForwardsError(t *testing.T) {
	api := new(ddbApiMock)
	api.On("UpdateTable", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	a := newThrottleAttack(api)
	state := TableThrottleAttackState{TableName: "t", TargetReadCapacity: 1, TargetWriteCapacity: 1}
	_, err := a.Start(context.Background(), &state)
	assert.Error(t, err)
}
