// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extmsk

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	tagTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type mskClusterDiscovery struct {
}

var (
	_                                  discovery_kit_sdk.TargetDescriber    = (*mskClusterDiscovery)(nil)
	_                                  discovery_kit_sdk.AttributeDescriber = (*mskClusterDiscovery)(nil)
	missingPermissionTagsAlreadyLogged                                      = false
)

func NewMskClusterDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &mskClusterDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalMsk)*time.Second),
	)
}

func (r *mskClusterDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: mskBrokerTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalMsk)),
		},
	}
}

func (r *mskClusterDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       mskBrokerTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "MSK Broker", Other: "MSK Brokers"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(mskIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.msk.cluster.state"},
				{Attribute: "aws.msk.cluster.version"},
				{Attribute: "aws.msk.cluster.broker.kafka-version"},
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

func (r *mskClusterDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.msk.cluster.broker.arn",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK broker arn",
				Other: "AWS MSK broker arns",
			},
		},
		{
			Attribute: "aws.msk.cluster.arn",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK cluster arn",
				Other: "AWS MSK cluster arns",
			},
		}, {
			Attribute: "aws.msk.cluster.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK cluster name",
				Other: "AWS MSK cluster names",
			},
		},
		{
			Attribute: "aws.msk.cluster.version",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK cluster version",
				Other: "AWS MSK cluster versions",
			},
		},
		{
			Attribute: "aws.msk.cluster.state",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK cluster state",
				Other: "AWS MSK cluster states",
			},
		}, {
			Attribute: "aws.msk.broker.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK broker id",
				Other: "AWS MSK broker ids",
			},
		},
		{
			Attribute: "aws.msk.broker.ebs-storage",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK broker ebs storage volume",
				Other: "AWS MSK cluster ebs storage volumes",
			},
		},
		{
			Attribute: "aws.msk.broker.ebs-throughput",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK broker ebs storage volume",
				Other: "AWS MSK cluster ebs storage volumes",
			},
		}, {
			Attribute: "aws.msk.broker.instance-type",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK broker instance type",
				Other: "AWS MSK broker instance types",
			},
		}, {
			Attribute: "aws.msk.broker.kafka-version",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK broker kafka version",
				Other: "AWS MSK broker kafka versions",
			},
		}, {
			Attribute: "aws.msk.broker.zookeeper-version",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS MSK zookeeper version",
				Other: "AWS MSK zookeeper versions",
			},
		},
	}
}

func (r *mskClusterDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getClusterTargetsForAccount, ctx, "msk-cluster")
}

func getClusterTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := kafka.NewFromConfig(account.AwsConfig)
	tagsClient := resourcegroupstaggingapi.NewFromConfig(account.AwsConfig)
	result, err := getAllMskClusters(ctx, client, tagsClient, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover msk-clusters for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_MSK=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllMskClusters(ctx context.Context, mskApi MskApi, tagsClient resourcegroupstaggingapi.GetResourcesAPIClient, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := kafka.NewListClustersV2Paginator(mskApi, &kafka.ListClustersV2Input{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}

		for _, mskCluster := range output.ClusterInfoList {
			//You can't list the nodes for a cluster that is in the CREATING state.
			if mskCluster.State == types.ClusterStateCreating {
				log.Warn().Msg("You can't list the nodes for a cluster that is in the CREATING state")
			} else {
				paginatorNodes := kafka.NewListNodesPaginator(mskApi, &kafka.ListNodesInput{ClusterArn: mskCluster.ClusterArn})
				for paginatorNodes.HasMorePages() {
					outputNode, err := paginatorNodes.NextPage(ctx)
					if err != nil {
						return result, err
					}

					if len(outputNode.NodeInfoList) == 0 {
						continue
					}

					tagsResponses, err := getTags(ctx, outputNode, tagsClient, len(account.TagFilters) > 0)
					if err != nil {
						return nil, err
					}

					for _, node := range outputNode.NodeInfoList {
						var tags []tagTypes.Tag
						for _, tagsResponse := range tagsResponses {
							if *node.NodeARN == *tagsResponse.ResourceARN {
								tags = tagsResponse.Tags
							}
						}
						if utils.MatchesTagFilter(tags, account.TagFilters) {
							result = append(result, toClusterTarget(mskCluster, node, tags, account.AccountNumber, account.Region, account.AssumeRole))
						}
					}
				}
			}
		}
	}

	return result, nil
}

func getTags(ctx context.Context, output *kafka.ListNodesOutput, tagsClient resourcegroupstaggingapi.GetResourcesAPIClient, tagsRequired bool) ([]tagTypes.ResourceTagMapping, error) {
	arns := make([]string, 0, len(output.NodeInfoList))
	for _, node := range output.NodeInfoList {
		arns = append(arns, *node.NodeARN)
	}
	tagsResult, tagsErr := tagsClient.GetResources(ctx, &resourcegroupstaggingapi.GetResourcesInput{
		ResourceARNList: arns,
	})
	if tagsErr != nil {
		if tagsRequired {
			return nil, tagsErr
		} else if !missingPermissionTagsAlreadyLogged {
			log.Warn().Err(tagsErr).Msg("Error fetching tags for msk nodes. Tags will be missing in the discovery.")
			missingPermissionTagsAlreadyLogged = true
		}
	}
	var tags []tagTypes.ResourceTagMapping
	if tagsResult != nil && tagsResult.ResourceTagMappingList != nil {
		tags = tagsResult.ResourceTagMappingList
	}
	return tags, nil
}

func toClusterTarget(cluster types.Cluster, node types.NodeInfo, tags []tagTypes.Tag, awsAccountNumber string, awsRegion string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(node.NodeARN)
	label := *cluster.ClusterName + "-" + fmt.Sprintf("%v", *node.BrokerNodeInfo.BrokerId)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.msk.cluster.broker.arn"] = []string{arn}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.msk.cluster.arn"] = []string{*cluster.ClusterArn}
	attributes["aws.msk.cluster.name"] = []string{label}
	attributes["aws.msk.cluster.version"] = []string{*cluster.CurrentVersion}
	attributes["aws.msk.cluster.state"] = []string{string(cluster.State)}
	attributes["aws.msk.cluster.broker.id"] = []string{fmt.Sprintf("%v", *node.BrokerNodeInfo.BrokerId)}

	if cluster.Provisioned != nil && cluster.Provisioned.CurrentBrokerSoftwareInfo != nil {
		if cluster.Provisioned.CurrentBrokerSoftwareInfo.KafkaVersion != nil {
			attributes["aws.msk.cluster.broker.kafka-version"] = []string{*cluster.Provisioned.CurrentBrokerSoftwareInfo.KafkaVersion}
		}
	}
	if cluster.Provisioned != nil && cluster.Provisioned.BrokerNodeGroupInfo != nil {
		if cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo != nil && cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo.EbsStorageInfo != nil && cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo.EbsStorageInfo.VolumeSize != nil {
			attributes["aws.msk.cluster.broker.ebs-storage"] = []string{strconv.Itoa(int(*cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo.EbsStorageInfo.VolumeSize))}
		}
		if cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo != nil && cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo.EbsStorageInfo != nil && cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo.EbsStorageInfo.ProvisionedThroughput != nil && cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo.EbsStorageInfo.ProvisionedThroughput.VolumeThroughput != nil {
			attributes["aws.msk.cluster.broker.ebs-throughput"] = []string{strconv.Itoa(int(*cluster.Provisioned.BrokerNodeGroupInfo.StorageInfo.EbsStorageInfo.ProvisionedThroughput.VolumeThroughput))}
		}
		if cluster.Provisioned.BrokerNodeGroupInfo.InstanceType != nil {
			attributes["aws.msk.cluster.broker.instance-type"] = []string{*cluster.Provisioned.BrokerNodeGroupInfo.InstanceType}
		}
	}

	if node.ZookeeperNodeInfo != nil && node.ZookeeperNodeInfo.ZookeeperVersion != nil {
		attributes["aws.msk.cluster.broker.zookeeper-version"] = []string{*node.ZookeeperNodeInfo.ZookeeperVersion}
	}

	for _, tag := range tags {
		attributes[fmt.Sprintf("aws.msk.cluster.broker.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: mskBrokerTargetId,
		Attributes: attributes,
	}
}
