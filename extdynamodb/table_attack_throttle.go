// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extdynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type tableThrottleAttack struct {
	clientProvider func(account string, region string, role *string) (DynamodbApi, error)
}

var (
	_ action_kit_sdk.Action[TableThrottleAttackState]         = (*tableThrottleAttack)(nil)
	_ action_kit_sdk.ActionWithStop[TableThrottleAttackState] = (*tableThrottleAttack)(nil)
)

func NewTableThrottleAttack() action_kit_sdk.ActionWithStop[TableThrottleAttackState] {
	return &tableThrottleAttack{
		clientProvider: func(account string, region string, role *string) (DynamodbApi, error) {
			awsAccess, err := utils.GetAwsAccess(account, region, role)
			if err != nil {
				return nil, err
			}
			return dynamodb.NewFromConfig(awsAccess.AwsConfig), nil
		},
	}
}

func (a *tableThrottleAttack) NewEmptyState() TableThrottleAttackState {
	return TableThrottleAttackState{}
}

func (a *tableThrottleAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    fmt.Sprintf("%s.throttle", tableTargetId),
		Label: "Change Read/Write Table Capacity",
		Description: "Temporarily lowers a PROVISIONED-mode DynamoDB table's read + write capacity (and the capacity of each GSI) to force ProvisionedThroughputExceededException. " +
			"Validates client retry / backoff logic and circuit breakers. Original capacities are restored on stop. " +
			"Tables in PAY_PER_REQUEST mode are not supported (no fixed capacity to lower). If the table has Application Auto Scaling, the autoscaler will eventually scale back up; " +
			"the attack still exercises the throttle path during the autoscaler's reaction window.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    new(dynamodbIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: tableTargetId,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by table name",
					Description: new("Find DynamoDB table by name"),
					Query:       "aws.dynamodb.table.name=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("DynamoDB"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("How long the lowered capacity stays in effect. Originals restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("60s"),
				Order:        new(1),
				Required:     new(true),
			},
			{
				Name:         "readCapacity",
				Label:        "Target read capacity (RCU)",
				Description:  new("New ReadCapacityUnits for the table and each GSI. Use 1 for aggressive throttling."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("1"),
				Order:        new(2),
				Required:     new(true),
				MinValue:     new(1),
			},
			{
				Name:         "writeCapacity",
				Label:        "Target write capacity (WCU)",
				Description:  new("New WriteCapacityUnits for the table and each GSI. Use 1 for aggressive throttling."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("1"),
				Order:        new(3),
				Required:     new(true),
				MinValue:     new(1),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *tableThrottleAttack) Prepare(ctx context.Context, state *TableThrottleAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.TableName = extutil.MustHaveValue(request.Target.Attributes, "aws.dynamodb.table.name")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")

	read := int64(extutil.ToInt(request.Config["readCapacity"]))
	write := int64(extutil.ToInt(request.Config["writeCapacity"]))
	if read < 1 || write < 1 {
		return nil, extension_kit.ToError("readCapacity and writeCapacity must be >= 1.", nil)
	}
	state.TargetReadCapacity = read
	state.TargetWriteCapacity = write

	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize DynamoDB client for AWS account %s", state.Account), err)
	}
	out, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(state.TableName)})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe DynamoDB table %s", state.TableName), err)
	}
	if out.Table == nil {
		return nil, extension_kit.ToError(fmt.Sprintf("DynamoDB table %s not found", state.TableName), nil)
	}

	// PAY_PER_REQUEST tables have no fixed capacity; reject.
	if out.Table.BillingModeSummary != nil && out.Table.BillingModeSummary.BillingMode == ddbtypes.BillingModePayPerRequest {
		return nil, extension_kit.ToError(fmt.Sprintf("DynamoDB table %s is in PAY_PER_REQUEST mode; throttle attack only supports PROVISIONED tables.", state.TableName), nil)
	}

	if pt := out.Table.ProvisionedThroughput; pt != nil {
		state.OriginalReadCapacity = aws.ToInt64(pt.ReadCapacityUnits)
		state.OriginalWriteCapacity = aws.ToInt64(pt.WriteCapacityUnits)
	}
	state.OriginalGsiCapacity = make(map[string][2]int64)
	for _, gsi := range out.Table.GlobalSecondaryIndexes {
		if gsi.IndexName == nil || gsi.ProvisionedThroughput == nil {
			continue
		}
		state.OriginalGsiCapacity[*gsi.IndexName] = [2]int64{
			aws.ToInt64(gsi.ProvisionedThroughput.ReadCapacityUnits),
			aws.ToInt64(gsi.ProvisionedThroughput.WriteCapacityUnits),
		}
	}

	// Reject up front if the requested capacity matches the current capacity for both the table and
	// every GSI. AWS UpdateTable rejects no-op throughput changes with ValidationException; surfacing
	// that as a Prepare error is clearer than letting it fail mid-experiment at Start.
	if state.TargetReadCapacity == state.OriginalReadCapacity && state.TargetWriteCapacity == state.OriginalWriteCapacity {
		needsGsiChange := false
		for _, rw := range state.OriginalGsiCapacity {
			if rw[0] != state.TargetReadCapacity || rw[1] != state.TargetWriteCapacity {
				needsGsiChange = true
				break
			}
		}
		if !needsGsiChange {
			return nil, extension_kit.ToError(fmt.Sprintf("Target capacity (RCU=%d, WCU=%d) already equals current capacity for table %s; pick different values to actually throttle.", state.TargetReadCapacity, state.TargetWriteCapacity, state.TableName), nil)
		}
	}

	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Will throttle table %s to RCU=%d WCU=%d (was RCU=%d WCU=%d) and %d GSI(s) to the same values.", state.TableName, state.TargetReadCapacity, state.TargetWriteCapacity, state.OriginalReadCapacity, state.OriginalWriteCapacity, len(state.OriginalGsiCapacity)),
		}}),
	}, nil
}

func (a *tableThrottleAttack) Start(ctx context.Context, state *TableThrottleAttackState) (*action_kit_api.StartResult, error) {
	if err := a.updateCapacity(ctx, state, state.TargetReadCapacity, state.TargetWriteCapacity, gsiTarget(state, state.TargetReadCapacity, state.TargetWriteCapacity)); err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to throttle DynamoDB table %s", state.TableName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Throttled table %s to RCU=%d WCU=%d", state.TableName, state.TargetReadCapacity, state.TargetWriteCapacity),
		}}),
	}, nil
}

func (a *tableThrottleAttack) Stop(ctx context.Context, state *TableThrottleAttackState) (*action_kit_api.StopResult, error) {
	if err := a.updateCapacity(ctx, state, state.OriginalReadCapacity, state.OriginalWriteCapacity, state.OriginalGsiCapacity); err != nil {
		log.Error().Err(err).Msgf("Failed to restore DynamoDB table %s capacity", state.TableName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore capacity on DynamoDB table %s", state.TableName), err)
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Restored table %s capacity to RCU=%d WCU=%d", state.TableName, state.OriginalReadCapacity, state.OriginalWriteCapacity),
		}}),
	}, nil
}

// gsiTarget builds a per-GSI capacity map setting every GSI to the same (read,write) tuple.
func gsiTarget(state *TableThrottleAttackState, read, write int64) map[string][2]int64 {
	out := make(map[string][2]int64, len(state.OriginalGsiCapacity))
	for name := range state.OriginalGsiCapacity {
		out[name] = [2]int64{read, write}
	}
	return out
}

func (a *tableThrottleAttack) updateCapacity(ctx context.Context, state *TableThrottleAttackState, read, write int64, gsiCapacity map[string][2]int64) error {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return err
	}
	input := &dynamodb.UpdateTableInput{
		TableName: aws.String(state.TableName),
		ProvisionedThroughput: &ddbtypes.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(read),
			WriteCapacityUnits: aws.Int64(write),
		},
	}
	if len(gsiCapacity) > 0 {
		updates := make([]ddbtypes.GlobalSecondaryIndexUpdate, 0, len(gsiCapacity))
		for name, rw := range gsiCapacity {
			n := name
			updates = append(updates, ddbtypes.GlobalSecondaryIndexUpdate{
				Update: &ddbtypes.UpdateGlobalSecondaryIndexAction{
					IndexName: &n,
					ProvisionedThroughput: &ddbtypes.ProvisionedThroughput{
						ReadCapacityUnits:  aws.Int64(rw[0]),
						WriteCapacityUnits: aws.Int64(rw[1]),
					},
				},
			})
		}
		input.GlobalSecondaryIndexUpdates = updates
	}
	_, err = client.UpdateTable(ctx, input)
	return err
}
