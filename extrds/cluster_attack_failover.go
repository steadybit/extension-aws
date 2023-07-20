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

type rdsClusterFailoverAttack struct {
	clientProvider func(account string) (rdsDBClusterApi, error)
}

var _ action_kit_sdk.Action[RdsClusterAttackState] = (*rdsClusterFailoverAttack)(nil)

func NewRdsClusterFailoverAttack() action_kit_sdk.Action[RdsClusterAttackState] {
	return rdsClusterFailoverAttack{defaultClusterClientProvider}
}

func (f rdsClusterFailoverAttack) NewEmptyState() RdsClusterAttackState {
	return RdsClusterAttackState{}
}

func (f rdsClusterFailoverAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.failover", rdsClusterTargetId),
		Label:       "Trigger Failover DB Cluster",
		Description: "Triggers DB cluster failover by promoting  a standby instance to primary",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(rdsIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: rdsClusterTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label: "by rds cluster id",
					Query: "aws.rds.cluster.id=\"\"",
				},
			}),
		}),
		Category:    extutil.Ptr("resource"),
		TimeControl: action_kit_api.Instantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (f rdsClusterFailoverAttack) Prepare(_ context.Context, state *RdsClusterAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return nil, convertClusterAttackState(request, state)
}

func (f rdsClusterFailoverAttack) Start(ctx context.Context, state *RdsClusterAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err)
	}

	input := rds.FailoverDBClusterInput{
		DBClusterIdentifier: &state.DBClusterIdentifier,
	}
	_, err = client.FailoverDBCluster(ctx, &input)
	if err != nil {
		return nil, extension_kit.ToError("Failed to failover database cluster", err)
	}
	return &action_kit_api.StartResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Database cluster %s failovered", state.DBClusterIdentifier),
		}},
	}, nil

}
