// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extelasticache

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	elasticacheNodeGroupTargetId = "com.steadybit.extension_aws.elasticache.node-group"
)

type ElasticacheClusterAttackState struct {
	ReplicationGroupId string
	Account            string
}

type ElasticacheApi interface {
	TestFailover(ctx context.Context, params *elasticache.TestFailoverInput, optFns ...func(*elasticache.Options)) (*elasticache.TestFailoverOutput, error)
	DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

func convertClusterAttackState(request action_kit_api.PrepareActionRequestBody, state *ElasticacheClusterAttackState) error {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.ReplicationGroupId = extutil.MustHaveValue(request.Target.Attributes, "aws.elasticache.replication-group.id")[0]
	return nil
}

func defaultElasticacheClientProvider(account string) (ElasticacheApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return elasticache.NewFromConfig(awsAccount.AwsConfig), nil
}
