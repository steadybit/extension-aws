// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
)

func RegisterClusterDiscoveryHandlers() {
	exthttp.RegisterHttpHandler("/rds/cluster/discovery", exthttp.GetterAsHandler(getRdsClusterDiscoveryDescription))
	exthttp.RegisterHttpHandler("/rds/cluster/discovery/target-description", exthttp.GetterAsHandler(getRdsClusterTargetDescription))
	exthttp.RegisterHttpHandler("/rds/cluster/discovery/attribute-descriptions", exthttp.GetterAsHandler(getRdsClusterAttributeDescriptions))
	exthttp.RegisterHttpHandler("/rds/cluster/discovery/discovered-targets", getRdsClusterDiscoveryResults)
}

func getRdsClusterDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         rdsClusterTargetId,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/rds/cluster/discovery/discovered-targets",
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func getRdsClusterTargetDescription() discovery_kit_api.TargetDescription {
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

func getRdsClusterAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
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
			},
		},
	}
}

func getRdsClusterDiscoveryResults(w http.ResponseWriter, r *http.Request, _ []byte) {
	targets, err := utils.ForEveryAccount(utils.Accounts, getClusterTargetsForAccount, mergeTargets, make([]discovery_kit_api.Target, 0, 100), r.Context())
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect RDS cluster information", err))
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
	}
}

func getClusterTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := rds.NewFromConfig(account.AwsConfig)
	targets, err := GetAllRdsClusters(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover rds-clusters for account %s. If this intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_RDS=true. Details: %s", account.AccountNumber, re.Error())
			return extutil.Ptr([]discovery_kit_api.Target{}), nil
		}
		return nil, err
	}
	return &targets, nil
}

func GetAllRdsClusters(ctx context.Context, rdsApi rdsDBClusterApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
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

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: rdsClusterTargetId,
		Attributes: attributes,
	}
}
