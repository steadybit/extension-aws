// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelasticache

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type replicationGroupDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*replicationGroupDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*replicationGroupDiscovery)(nil)
)

func NewRdsClusterDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &replicationGroupDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalElasticache)*time.Second),
	)
}

func (r *replicationGroupDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: replicationGroupTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalElasticache)),
		},
	}
}

func (r *replicationGroupDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       replicationGroupTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "Elasticache cluster", Other: "Elasticache clusters"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(elasticacheIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws.elasticache.cluster.id"},
				{Attribute: "aws.elasticache.replication-group.id"},
				{Attribute: "aws.zone"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws.elasticache.cluster.id",
					Direction: "ASC",
				},
			},
		},
	}
}

func (r *replicationGroupDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.elasticache.engine",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS database engine",
				Other: "AWS RDS database engines",
			},
		}, {
			Attribute: "aws.elasticache.cluster",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster",
				Other: "AWS RDS clusters",
			},
		}, {
			Attribute: "aws.elasticache.cluster.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster ID",
				Other: "AWS RDS cluster IDs",
			},
		}, {
			Attribute: "aws.elasticache.cluster.status",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster status",
				Other: "AWS RDS cluster status",
			},
		}, {
			Attribute: "aws.elasticache.cluster.multi-az",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster Multi-AZ",
				Other: "AWS RDS cluster Multi-AZ",
			},
		}, {
			Attribute: "aws.elasticache.cluster.reader",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster reader instance",
				Other: "AWS RDS cluster reader instances",
			},
		}, {
			Attribute: "aws.elasticache.cluster.writer",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster writer instance",
				Other: "AWS RDS cluster writer instances",
			},
		},
	}
}

func (r *replicationGroupDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryAccount(utils.Accounts, getClusterTargetsForAccount, ctx, "rds-cluster")
}

func getClusterTargetsForAccount(account *utils.AwsAccount, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := elasticache.NewFromConfig(account.AwsConfig)
	result, err := getAllReplicationGroups(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover rds-clusters for account %s. If this intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_RDS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllReplicationGroups(ctx context.Context, cacheApi ReplicationGroupApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := elasticache.NewDescribeReplicationGroupsPaginator(cacheApi, &elasticache.DescribeReplicationGroupsInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}

		for _, replicationGroup := range output.ReplicationGroups {
			result = append(result, toClusterTarget(replicationGroup, awsAccountNumber, awsRegion))
		}
	}

	return result, nil
}

func toClusterTarget(replicationGroup types.ReplicationGroup, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	arn := aws.ToString(replicationGroup.ARN)
	label := aws.ToString(replicationGroup.ReplicationGroupId)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.elasticache.preferred-zone"] = []string{aws.ToString(replicationGroup.PreferredAvailabilityZone)}
	attributes["aws.elasticache.engine"] = []string{aws.ToString(replicationGroup.Engine)}
	attributes["aws.elasticache.engine.version"] = []string{aws.ToString(replicationGroup.EngineVersion)}
	attributes["aws.elasticache.cluster.id"] = []string{label}
	attributes["aws.elasticache.cluster.status"] = []string{aws.ToString(replicationGroup.ReplicationGroupStatus)}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: replicationGroupTargetId,
		Attributes: attributes,
	}
}
