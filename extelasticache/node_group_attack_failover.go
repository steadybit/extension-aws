// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelasticache

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type elasticacheNodeGroupFailoverAttack struct {
	clientProvider func(account string, region string, role *string) (ElasticacheApi, error)
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
		Label:       "Trigger Failover",
		Description: "Triggers nodegroup failover by promoting a replica node to primary",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(elasticacheIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: elasticacheNodeGroupTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "elasticache nodegroup id",
					Description: extutil.Ptr("Find node groups by replication group id and node group id"),
					Query:       "aws.elasticache.replication-group.id=\"\" and aws.elasticache.replication-group.node-group.id=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("AWS"),
		Category:    extutil.Ptr("ElastiCache"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (f elasticacheNodeGroupFailoverAttack) Prepare(_ context.Context, state *ElasticacheClusterAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	state.ReplicationGroupID = extutil.MustHaveValue(request.Target.Attributes, "aws.elasticache.replication-group.id")[0]
	state.NodeGroupID = extutil.MustHaveValue(request.Target.Attributes, "aws.elasticache.replication-group.node-group.id")[0]
	return nil, nil
}

func (f elasticacheNodeGroupFailoverAttack) Start(ctx context.Context, state *ElasticacheClusterAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Elasticache client for AWS account %s", state.Account), err)
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
