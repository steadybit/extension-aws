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

type rdsInstanceRebootAttack struct {
	clientProvider func(account string, region string) (rdsDBInstanceApi, error)
}

var _ action_kit_sdk.Action[RdsInstanceAttackState] = (*rdsInstanceRebootAttack)(nil)

func NewRdsInstanceRebootAttack() action_kit_sdk.Action[RdsInstanceAttackState] {
	return rdsInstanceRebootAttack{defaultInstanceClientProvider}
}

func (f rdsInstanceRebootAttack) NewEmptyState() RdsInstanceAttackState {
	return RdsInstanceAttackState{}
}

func (f rdsInstanceRebootAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.reboot", rdsInstanceTargetId),
		Label:       "Trigger DB Instance Reboot",
		Description: "Triggers rebooting a database instance",
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
		Parameters: []action_kit_api.ActionParameter{
			{
				Advanced:     extutil.Ptr(false),
				DefaultValue: extutil.Ptr("false"),
				Description:  extutil.Ptr("Specifies whether the reboot is conducted through a Multi-AZ failover."),
				Hint: &action_kit_api.ActionHint{
					Content: "You can't enable force failover if the instance isn't configured for Multi-AZ.",
					Type:    action_kit_api.HintInfo,
				},
				Label: "Force Failover",
				Name:  "force-failover",
				Type:  action_kit_api.Boolean,
			},
		},
	}
}

func (f rdsInstanceRebootAttack) Prepare(_ context.Context, state *RdsInstanceAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return nil, convertInstanceAttackState(request, state)
}

func (f rdsInstanceRebootAttack) Start(ctx context.Context, state *RdsInstanceAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account, state.Region)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err)
	}

	input := rds.RebootDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
		ForceFailover:        &state.ForceFailover,
	}
	_, err = client.RebootDBInstance(ctx, &input)
	if err != nil {
		return nil, extension_kit.ToError("Failed to reboot database instance", err)
	}
	return &action_kit_api.StartResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Database instance %s reboot triggered", state.DBInstanceIdentifier),
		}},
	}, nil

}
