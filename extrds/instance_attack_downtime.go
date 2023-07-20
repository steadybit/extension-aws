// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type rdsInstanceDowntimeAttack struct {
	clientProvider func(account string) (rdsDBInstanceApi, error)
}

var (
	_ action_kit_sdk.Action[RdsInstanceAttackState]         = (*rdsInstanceDowntimeAttack)(nil)
	_ action_kit_sdk.ActionWithStop[RdsInstanceAttackState] = (*rdsInstanceDowntimeAttack)(nil)
)

func NewRdsInstanceDowntime() action_kit_sdk.Action[RdsInstanceAttackState] {
	return rdsInstanceDowntimeAttack{defaultClientProvider}
}

func (f rdsInstanceDowntimeAttack) NewEmptyState() RdsInstanceAttackState {
	return RdsInstanceAttackState{}
}

func (f rdsInstanceDowntimeAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.downtime", rdsTargetId),
		Label:       "Apply Instance Downtime",
		Description: "Stops a DB instance and restarts it after a given time",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(rdsIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: rdsTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label: "by rds instance id",
					Query: "aws.rds.instance.id=\"\"",
				},
			}),
		}),
		Category:    extutil.Ptr("resource"),
		TimeControl: action_kit_api.External,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("300s"),
			},
		},
	}
}

func (f rdsInstanceDowntimeAttack) Prepare(_ context.Context, state *RdsInstanceAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return nil, convertAttackState(request, state)
}

func (f rdsInstanceDowntimeAttack) Start(ctx context.Context, state *RdsInstanceAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err))
	}

	input := rds.StopDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}
	_, err = client.StopDBInstance(ctx, &input)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to stop database instance", err))
	}
	return &action_kit_api.StartResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Database instance %s stopped", state.DBInstanceIdentifier),
		}},
	}, nil
}

func (f rdsInstanceDowntimeAttack) Stop(ctx context.Context, state *RdsInstanceAttackState) (*action_kit_api.StopResult, error) {
	client, err := f.clientProvider(state.Account)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err))
	}

	input := rds.StartDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}
	_, err = client.StartDBInstance(ctx, &input)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to start database instance", err))
	}
	return &action_kit_api.StopResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Database instance %s started", state.DBInstanceIdentifier),
		}},
	}, nil
}
