// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extelasticache

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
)

const (
	replicationGroupTargetId = "com.steadybit.extension_aws.ealsticache.replication-group"
)

type ReplicationGroupAttackState struct {
	ReplicationGroupId string
	NodeGroupIds       []string
	Account            string
}

type ReplicationGroupApi interface {
	TestFailover(ctx context.Context, params *elasticache.TestFailoverInput, optFns ...func(*elasticache.Options)) (*elasticache.TestFailoverOutput, error)
	DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

func convertReplicationGroupAttackState(request action_kit_api.PrepareActionRequestBody, state *ReplicationGroupAttackState) error {
	replicationGroupId := request.Target.Attributes["aws.elasticache.replication-group.id"]
	if len(replicationGroupId) == 0 {
		return extension_kit.ToError("Target is missing the 'aws.elasticache.replication-group.id' target attribute.", nil)
	}

	nodeGroupIds := request.Target.Attributes["aws.elasticache.replication-group.node-groups"]
	if len(nodeGroupIds) == 0 {
		return extension_kit.ToError("Target is missing the 'aws.elasticache.replication-group.node-group' target attribute.", nil)
	}

	account := request.Target.Attributes["aws.account"]
	if len(account) == 0 {
		return extension_kit.ToError("Target is missing the 'aws.account' target attribute.", nil)
	}

	state.Account = account[0]
	state.ReplicationGroupId = replicationGroupId[0]
	state.NodeGroupIds = nodeGroupIds
	return nil
}
func defaultReplicationGroupClientProvider(account string) (ReplicationGroupApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return elasticache.NewFromConfig(awsAccount.AwsConfig), nil
}
