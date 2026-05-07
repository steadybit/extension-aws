// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type natGatewayDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*natGatewayDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*natGatewayDiscovery)(nil)
)

func NewNatGatewayDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&natGatewayDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalNatGateway)*time.Second),
	)
}

func (d *natGatewayDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: natGatewayTargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalNatGateway)),
		},
	}
}

func (d *natGatewayDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       natGatewayTargetType,
		Label:    discovery_kit_api.PluralLabel{One: "NAT Gateway", Other: "NAT Gateways"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(natGatewayIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.zone"},
				{Attribute: "aws.nat-gateway.connectivity-type"},
				{Attribute: "aws.nat-gateway.availability-mode"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *natGatewayDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.nat-gateway.id", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway ID", Other: "NAT Gateway IDs"}},
		{Attribute: "aws.nat-gateway.connectivity-type", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway connectivity type", Other: "NAT Gateway connectivity types"}},
		{Attribute: "aws.nat-gateway.state", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway state", Other: "NAT Gateway states"}},
		{Attribute: "aws.nat-gateway.subnet", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway subnet", Other: "NAT Gateway subnets"}},
		{Attribute: "aws.nat-gateway.vpc", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway VPC", Other: "NAT Gateway VPCs"}},
		{Attribute: "aws.nat-gateway.elastic-ips", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway Elastic IP", Other: "NAT Gateway Elastic IPs"}},
		{Attribute: "aws.vpc.nat-gateway-count-in-vpc", Label: discovery_kit_api.PluralLabel{One: "NAT Gateways in VPC", Other: "NAT Gateways in VPC"}},
		{Attribute: "aws.nat-gateway.availability-mode", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway availability mode", Other: "NAT Gateway availability modes"}},
	}
}

func (d *natGatewayDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getNatGatewayTargets, ctx, "nat-gateway")
}

func getNatGatewayTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	result, err := getAllNatGateways(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover NAT gateways for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_NAT_GATEWAY=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type natGatewayApi interface {
	DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
}

func getAllNatGateways(ctx context.Context, client natGatewayApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	gateways, err := listAllNatGateways(ctx, client)
	if err != nil {
		return nil, err
	}

	subnetToAz, err := buildSubnetAzMap(ctx, client, gateways)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to map NAT gateway subnets to AZs; AZ attributes will be missing.")
		subnetToAz = map[string]string{}
	}

	gwsPerVpc := make(map[string]int)
	for _, gw := range gateways {
		if gw.VpcId == nil || gw.State == types.NatGatewayStateDeleted {
			continue
		}
		gwsPerVpc[*gw.VpcId]++
	}

	result := make([]discovery_kit_api.Target, 0, len(gateways))
	for _, gw := range gateways {
		if gw.State == types.NatGatewayStateDeleted {
			continue
		}
		if !matchesEc2TagFilter(gw.Tags, account.TagFilters) {
			continue
		}
		result = append(result, toNatGatewayTarget(gw, subnetToAz, gwsPerVpc, account.AccountNumber, account.Region, account.AssumeRole))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesNatGateway), nil
}

func listAllNatGateways(ctx context.Context, client natGatewayApi) ([]types.NatGateway, error) {
	result := make([]types.NatGateway, 0)
	var nextToken *string
	for {
		out, err := client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		result = append(result, out.NatGateways...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return result, nil
}

func buildSubnetAzMap(ctx context.Context, client natGatewayApi, gateways []types.NatGateway) (map[string]string, error) {
	subnetSet := make(map[string]struct{})
	for _, gw := range gateways {
		if gw.SubnetId != nil {
			subnetSet[*gw.SubnetId] = struct{}{}
		}
	}
	if len(subnetSet) == 0 {
		return map[string]string{}, nil
	}
	ids := make([]string, 0, len(subnetSet))
	for id := range subnetSet {
		ids = append(ids, id)
	}
	out, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: ids})
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(out.Subnets))
	for _, s := range out.Subnets {
		if s.SubnetId != nil && s.AvailabilityZone != nil {
			m[*s.SubnetId] = *s.AvailabilityZone
		}
	}
	return m, nil
}

func matchesEc2TagFilter(tags []types.Tag, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		for _, tag := range tags {
			if tag.Key != nil && *tag.Key == filter.Key {
				for _, v := range filter.Values {
					if tag.Value != nil && *tag.Value == v {
						matched = true
						break
					}
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func toNatGatewayTarget(gw types.NatGateway, subnetToAz map[string]string, gwsPerVpc map[string]int, account string, region string, role *string) discovery_kit_api.Target {
	id := aws.ToString(gw.NatGatewayId)
	name := nameFromTags(gw.Tags, id)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.nat-gateway.id"] = []string{id}
	attributes["aws.nat-gateway.state"] = []string{string(gw.State)}
	if gw.ConnectivityType != "" {
		attributes["aws.nat-gateway.connectivity-type"] = []string{string(gw.ConnectivityType)}
	}
	if gw.SubnetId != nil {
		attributes["aws.nat-gateway.subnet"] = []string{*gw.SubnetId}
		if az, ok := subnetToAz[*gw.SubnetId]; ok {
			attributes["aws.zone"] = []string{az}
		}
	}
	if gw.VpcId != nil {
		attributes["aws.nat-gateway.vpc"] = []string{*gw.VpcId}
		count := gwsPerVpc[*gw.VpcId]
		attributes["aws.vpc.nat-gateway-count-in-vpc"] = []string{strconv.Itoa(count)}
	}

	eips := make([]string, 0)
	for _, addr := range gw.NatGatewayAddresses {
		if addr.PublicIp != nil {
			eips = append(eips, *addr.PublicIp)
		}
	}
	if len(eips) > 0 {
		sort.Strings(eips)
		attributes["aws.nat-gateway.elastic-ips"] = eips
	}

	// AWS NatGateway.AvailabilityMode: "zonal" (single-AZ) or "regional" (multi-AZ).
	if gw.AvailabilityMode != "" {
		attributes["aws.nat-gateway.availability-mode"] = []string{string(gw.AvailabilityMode)}
	}

	for _, tag := range gw.Tags {
		if tag.Key == nil {
			continue
		}
		attributes[fmt.Sprintf("aws.nat-gateway.label.%s", strings.ToLower(*tag.Key))] = []string{aws.ToString(tag.Value)}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		Label:      name,
		TargetType: natGatewayTargetType,
		Attributes: attributes,
	}
}

func nameFromTags(tags []types.Tag, fallback string) string {
	for _, t := range tags {
		if t.Key != nil && strings.EqualFold(*t.Key, "Name") && t.Value != nil && *t.Value != "" {
			return *t.Value
		}
	}
	return fallback
}
