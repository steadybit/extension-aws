// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extelb

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/extec2"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
)

type nlbDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*nlbDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*nlbDiscovery)(nil)
)

func NewNlbDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&nlbDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalElbNlb)*time.Second),
	)
}

func (d *nlbDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: nlbTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalElbNlb)),
		},
	}
}

func (d *nlbDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       nlbTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "Network Load Balancer", Other: "Network Load Balancers"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(nlbIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws-elb.nlb.name"},
				{Attribute: "aws-elb.nlb.scheme"},
				{Attribute: "aws-elb.nlb.cross-zone-load-balancing"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "aws-elb.nlb.name", Direction: "ASC"}},
		},
	}
}

func (d *nlbDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws-elb.nlb.name", Label: discovery_kit_api.PluralLabel{One: "NLB name", Other: "NLB names"}},
		{Attribute: "aws-elb.nlb.dns", Label: discovery_kit_api.PluralLabel{One: "NLB DNS name", Other: "NLB DNS names"}},
		{Attribute: "aws-elb.nlb.arn", Label: discovery_kit_api.PluralLabel{One: "NLB ARN", Other: "NLB ARNs"}},
		{Attribute: "aws-elb.nlb.scheme", Label: discovery_kit_api.PluralLabel{One: "NLB scheme", Other: "NLB schemes"}},
		{Attribute: "aws-elb.nlb.ip-address-type", Label: discovery_kit_api.PluralLabel{One: "NLB IP address type", Other: "NLB IP address types"}},
		{Attribute: "aws-elb.nlb.listener.port", Label: discovery_kit_api.PluralLabel{One: "NLB listener port", Other: "NLB listener ports"}},
		{Attribute: "aws-elb.nlb.listener.protocol", Label: discovery_kit_api.PluralLabel{One: "NLB listener protocol", Other: "NLB listener protocols"}},
		{Attribute: "aws-elb.nlb.cross-zone-load-balancing", Label: discovery_kit_api.PluralLabel{One: "NLB cross-zone load balancing", Other: "NLB cross-zone load balancing"}},
		{Attribute: "aws-elb.nlb.deletion-protection", Label: discovery_kit_api.PluralLabel{One: "NLB deletion protection", Other: "NLB deletion protection"}},
		{Attribute: "aws-elb.nlb.access-logs.enabled", Label: discovery_kit_api.PluralLabel{One: "NLB access logs", Other: "NLB access logs"}},
	}
}

func (d *nlbDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getNlbTargetsForAccount, ctx, "nlb")
}

func getNlbTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := elasticloadbalancingv2.NewFromConfig(account.AwsConfig)
	result, err := getNlbs(ctx, client, extec2.Util, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover NLBs for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ELB=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type NlbDiscoveryApi interface {
	elasticloadbalancingv2.DescribeLoadBalancersAPIClient
	elasticloadbalancingv2.DescribeListenersAPIClient
	DescribeTags(ctx context.Context, params *elasticloadbalancingv2.DescribeTagsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTagsOutput, error)
	DescribeLoadBalancerAttributes(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancerAttributesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput, error)
}

func getNlbs(ctx context.Context, api NlbDiscoveryApi, ec2Util albDiscoveryEc2Util, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(api, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, extension_kit.ToError("Failed to fetch load balancers.", err)
		}
		lbPages := utils.SplitIntoPages(output.LoadBalancers, describeTagsMaxPagesize)
		for _, lbPage := range lbPages {
			lbArns := make([]string, 0, len(lbPage))
			for _, lb := range lbPage {
				if lb.Type == types.LoadBalancerTypeEnumNetwork {
					lbArns = append(lbArns, *lb.LoadBalancerArn)
				}
			}
			if len(lbArns) == 0 {
				continue
			}
			tagsResult, err := api.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{ResourceArns: lbArns})
			if err != nil {
				return nil, extension_kit.ToError("Failed to fetch tags.", err)
			}

			for _, lb := range lbPage {
				if lb.Type != types.LoadBalancerTypeEnumNetwork {
					continue
				}
				var tags []types.Tag
				for _, td := range tagsResult.TagDescriptions {
					if *td.ResourceArn == *lb.LoadBalancerArn {
						tags = td.Tags
					}
				}
				if !matchesTagFilter(tags, account.TagFilters) {
					continue
				}

				listenersOut, err := api.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{LoadBalancerArn: lb.LoadBalancerArn})
				if err != nil {
					return nil, extension_kit.ToError("Failed to fetch NLB listeners.", err)
				}

				attrsOut, err := api.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{LoadBalancerArn: lb.LoadBalancerArn})
				if err != nil {
					return nil, extension_kit.ToError("Failed to fetch NLB attributes.", err)
				}

				result = append(result, toNlbTarget(&lb, tags, listenersOut.Listeners, attrsOut.Attributes, ec2Util, account.AccountNumber, account.Region, account.AssumeRole))
			}
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesElb), nil
}

func toNlbTarget(lb *types.LoadBalancer, tags []types.Tag, listeners []types.Listener, lbAttrs []types.LoadBalancerAttribute, ec2Util albDiscoveryEc2Util, awsAccount string, awsRegion string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(lb.LoadBalancerArn)
	name := aws.ToString(lb.LoadBalancerName)

	zones := make([]string, 0, len(lb.AvailabilityZones))
	zoneIds := make([]string, 0, len(lb.AvailabilityZones))
	subnets := make([]string, 0, len(lb.AvailabilityZones))
	for _, az := range lb.AvailabilityZones {
		zones = append(zones, aws.ToString(az.ZoneName))
		zoneApi := ec2Util.GetZone(awsAccount, awsRegion, aws.ToString(az.ZoneName))
		if zoneApi != nil {
			zoneIds = append(zoneIds, *zoneApi.ZoneId)
		}
		if az.SubnetId != nil {
			subnets = append(subnets, *az.SubnetId)
		}
	}

	listenerPorts := make([]string, 0, len(listeners))
	listenerProtocols := make(map[string]bool)
	for _, l := range listeners {
		if l.Port != nil {
			listenerPorts = append(listenerPorts, fmt.Sprintf("%d", *l.Port))
		}
		if l.Protocol != "" {
			listenerProtocols[string(l.Protocol)] = true
		}
	}
	protocols := make([]string, 0, len(listenerProtocols))
	for p := range listenerProtocols {
		protocols = append(protocols, p)
	}

	attributes := make(map[string][]string)
	attributes["aws-elb.nlb.name"] = []string{name}
	attributes["aws-elb.nlb.dns"] = []string{aws.ToString(lb.DNSName)}
	attributes["aws-elb.nlb.arn"] = []string{arn}
	if lb.Scheme != "" {
		attributes["aws-elb.nlb.scheme"] = []string{string(lb.Scheme)}
	}
	if lb.IpAddressType != "" {
		attributes["aws-elb.nlb.ip-address-type"] = []string{string(lb.IpAddressType)}
	}
	if len(listenerPorts) > 0 {
		attributes["aws-elb.nlb.listener.port"] = listenerPorts
	}
	if len(protocols) > 0 {
		attributes["aws-elb.nlb.listener.protocol"] = protocols
	}
	attributes["aws.account"] = []string{awsAccount}
	attributes["aws.region"] = []string{awsRegion}
	if lb.VpcId != nil {
		attributes["aws.vpc.id"] = []string{aws.ToString(lb.VpcId)}
		attributes["aws.vpc.name"] = []string{ec2Util.GetVpcName(awsAccount, awsRegion, aws.ToString(lb.VpcId))}
	}
	if len(zones) > 0 {
		attributes["aws.zone"] = zones
	}
	if len(zoneIds) > 0 {
		attributes["aws.zone.id"] = zoneIds
	}
	if len(subnets) > 0 {
		attributes["aws-elb.nlb.subnets"] = subnets
	}

	for _, a := range lbAttrs {
		if a.Key == nil || a.Value == nil {
			continue
		}
		switch *a.Key {
		case "load_balancing.cross_zone.enabled":
			attributes["aws-elb.nlb.cross-zone-load-balancing"] = []string{normalizeBool(*a.Value)}
		case "deletion_protection.enabled":
			attributes["aws-elb.nlb.deletion-protection"] = []string{normalizeBool(*a.Value)}
		case "access_logs.s3.enabled":
			attributes["aws-elb.nlb.access-logs.enabled"] = []string{normalizeBool(*a.Value)}
		}
	}

	for _, tag := range tags {
		if tag.Key == nil {
			continue
		}
		if *tag.Key == "elbv2.k8s.aws/cluster" {
			attributes["k8s.cluster-name"] = []string{aws.ToString(tag.Value)}
		}
		attributes[fmt.Sprintf("aws-elb.nlb.label.%s", strings.ToLower(*tag.Key))] = []string{aws.ToString(tag.Value)}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: nlbTargetId,
		Attributes: attributes,
	}
}

func normalizeBool(v string) string {
	if b, err := strconv.ParseBool(v); err == nil {
		return strconv.FormatBool(b)
	}
	return v
}
