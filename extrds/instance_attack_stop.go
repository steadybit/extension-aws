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

type rdsInstanceStopAttack struct {
	clientProvider func(account string) (rdsDBInstanceApi, error)
}

var (
	_ action_kit_sdk.Action[RdsInstanceAttackState] = (*rdsInstanceStopAttack)(nil)
)

func NewRdsInstanceStopAttack() action_kit_sdk.Action[RdsInstanceAttackState] {
	return rdsInstanceStopAttack{defaultInstanceClientProvider}
}

func (f rdsInstanceStopAttack) NewEmptyState() RdsInstanceAttackState {
	return RdsInstanceAttackState{}
}

func (f rdsInstanceStopAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.stop", rdsInstanceTargetId),
		Label:       "Trigger DB Instance Stop",
		Description: "Triggers stopping a DB instance",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(rdsIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: rdsInstanceTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label: "by rds instance id",
					Query: "aws.rds.instance.id=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("RDS"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
		Hint: &action_kit_api.ActionHint{
			Content: "This action will not perform a rollback. You need to take care about restarting.\n\nStopping a DB instance may take several minutes or hours in case of issues.",
			Type:    action_kit_api.HintWarning,
		},
	}
}

func (f rdsInstanceStopAttack) Prepare(_ context.Context, state *RdsInstanceAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return nil, convertInstanceAttackState(request, state)
}

func (f rdsInstanceStopAttack) Start(ctx context.Context, state *RdsInstanceAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err)
	}

	input := rds.StopDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}

	_, err = client.StopDBInstance(ctx, &input)
	if err != nil {
		return nil, extension_kit.ToError("Failed to stop database instance", err)
	}

	return &action_kit_api.StartResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Database instance %s stopped", state.DBInstanceIdentifier),
		}},
	}, nil
}
