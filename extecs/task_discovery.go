// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

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
	"github.com/steadybit/extension-aws/extec2"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strings"
	"time"
)

// pageSize is restricted by AWS ECS API.
const taskPageSize = 100

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
			Attribute: "aws-ecs.task.amazon-ssm-agent",
			Label: discovery_kit_api.PluralLabel{
				One:   "ECS task includes Amazon SSM agent",
				Other: "ECS tasks includes Amazon SSM agent",
			},
		},
		{
			Attribute: "aws-ecs.task.enable-execute-command",
			Label: discovery_kit_api.PluralLabel{
				One:   "ECS task has execute command enabled",
				Other: "ECS tasks has execute command enabled",
			},
		},
	}
}

func (e *ecsTaskDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getTargetsForAccount, ctx, "ecs-task")
}

func getTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := ecs.NewFromConfig(account.AwsConfig)
	result, err := GetAllEcsTasks(ctx, client, extec2.Util, account)
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
	ecs.ListTasksAPIClient
	ecs.DescribeTasksAPIClient
	ecs.ListClustersAPIClient
}

type taskDiscoveryEc2Util interface {
	extec2.GetZoneUtil
	extec2.GetVpcNameUtil
}

func GetAllEcsTasks(ctx context.Context, ecsApi EcsTasksApi, ec2Util taskDiscoveryEc2Util, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
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

			taskArnPages := utils.SplitIntoPages(output.TaskArns, taskPageSize)
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
					if task.LastStatus != nil && *task.LastStatus == "RUNNING" && !ignoreTask(task) && matchesTagFilter(task.Tags, account.TagFilters) {
						result = append(result, toTarget(task, ec2Util, account.AccountNumber, account.Region, account.AssumeRole))
					}
				}
			}
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesEcs), nil
}

func ignoreTask(service types.Task) bool {
	if config.Config.DisableDiscoveryExcludes {
		return false
	}
	for _, tag := range service.Tags {
		if aws.ToString(tag.Key) == "steadybit.com/discovery-disabled" && aws.ToString(tag.Value) == "true" {
			return true
		}
	}
	return false
}

func toTarget(task types.Task, ec2Util taskDiscoveryEc2Util, awsAccountNumber string, awsRegion string, role *string) discovery_kit_api.Target {
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
	availabilityZoneApi := ec2Util.GetZone(awsAccountNumber, availabilityZoneName, awsRegion)

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

	if hasAmazonSsmSidecar(task) {
		attributes["aws-ecs.task.amazon-ssm-agent"] = []string{"true"}
	}
	if task.EnableExecuteCommand {
		attributes["aws-ecs.task.enable-execute-command"] = []string{"true"}
	}
	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      arn,
		TargetType: ecsTaskTargetId,
		Attributes: attributes,
	}
}

func hasAmazonSsmSidecar(task types.Task) bool {
	for _, container := range task.Containers {
		if container.Image != nil && strings.Contains(*container.Image, "amazon-ssm-agent") {
			return true
		}
	}
	return false
}
