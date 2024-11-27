// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type rdsClusterDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*rdsClusterDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*rdsClusterDiscovery)(nil)
)

func NewRdsClusterDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &rdsClusterDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalRds)*time.Second),
	)
}

func (r *rdsClusterDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: rdsClusterTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalRds)),
		},
	}
}

func (r *rdsClusterDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       rdsClusterTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "RDS cluster", Other: "RDS clusters"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(rdsIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.rds.cluster.status"},
				{Attribute: "aws.zone"},
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

func (r *rdsClusterDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.rds.engine",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS database engine",
				Other: "AWS RDS database engines",
			},
		}, {
			Attribute: "aws.rds.cluster",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster",
				Other: "AWS RDS clusters",
			},
		}, {
			Attribute: "aws.rds.cluster.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster ID",
				Other: "AWS RDS cluster IDs",
			},
		}, {
			Attribute: "aws.rds.cluster.status",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster status",
				Other: "AWS RDS cluster status",
			},
		}, {
			Attribute: "aws.rds.cluster.multi-az",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster Multi-AZ",
				Other: "AWS RDS cluster Multi-AZ",
			},
		}, {
			Attribute: "aws.rds.cluster.reader",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster reader instance",
				Other: "AWS RDS cluster reader instances",
			},
		}, {
			Attribute: "aws.rds.cluster.writer",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS cluster writer instance",
				Other: "AWS RDS cluster writer instances",
			},
		},
	}
}

func (r *rdsClusterDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryAccount(utils.Accounts, getClusterTargetsForAccount, ctx, "rds-cluster")
}

func getClusterTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := rds.NewFromConfig(account.AwsConfig)
	result, err := getAllRdsClusters(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover rds-clusters for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_RDS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllRdsClusters(ctx context.Context, rdsApi rdsDBClusterApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := rds.NewDescribeDBClustersPaginator(rdsApi, &rds.DescribeDBClustersInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}

		for _, dbCluster := range output.DBClusters {
			result = append(result, toClusterTarget(dbCluster, awsAccountNumber, awsRegion))
		}
	}

	return result, nil
}

func toClusterTarget(dbCluster types.DBCluster, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	arn := aws.ToString(dbCluster.DBClusterArn)
	label := aws.ToString(dbCluster.DBClusterIdentifier)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.zone"] = dbCluster.AvailabilityZones
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.rds.engine"] = []string{aws.ToString(dbCluster.Engine)}
	attributes["aws.rds.cluster.id"] = []string{label}
	attributes["aws.rds.cluster.status"] = []string{aws.ToString(dbCluster.Status)}
	if dbCluster.MultiAZ != nil {
		attributes["aws.rds.cluster.multi-az"] = []string{fmt.Sprintf("%t", *dbCluster.MultiAZ)}
	}

	for _, tag := range dbCluster.TagList {
		attributes[fmt.Sprintf("aws.rds.cluster.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	for _, member := range dbCluster.DBClusterMembers {
		if member.IsClusterWriter == nil {
			continue
		}

		var key string
		if *member.IsClusterWriter {
			key = "aws.rds.cluster.writer"
		} else {
			key = "aws.rds.cluster.reader"
		}
		attributes[key] = append(attributes[key], aws.ToString(member.DBInstanceIdentifier))
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: rdsClusterTargetId,
		Attributes: attributes,
	}
}
