// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelasticache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type elasticacheReplicationGroupDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*elasticacheReplicationGroupDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*elasticacheReplicationGroupDiscovery)(nil)
)

func NewElasticacheReplicationGroupDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &elasticacheReplicationGroupDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalElasticacheReplicationGroup)*time.Second),
	)
}

func (r *elasticacheReplicationGroupDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: elasticacheNodeGroupTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalElasticacheReplicationGroup)),
		},
	}
}

func (r *elasticacheReplicationGroupDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       elasticacheNodeGroupTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "Elasticache", Other: "Elasticaches"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(elasticacheIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.elasticache.replication-group.status"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "steadybit.label",
					Direction: "ASC",
				},
			},
		},
	}
}

func (r *elasticacheReplicationGroupDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.elasticache.replication-group.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group ID",
				Other: "AWS Elasticache replication group IDs",
			},
		}, {
			Attribute: "aws.elasticache.replication-group.status",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group status",
				Other: "AWS Elasticache replication group status",
			},
		}, {
			Attribute: "aws.elasticache.replication-group.multi-az",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group Multi-AZ",
				Other: "AWS Elasticache replication group Multi-AZ",
			},
		}, {
			Attribute: "aws.elasticache.replication-group.cluster-mode",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group cluster mode",
				Other: "AWS Elasticache replication group cluster modes",
			},
		}, {
			Attribute: "aws.elasticache.replication-group.cache-node-type",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group cache node type",
				Other: "AWS Elasticache replication group cache node types",
			},
		},
		{
			Attribute: "aws.elasticache.replication-group.automatic-failover",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group automatic failover setting",
				Other: "AWS Elasticache replication group automatic failover settings",
			},
		}, {
			Attribute: "aws.elasticache.replication-group.node-group.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group node group id",
				Other: "AWS Elasticache replication group node group ids",
			},
		}, {
			Attribute: "aws.elasticache.replication-group.node-group.status",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Elasticache replication group node group status",
				Other: "AWS Elasticache replication group node group status",
			},
		},
	}
}

func (r *elasticacheReplicationGroupDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryAccount(utils.Accounts, getClusterTargetsForAccount, ctx, "replication-group")
}

func getClusterTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := elasticache.NewFromConfig(account.AwsConfig)
	result, err := getAllElasticacheReplicationGroups(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover elasticache replication groups for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ELASTICACHE=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllElasticacheReplicationGroups(ctx context.Context, elasticacheApi ElasticacheApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := elasticache.NewDescribeReplicationGroupsPaginator(elasticacheApi, &elasticache.DescribeReplicationGroupsInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}

		for _, replicationGroup := range output.ReplicationGroups {
			for _, nodeGroup := range replicationGroup.NodeGroups {
				result = append(result, toNodeGroupTarget(nodeGroup, replicationGroup, awsAccountNumber, awsRegion))
			}
		}
	}

	return result, nil
}

func toNodeGroupTarget(nodegroup types.NodeGroup, replicationGroup types.ReplicationGroup, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.elasticache.replication-group.id"] = []string{aws.ToString(replicationGroup.ReplicationGroupId)}
	attributes["aws.elasticache.replication-group.status"] = []string{aws.ToString(replicationGroup.Status)}
	attributes["aws.elasticache.replication-group.automatic-failover"] = []string{string(replicationGroup.AutomaticFailover)}
	attributes["aws.elasticache.replication-group.cluster-mode"] = []string{string(replicationGroup.ClusterMode)}
	attributes["aws.elasticache.replication-group.multi-az"] = []string{string(replicationGroup.MultiAZ)}
	attributes["aws.elasticache.replication-group.cache-node-type"] = []string{aws.ToString(replicationGroup.CacheNodeType)}
	attributes["aws.elasticache.replication-group.node-group.id"] = []string{aws.ToString(nodegroup.NodeGroupId)}
	attributes["aws.elasticache.replication-group.node-group.status"] = []string{aws.ToString(nodegroup.Status)}

	return discovery_kit_api.Target{
		Id:         aws.ToString(replicationGroup.ReplicationGroupId) + "-" + aws.ToString(nodegroup.NodeGroupId),
		Label:      aws.ToString(replicationGroup.ReplicationGroupId) + "-" + aws.ToString(nodegroup.NodeGroupId),
		TargetType: elasticacheNodeGroupTargetId,
		Attributes: attributes,
	}
}
