/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extelb

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/extec2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strings"
	"time"
)

const describeTagsMaxPagesize = 20

type albDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*albDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*albDiscovery)(nil)
)

func NewAlbDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &albDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalElbAlb)*time.Second),
	)
}

func (e *albDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: albTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalElbAlb)),
		},
	}
}

func (e *albDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       albTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "Application Load Balancer", Other: "Application Load Balancers"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(albIcon),

		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws-elb.alb.name"},
				{Attribute: "aws-elb.alb.dns"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws-elb.alb.name",
					Direction: "ASC",
				},
			},
		},
	}
}

func (e *albDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws-elb.alb.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "Name",
				Other: "Names",
			},
		},
		{
			Attribute: "aws-elb.alb.dns",
			Label: discovery_kit_api.PluralLabel{
				One:   "DNS",
				Other: "DNS",
			},
		},
	}
}

func (e *albDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getTargetsForAccount, ctx, "ecs-task")
}

func getTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := elasticloadbalancingv2.NewFromConfig(account.AwsConfig)
	result, err := GetAlbs(ctx, client, extec2.Util, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover albs for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ELB=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type AlbDiscoveryApi interface {
	elasticloadbalancingv2.DescribeLoadBalancersAPIClient
	elasticloadbalancingv2.DescribeListenersAPIClient
	DescribeTags(ctx context.Context, params *elasticloadbalancingv2.DescribeTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTagsOutput, error)
}

type albDiscoveryEc2Util interface {
	extec2.GetZoneUtil
	extec2.GetVpcNameUtil
}

func GetAlbs(ctx context.Context, albDiscoveryApi AlbDiscoveryApi, ec2Util albDiscoveryEc2Util, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(albDiscoveryApi, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, extension_kit.ToError("Failed to fetch load balancers.", err)
		}

		lbPages := utils.SplitIntoPages(output.LoadBalancers, describeTagsMaxPagesize)
		for _, lbPage := range lbPages {
			lbArns := make([]string, 0, len(lbPage))
			for _, loadBalancer := range lbPage {
				lbArns = append(lbArns, *loadBalancer.LoadBalancerArn)
			}
			describeTagsResult, err := albDiscoveryApi.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
				ResourceArns: lbArns,
			})
			if err != nil {
				return nil, extension_kit.ToError("Failed to fetch tags.", err)
			}

			for _, loadBalancer := range lbPage {
				if loadBalancer.Type != types.LoadBalancerTypeEnumApplication {
					continue
				}

				var tags []types.Tag
				for _, tagDescription := range describeTagsResult.TagDescriptions {
					if *tagDescription.ResourceArn == *loadBalancer.LoadBalancerArn {
						tags = tagDescription.Tags
					}
				}

				describeListenersResult, err := albDiscoveryApi.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
					LoadBalancerArn: loadBalancer.LoadBalancerArn,
				})
				if err != nil {
					return nil, extension_kit.ToError("Failed to fetch load balancer listeners.", err)
				}

				if matchesTagFilter(tags, account.TagFilters) {
					result = append(result, toTarget(&loadBalancer, tags, describeListenersResult.Listeners, ec2Util, account.AccountNumber, account.Region, account.AssumeRole))
				}
			}
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesEcs), nil
}

func toTarget(lb *types.LoadBalancer, tags []types.Tag, listeners []types.Listener, ec2Util albDiscoveryEc2Util, awsAccountNumber string, awsRegion string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(lb.LoadBalancerArn)
	name := aws.ToString(lb.LoadBalancerName)
	zones := make([]string, 0, len(lb.AvailabilityZones))
	zoneIds := make([]string, 0, len(lb.AvailabilityZones))
	for _, zone := range lb.AvailabilityZones {
		zones = append(zones, aws.ToString(zone.ZoneName))
		zoneApi := ec2Util.GetZone(awsAccountNumber, aws.ToString(zone.ZoneName), awsRegion)
		if zoneApi != nil {
			zoneIds = append(zoneIds, *zoneApi.ZoneId)
		}
	}
	listenerPorts := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		if listener.Port != nil {
			listenerPorts = append(listenerPorts, fmt.Sprintf("%d", *listener.Port))
		}
	}

	attributes := make(map[string][]string)
	attributes["aws-elb.alb.name"] = []string{name}
	attributes["aws-elb.alb.dns"] = []string{aws.ToString(lb.DNSName)}
	attributes["aws-elb.alb.arn"] = []string{arn}
	attributes["aws-elb.alb.listener.port"] = listenerPorts
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{awsRegion}
	if lb.VpcId != nil {
		attributes["aws.vpc.id"] = []string{aws.ToString(lb.VpcId)}
		attributes["aws.vpc.name"] = []string{ec2Util.GetVpcName(awsAccountNumber, awsRegion, aws.ToString(lb.VpcId))}
	}
	attributes["aws.zone"] = zones
	attributes["aws.zone.id"] = zoneIds

	for _, tag := range tags {
		if *tag.Key == "elbv2.k8s.aws/cluster" {
			attributes["k8s.cluster-name"] = []string{aws.ToString(tag.Value)}
		}
		attributes[fmt.Sprintf("aws-elb.alb.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: albTargetId,
		Attributes: attributes,
	}
}
