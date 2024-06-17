// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extecs

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strconv"
	"strings"
	"time"
)

// pageSize is restricted by AWS ECS API.
const servicePageSize = 10

type ecsServiceDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*ecsServiceDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*ecsServiceDiscovery)(nil)
)

type ecsServiceDiscoveryApi interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

func NewEcsServiceDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &ecsServiceDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalEcsService)*time.Second),
	)
}

func (e *ecsServiceDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: ecsServiceTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEcsService)),
		},
	}
}

func (e *ecsServiceDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id: ecsServiceTargetId,
		Label: discovery_kit_api.PluralLabel{
			One:   "ECS service",
			Other: "ECS services",
		},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(ecsServiceIcon),

		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws-ecs.service.name"},
				{Attribute: "aws-ecs.cluster.name"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws-ecs.service.name",
					Direction: "ASC",
				},
			},
		},
	}
}

func (e *ecsServiceDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws-ecs.cluster.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "ECS cluster name",
				Other: "ECS cluster names",
			},
		},
		{
			Attribute: "aws-ecs.service.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "ECS service name",
				Other: "ECS service names",
			},
		},
	}
}

func (e *ecsServiceDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryAccount(utils.Accounts, getEcsServicesForAccount, ctx, "ecs-service")
}

func getEcsServicesForAccount(account *utils.AwsAccount, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := ecs.NewFromConfig(account.AwsConfig)
	result, err := GetAllEcsServices(account.AwsConfig.Region, account.AccountNumber, client, ctx)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover ecs-service for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ECS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func GetAllEcsServices(awsRegion string, awsAccountNumber string, ecsServiceApi ecsServiceDiscoveryApi, ctx context.Context) ([]discovery_kit_api.Target, error) {
	listClusterOutput, err := ecsServiceApi.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	result := make([]discovery_kit_api.Target, 0, 20)
	for _, clusterArn := range listClusterOutput.ClusterArns {
		targets, err := getAllServicesInCluster(clusterArn, awsRegion, awsAccountNumber, ecsServiceApi, ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, targets...)
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesEcs), nil
}

func getAllServicesInCluster(clusterArn string, awsRegion string, awsAccountNumber string, ecsServiceApi ecsServiceDiscoveryApi, ctx context.Context) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)
	paginator := ecs.NewListServicesPaginator(ecsServiceApi, &ecs.ListServicesInput{
		Cluster: extutil.Ptr(clusterArn),
	})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		serviceArnPages := splitIntoPages(output.ServiceArns, servicePageSize)
		for _, serviceArnPage := range serviceArnPages {
			describeServicesOutput, err := ecsServiceApi.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  extutil.Ptr(clusterArn),
				Services: serviceArnPage,
				Include: []types.ServiceField{
					types.ServiceFieldTags,
				},
			})
			if err != nil {
				return nil, err
			}

			for _, service := range describeServicesOutput.Services {
				result = append(result, toServiceTarget(service, awsAccountNumber, awsRegion))
			}
		}
	}
	return result, nil
}

func toServiceTarget(service types.Service, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{awsRegion}

	clusterArn := aws.ToString(service.ClusterArn)
	attributes["aws-ecs.cluster.arn"] = []string{clusterArn}
	clusterName := strings.SplitAfter(clusterArn, "/")[1]
	attributes["aws-ecs.cluster.name"] = []string{clusterName}

	serviceArn := aws.ToString(service.ServiceArn)
	attributes["aws-ecs.service.arn"] = []string{serviceArn}
	serviceName := aws.ToString(service.ServiceName)
	attributes["aws-ecs.service.name"] = []string{serviceName}
	attributes["aws-ecs.service.desired-count"] = []string{strconv.Itoa(int(service.DesiredCount))}

	for _, tag := range service.Tags {
		attributes[fmt.Sprintf("aws-ecs.service.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	return discovery_kit_api.Target{
		Id:         serviceArn,
		Label:      serviceName,
		TargetType: ecsServiceTargetId,
		Attributes: attributes,
	}
}
