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
	"strings"
	"time"
)

type ecsTaskDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*ecsTaskDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*ecsTaskDiscovery)(nil)
)

func NewEcsTaskDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &ecsTaskDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalEcsTask)*time.Second),
	)
}

func (e *ecsTaskDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: ecsTaskTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEcsTask)),
		},
	}
}

func (e *ecsTaskDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       ecsTaskTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "ECS task", Other: "ECS tasks"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(ecsTaskIcon),

		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws-ecs.task.arn"},
				{Attribute: "aws-ecs.cluster.name"},
				{Attribute: "aws-ecs.service.name"},
				{Attribute: "aws.account"},
				{Attribute: "aws.zone"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws-ecs.task.arn",
					Direction: "ASC",
				},
			},
		},
	}
}

func (e *ecsTaskDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws-ecs.task.arn",
			Label: discovery_kit_api.PluralLabel{
				One:   "ECS task ARN",
				Other: "ECS task ARNs",
			},
		},
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
		}}
}

func (e *ecsTaskDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, ctx, "ecs-task")
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := ecs.NewFromConfig(account.AwsConfig)
	result, err := GetAllEcsTasks(ctx, client, utils.Zones, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover ecs-task for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ECS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type EcsTasksApi interface {
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
}

func GetAllEcsTasks(ctx context.Context, ecsApi EcsTasksApi, zoneUtil utils.GetZoneUtil, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	listClusterOutput, err := ecsApi.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return result, err
	}
	for _, clusterArn := range listClusterOutput.ClusterArns {
		paginator := ecs.NewListTasksPaginator(ecsApi, &ecs.ListTasksInput{
			Cluster: extutil.Ptr(clusterArn),
		})
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return result, err
			}

			taskArnPages := splitIntoPages(output)
			for _, taskArnPage := range taskArnPages {
				describeTasksOutput, err := ecsApi.DescribeTasks(ctx, &ecs.DescribeTasksInput{
					Cluster: extutil.Ptr(clusterArn),
					Tasks:   taskArnPage,
					Include: []types.TaskField{types.TaskFieldTags},
				})
				if err != nil {
					return nil, err
				}

				for _, task := range describeTasksOutput.Tasks {
					if task.LastStatus != nil && *task.LastStatus == "RUNNING" {
						result = append(result, toTarget(task, zoneUtil, awsAccountNumber, awsRegion))
					}
				}
			}
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesEcs), nil
}

func splitIntoPages(output *ecs.ListTasksOutput) [][]string {
	taskArnPages := make([][]string, 0, 10)
	for i := 0; i < len(output.TaskArns); {
		end := i + 100
		if end > len(output.TaskArns) {
			end = len(output.TaskArns)
		}
		taskArnPages = append(taskArnPages, output.TaskArns[i:end])
		i = end
	}
	return taskArnPages
}

func toTarget(task types.Task, zoneUtil utils.GetZoneUtil, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	var service, clusterName *string
	for _, tag := range task.Tags {
		if *tag.Key == "aws:ecs:serviceName" {
			service = tag.Value
		}
		if *tag.Key == "aws:ecs:clusterName" {
			clusterName = tag.Value
		}
	}

	arn := aws.ToString(task.TaskArn)
	availabilityZoneName := aws.ToString(task.AvailabilityZone)
	availabilityZoneApi := zoneUtil.GetZone(awsAccountNumber, availabilityZoneName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.zone"] = []string{availabilityZoneName}
	if availabilityZoneApi != nil {
		attributes["aws.zone.id"] = []string{*availabilityZoneApi.ZoneId}
	}
	attributes["aws.region"] = []string{awsRegion}
	if clusterName != nil {
		attributes["aws-ecs.cluster.name"] = []string{*clusterName}
	}
	attributes["aws-ecs.cluster.arn"] = []string{aws.ToString(task.ClusterArn)}
	attributes["aws-ecs.task.arn"] = []string{aws.ToString(task.TaskArn)}
	if service != nil {
		attributes["aws-ecs.service.name"] = []string{*service}
	}
	attributes["aws-ecs.task.launch-type"] = []string{string(task.LaunchType)}
	for _, tag := range task.Tags {
		if aws.ToString(tag.Key) == "aws:ecs:serviceName" || aws.ToString(tag.Key) == "aws:ecs:clusterName" {
			continue
		}
		attributes[fmt.Sprintf("aws-ecs.task.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      arn,
		TargetType: ecsTaskTargetId,
		Attributes: attributes,
	}
}
