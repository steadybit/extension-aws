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

type elasticacheNodeGroupFailoverAttack struct {
	clientProvider func(account string) (ElasticacheApi, error)
}

var _ action_kit_sdk.Action[ElasticacheClusterAttackState] = (*elasticacheNodeGroupFailoverAttack)(nil)

func NewElasticacheNodeGroupFailoverAttack() action_kit_sdk.Action[ElasticacheClusterAttackState] {
	return elasticacheNodeGroupFailoverAttack{defaultElasticacheClientProvider}
}

func (f elasticacheNodeGroupFailoverAttack) NewEmptyState() ElasticacheClusterAttackState {
	return ElasticacheClusterAttackState{}
}

func (f elasticacheNodeGroupFailoverAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.failover", elasticacheNodeGroupTargetId),
		Label:       "Trigger Failover DB Cluster",
		Description: "Triggers DB cluster failover by promoting a standby instance to primary",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(elasticacheIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: elasticacheNodeGroupTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label: "by elasticache nodegroup id",
					Query: "aws.elasticache.replication-group.node-group.id=\"\"",
				},
			}),
		}),
		Category:    extutil.Ptr("resource"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (f elasticacheNodeGroupFailoverAttack) Prepare(_ context.Context, state *ElasticacheClusterAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	return nil, convertClusterAttackState(request, state)
}

func (f elasticacheNodeGroupFailoverAttack) Start(ctx context.Context, state *ElasticacheClusterAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err)
	}

	input := elasticache.TestFailoverInput{
		NodeGroupId:        &state.NodeGroupID,
		ReplicationGroupId: &state.ReplicationGroupID,
	}
	_, err = client.TestFailover(ctx, &input)
	if err != nil {
		return nil, extension_kit.ToError("Failed to failover elasticache nodegroup", err)
	}
	return &action_kit_api.StartResult{
		Messages: &[]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Elasticache replication group %s for nodegroup %s failover triggered", state.ReplicationGroupID, state.NodeGroupID),
		}},
	}, nil

}
