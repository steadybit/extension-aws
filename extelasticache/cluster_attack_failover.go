// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelasticache

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type replicationGroupFailoverAttack struct {
	clientProvider func(account string) (ReplicationGroupApi, error)
}

var _ action_kit_sdk.Action[ReplicationGroupAttackState] = (*replicationGroupFailoverAttack)(nil)

func NewRdsClusterFailoverAttack() action_kit_sdk.Action[ReplicationGroupAttackState] {
	return replicationGroupFailoverAttack{defaultReplicationGroupClientProvider}
}

func (f replicationGroupFailoverAttack) NewEmptyState() ReplicationGroupAttackState {
	return ReplicationGroupAttackState{}
}

func (f replicationGroupFailoverAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.failover", replicationGroupTargetId),
		Label:       "Trigger Elasticache Failover",
		Description: "Triggers cache node group failover",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(elasticacheIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: replicationGroupTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label: "by cache cluster id",
					Query: "aws.elasticache.cluster.id=\"\"",
				},
			}),
		}),
		Category:    extutil.Ptr("resource"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (f replicationGroupFailoverAttack) Prepare(_ context.Context, state *ReplicationGroupAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return nil, convertReplicationGroupAttackState(request, state)
}

func (f replicationGroupFailoverAttack) Start(ctx context.Context, state *ReplicationGroupAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize elasticache client for AWS account %s", state.Account), err)
	}

	input := elasticache.TestFailoverInput{
		ReplicationGroupId: &state.ReplicationGroupId,
		NodeGroupId:        &state.ReplicationGroupId,
	}
	_, err = client.TestFailover(ctx, &input)
	if err != nil {
		return nil, extension_kit.ToError("Failed to failover cache cluster", err)
	}
	return &action_kit_api.StartResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Cache cluster %s failover triggered", state.ReplicationGroupId),
		}},
	}, nil

}
